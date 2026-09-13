package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"redact-gateway/internal/config"
	"redact-gateway/internal/store"
)

func TestProxyRedactsAndRestoresAgainstRealUpstream(t *testing.T) {
	t.Parallel()
	upstreamSawRedaction := make(chan bool, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		var value map[string]any
		_ = json.Unmarshal(body, &value)
		messages := value["messages"].([]any)
		content := messages[0].(map[string]any)["content"].(string)
		upstreamSawRedaction <- !bytes.Contains(body, []byte("alice@example.com")) && bytes.Contains(body, []byte("{{RG_EMAIL_"))
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
	}))
	defer upstream.Close()

	dataDir := t.TempDir()
	eventStore, err := store.Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	cfg := config.Config{
		ListenAddr: "127.0.0.1:0", AdminAddr: "127.0.0.1:0", DataDir: dataDir,
		AllowPrivateHosts: true, MaxBodyBytes: 1024 * 1024, MaxRedactions: 100, CORSOrigin: "*",
	}
	proxy := NewProxy(cfg, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	server := httptest.NewServer(proxy)
	defer server.Close()

	body := []byte(`{"model":"test","messages":[{"role":"user","content":"email alice@example.com"}]}`)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/E$"+upstream.URL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", response.StatusCode, responseBody)
	}
	if !bytes.Contains(responseBody, []byte("alice@example.com")) || bytes.Contains(responseBody, []byte("{{RG_EMAIL_")) {
		t.Fatalf("response was not restored: %s", responseBody)
	}
	if !<-upstreamSawRedaction {
		t.Fatal("upstream received plaintext")
	}

	events, err := eventStore.Events(context.Background(), 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].UpstreamHost == "" || events[0].RedactionCount != 1 {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestPrivateUpstreamBlockedByDefault(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	eventStore, err := store.Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	cfg := config.Config{MaxBodyBytes: 1024, MaxRedactions: 10, CORSOrigin: "*"}
	proxy := NewProxy(cfg, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	request := httptest.NewRequest(http.MethodGet, "http://gateway/$http://127.0.0.1:9000/v1/models", nil)
	recorder := httptest.NewRecorder()
	proxy.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	events, err := eventStore.Events(ctx, 10, "")
	if err != nil || len(events) != 1 || events[0].ErrorClass != "upstream_blocked" {
		t.Fatalf("blocked request not recorded: events=%+v err=%v", events, err)
	}
}
