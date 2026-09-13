package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/store"
)

func TestProxyRestorationDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name, state, class           string
		restored, unique, unresolved int
	}{
		{"no_echo", "no_placeholders", "", 0, 0, 0},
		{"known", "restored", "", 1, 1, 0},
		{"unknown", "unresolved", "", 0, 0, 1},
		{"mixed", "partial", "", 1, 1, 1},
		{"snapshots", "restored", "", 3, 1, 0},
		{"response.failed", "response_error", "upstream_response_failed", 0, 0, 0},
		{"response.incomplete", "response_error", "upstream_response_incomplete", 0, 0, 0},
		{"error", "response_error", "upstream_stream_error", 0, 0, 0},
		{"binary", "not_processed", "", 0, 0, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					return
				}
				input, _ := body["input"].(string)
				token := regexp.MustCompile(`\{\{RG_EMAIL_[A-Z2-7]{16}\}\}`).FindString(input)
				if token == "" || strings.Contains(input, "alice@example.com") {
					t.Error("request not redacted")
					return
				}
				unknown := "{{RG_EMAIL_ABCDEFGHIJKLMNOP}}"
				if token == unknown {
					unknown = "{{RG_EMAIL_QRSTUVWXYZABCDEF}}"
				}
				if test.name == "binary" {
					w.Header().Set("Content-Type", "application/octet-stream")
					io.WriteString(w, token)
					return
				}
				if test.name == "snapshots" || test.class != "" {
					w.Header().Set("Content-Type", "text/event-stream")
					write := func(event any) { data, _ := json.Marshal(event); fmt.Fprintf(w, "data: %s\n\n", data) }
					if test.class != "" {
						write(map[string]any{"type": test.name, "error": map[string]any{"message": "fake upstream error"}})
						return
					}
					write(map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": token})
					write(map[string]any{"type": "response.output_text.done", "output_index": 0, "text": token})
					write(map[string]any{"type": "response.completed", "response": map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"text": token}}}}}})
					return
				}
				text := "Done. {{RG_TYPE_TOKEN}}"
				switch test.name {
				case "known":
					text = token
				case "unknown":
					text = unknown
				case "mixed":
					text = token + " " + unknown
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"output_text": text})
			}))
			defer upstream.Close()
			eventStore, err := store.Open(t.TempDir(), 0)
			if err != nil {
				t.Fatal(err)
			}
			defer eventStore.Close()
			proxy := NewProxy(config.Config{AllowPrivateHosts: true, MaxBodyBytes: 1024 * 1024, MaxRedactions: 100, CORSOrigin: "*"}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
			request := httptest.NewRequest(http.MethodPost, "http://gateway/E$"+upstream.URL+"/v1/responses", strings.NewReader(`{"model":"test","input":"alice@example.com"}`))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			proxy.ServeHTTP(recorder, request)
			events, err := eventStore.Events(context.Background(), 10, "")
			if err != nil || len(events) != 1 {
				t.Fatalf("events=%#v err=%v", events, err)
			}
			event := events[0]
			if recorder.Code != 200 || event.Status != 200 || event.RedactionCount != 1 || event.RestoreStatus != test.state || event.ErrorClass != test.class || event.RestoreCount != test.restored || event.RestoreUniqueCount != test.unique || event.RestoreUnresolvedCount != test.unresolved {
				t.Fatalf("unexpected diagnostics: %+v", event)
			}
			encoded, _ := json.Marshal(event)
			if strings.Contains(string(encoded), "alice@example.com") || strings.Contains(string(encoded), "{{RG_") || strings.Contains(string(encoded), "fake upstream error") {
				t.Fatal("event contains response content")
			}
		})
	}
}
