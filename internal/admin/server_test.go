package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/gateway"
	"github.com/suprelory/redact-gateway/internal/store"
)

func TestSettingsEndpointUpdatesRuntimeAndPersists(t *testing.T) {
	dataDir := t.TempDir()
	eventStore, err := store.Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	proxy := gateway.NewProxy(config.Config{MaxBodyBytes: 1024, MaxRedactions: 10, CORSOrigin: "*"}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	handler := NewServer("test-token", proxy, eventStore).Handler()

	body := bytes.NewBufferString(`{"allowed_hosts":[" API.OpenAI.com. ","api.anthropic.com"]}`)
	request := httptest.NewRequest(http.MethodPut, "http://gateway/api/v1/settings", body)
	request.Header.Set("X-Redact-Token", "test-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", recorder.Code, recorder.Body.String())
	}

	var response settingsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	want := []string{"api.openai.com", "api.anthropic.com"}
	if !reflect.DeepEqual(response.AllowedHosts, want) || !reflect.DeepEqual(proxy.AllowedHosts(), want) {
		t.Fatalf("response = %#v, proxy = %#v, want %#v", response.AllowedHosts, proxy.AllowedHosts(), want)
	}

	stored, found, err := eventStore.LoadAllowedHosts(request.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !found || !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored = %#v, found=%v, want %#v", stored, found, want)
	}
}

func TestSettingsEndpointRejectsInvalidHost(t *testing.T) {
	dataDir := t.TempDir()
	eventStore, err := store.Open(dataDir, 30)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	proxy := gateway.NewProxy(config.Config{MaxBodyBytes: 1024, MaxRedactions: 10, CORSOrigin: "*"}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	handler := NewServer("test-token", proxy, eventStore).Handler()

	request := httptest.NewRequest(http.MethodPut, "http://gateway/api/v1/settings", bytes.NewBufferString(`{"allowed_hosts":["https://api.openai.com"]}`))
	request.Header.Set("X-Redact-Token", "test-token")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d", recorder.Code)
	}
	if len(proxy.AllowedHosts()) != 0 {
		t.Fatalf("invalid update changed runtime hosts: %#v", proxy.AllowedHosts())
	}
}
