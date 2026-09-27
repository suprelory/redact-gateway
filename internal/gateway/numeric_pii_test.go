package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/store"
)

func TestProxyBlocksNumericPIIBeforeUpstream(t *testing.T) {
	var upstreamRequests atomic.Int32
	var plaintextReceived atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upstreamRequests.Add(1)
		body, _ := io.ReadAll(request.Body)
		for _, secret := range []string{"13800138000", "110105194912310038", "4111111111111111"} {
			if bytes.Contains(body, []byte(secret)) {
				plaintextReceived.Store(true)
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	defer upstream.Close()
	dataDir := t.TempDir()
	eventStore, err := store.Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	proxy := NewProxy(config.Config{DataDir: dataDir, AllowPrivateHosts: true, MaxBodyBytes: 1024 * 1024, MaxRedactions: 100, CORSOrigin: "*"}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	for _, body := range []string{
		`{"data":{"phone":13800138000}}`, `{"data":{"phone":13800138000.0}}`, `{"data":{"phone":1.3800138e10}}`,
		`{"content":"{\"phone\":13800138000}"}`, `{"data":{"id":110105194912310038}}`, `{"data":{"card":4111111111111111}}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "http://gateway/PIB$"+upstream.URL+"/v1/chat/completions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		proxy.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest || upstreamRequests.Load() != 0 {
			t.Fatalf("numeric PII reached upstream or returned wrong status: %d / %d / %s", recorder.Code, upstreamRequests.Load(), recorder.Body.String())
		}
		var response struct {
			Error struct {
				Code    string
				Message string
			}
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Error.Code != "numeric_sensitive_value" || !strings.Contains(response.Error.Message, "JSON string") || strings.Contains(response.Error.Message, "13800138") || strings.Contains(response.Error.Message, "11010519") || strings.Contains(response.Error.Message, "41111111") {
			t.Fatalf("numeric rejection leaked PII or lacked guidance: %s", recorder.Body.String())
		}
	}
	deepBody := strings.Repeat("[", 65) + `"value"` + strings.Repeat("]", 65)
	deepRequest := httptest.NewRequest(http.MethodPost, "http://gateway/PIB$"+upstream.URL+"/v1/chat/completions", strings.NewReader(deepBody))
	deepRequest.Header.Set("Content-Type", "application/json")
	deepRecorder := httptest.NewRecorder()
	proxy.ServeHTTP(deepRecorder, deepRequest)
	if deepRecorder.Code != http.StatusRequestEntityTooLarge || !strings.Contains(deepRecorder.Body.String(), `"code":"json_nesting_limit"`) || upstreamRequests.Load() != 0 {
		t.Fatalf("nesting limit did not block before upstream: %d / %s", deepRecorder.Code, deepRecorder.Body.String())
	}
	// Sending the same identifiers as strings is accepted, masked before the
	// upstream request, and restored in its echoed response.
	body := `{"data":{"phone":"13800138000","id":"110105194912310038","card":"4111111111111111"}}`
	request := httptest.NewRequest(http.MethodPost, "http://gateway/PIB$"+upstream.URL+"/v1/chat/completions", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || upstreamRequests.Load() != 1 || plaintextReceived.Load() {
		t.Fatalf("string PII was not safely forwarded: %d / %d / %v", recorder.Code, upstreamRequests.Load(), plaintextReceived.Load())
	}
	for _, secret := range []string{"13800138000", "110105194912310038", "4111111111111111"} {
		if !strings.Contains(recorder.Body.String(), secret) {
			t.Fatal("string PII was not restored")
		}
	}
	events, err := eventStore.Events(context.Background(), 20, "")
	if err != nil || len(events) != 8 {
		t.Fatalf("unexpected audit events: %d / %v", len(events), err)
	}
	rejections := 0
	for _, event := range events {
		if event.ErrorClass == "numeric_sensitive_value" {
			rejections++
			if event.Status != http.StatusBadRequest || event.Protocol != "openai_chat" {
				t.Fatalf("numeric rejection audit lost protocol/status: %#v", event)
			}
		}
	}
	if rejections != 6 {
		t.Fatalf("numeric rejection events = %d, want 6", rejections)
	}
}
