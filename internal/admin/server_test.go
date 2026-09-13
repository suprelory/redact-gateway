package admin

import (
	"bytes"
	"context"
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

func TestEventsEndpointPagination(t *testing.T) {
	eventStore, err := store.Open(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	for _, event := range []store.Event{
		{RequestID: "oldest", UpstreamHost: "api.example.com", UpstreamPath: "/v1/messages"},
		{RequestID: "middle", UpstreamHost: "api.example.com", UpstreamPath: "/v1/messages"},
		{RequestID: "newest", UpstreamHost: "other.example.com", UpstreamPath: "/v1/responses"},
	} {
		event.Timestamp = "2026-09-13T12:00:00Z"
		if err := eventStore.InsertEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewServer("test-token", nil, eventStore).Handler()
	tests := []struct {
		query              string
		total, page, limit int
		requestIDs         []string
	}{
		{"", 3, 1, 100, []string{"newest", "middle", "oldest"}},
		{"?limit=1&page=2", 3, 2, 1, []string{"middle"}},
		{"?limit=1&page=2&q=MESSAGE&upstream=api.example.com", 2, 2, 1, []string{"oldest"}},
		{"?limit=2&page=999", 3, 2, 2, []string{"oldest"}},
		{"?limit=501&page=-1", 3, 1, 100, []string{"newest", "middle", "oldest"}},
		{"?limit=invalid&page=99999999999999999999999", 3, 1, 100, []string{"newest", "middle", "oldest"}},
		{"?page=4&q=missing", 0, 1, 100, []string{}},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://gateway/api/v1/events"+test.query, nil)
			request.Header.Set("X-Redact-Token", "test-token")
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d: %s", recorder.Code, recorder.Body.String())
			}
			var result store.EventPage
			if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(result.Events))
			for _, event := range result.Events {
				ids = append(ids, event.RequestID)
			}
			if result.Total != test.total || result.Page != test.page || result.Limit != test.limit || result.Events == nil || !reflect.DeepEqual(ids, test.requestIDs) {
				t.Fatalf("unexpected page: %+v", result)
			}
		})
	}
}
