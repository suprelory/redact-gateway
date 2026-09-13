package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/redact"
	"github.com/suprelory/redact-gateway/internal/store"
)

func sessionTestConfig() config.Config {
	return config.Config{
		MaxBodyBytes: 1024 * 1024, MaxRedactions: 100, AllowPrivateHosts: true,
		SessionCacheEnabled: true, SessionCacheTTL: time.Hour,
		SessionCacheMaxSessions: 20, SessionCacheMaxEntries: 100, SessionCacheMaxBytes: 64 * 1024,
	}
}

func sessionTestRequest() *http.Request {
	request := httptest.NewRequest(http.MethodPost, "http://gateway/E$https://example.com/v1/responses", nil)
	request.Header.Set(sessionHeader, "session_0123456789abcdef")
	request.Header.Set("Authorization", "Bearer test-credential")
	return request
}

func TestSessionScopeSeparatesCredentialsAndUpstreams(t *testing.T) {
	for _, change := range []string{"session", "authorization", "api_key", "azure_key", "organization", "project", "scheme", "host", "port", "path", "query"} {
		t.Run(change, func(t *testing.T) {
			proxy := NewProxy(sessionTestConfig(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
			defer proxy.Close()
			request := sessionTestRequest()
			upstream, _ := url.Parse("https://example.com/v1/responses")
			first, err := proxy.requestContext(request, upstream)
			if err != nil {
				t.Fatal(err)
			}
			token, err := first.RedactText("alice@example.com", redact.DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			same, err := proxy.requestContext(request, upstream)
			if err != nil || same.RestoreText(token) != "alice@example.com" {
				t.Fatalf("same scope missed its map: %v", err)
			}
			switch change {
			case "session":
				request.Header.Set(sessionHeader, "session_other_0123456789")
			case "authorization":
				request.Header.Set("Authorization", "Bearer another-credential")
			case "api_key":
				request.Header.Set("X-Api-Key", "another-key")
			case "azure_key":
				request.Header.Set("Api-Key", "another-key")
			case "organization":
				request.Header.Set("OpenAI-Organization", "another-org")
			case "project":
				request.Header.Set("OpenAI-Project", "another-project")
			case "scheme":
				upstream.Scheme = "http"
			case "host":
				upstream.Host = "another.example.com"
			case "port":
				upstream.Host = "example.com:8443"
			case "path":
				upstream.Path = "/tenant/v1/responses"
			case "query":
				upstream.RawQuery = "project=another"
			}
			other, err := proxy.requestContext(request, upstream)
			if err != nil || other.RestoreText(token) != token || other.UnresolvedCount() != 1 {
				t.Fatalf("map crossed %s boundary: %v", change, err)
			}
		})
	}
}

func TestSessionCacheRequiresExplicitScopeAndForwardedCredentials(t *testing.T) {
	for _, mode := range []string{"disabled", "no_session", "no_credentials", "hop_by_hop_credentials"} {
		t.Run(mode, func(t *testing.T) {
			cfg := sessionTestConfig()
			if mode == "disabled" {
				cfg.SessionCacheEnabled = false
			}
			proxy := NewProxy(cfg, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
			defer proxy.Close()
			request := sessionTestRequest()
			switch mode {
			case "no_session":
				request.Header.Del(sessionHeader)
			case "no_credentials":
				request.Header.Del("Authorization")
			case "hop_by_hop_credentials":
				request.Header.Set("Connection", "Authorization")
			}
			upstream, _ := url.Parse("https://example.com/v1/responses")
			first, err := proxy.requestContext(request, upstream)
			if err != nil {
				t.Fatal(err)
			}
			token, err := first.RedactText("alice@example.com", redact.DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			second, err := proxy.requestContext(request, upstream)
			if err != nil || second.RestoreText(token) != token {
				t.Fatalf("request isolation failed: %v", err)
			}
		})
	}
}

func TestSessionHeaderValidationAndForwarding(t *testing.T) {
	proxy := NewProxy(sessionTestConfig(), nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	defer proxy.Close()
	upstream, _ := url.Parse("https://example.com/v1/responses")
	for _, value := range []string{"", "short", "session with spaces", strings.Repeat("x", 257), "session_中文0123456789"} {
		request := sessionTestRequest()
		request.Header.Set(sessionHeader, value)
		if _, err := proxy.requestContext(request, upstream); err == nil {
			t.Fatalf("invalid session ID accepted: %q", value)
		}
	}
	request := sessionTestRequest()
	request.Header.Add(sessionHeader, "another_session_0123456789")
	if _, err := proxy.requestContext(request, upstream); err == nil {
		t.Fatal("duplicate session headers accepted")
	}
	forwarded := make(http.Header)
	copyRequestHeaders(forwarded, request.Header)
	if forwarded.Get(sessionHeader) != "" || forwarded.Get("Authorization") != request.Header.Get("Authorization") {
		t.Fatal("session header forwarded or credentials discarded")
	}
}

func TestProxyRestoresResponsesContinuationFromSession(t *testing.T) {
	var mu sync.Mutex
	var issued string
	const responseID = "resp_Dh2n7aBPm5Jq3N6Lw4Uv8Ee9RzFc0XyK"
	const encrypted = "gAAAAABmZGF0YQ5pOyd7wxEdGhciFzx1OeqSFD0vGzjMK2QYpHVNeRjYAZv0"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Header.Get(sessionHeader) != "" || r.Header.Get("Authorization") != "Bearer test-credential" {
			t.Error("gateway-only header or incorrect credentials sent upstream")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if body["previous_response_id"] == nil {
			input, _ := body["input"].(string)
			issued = regexp.MustCompile(`\{\{RG_EMAIL_[A-Z2-7]{16}\}\}`).FindString(input)
			if issued == "" || strings.Contains(input, "alice@example.com") {
				t.Error("upstream received an unmasked first request")
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"id": responseID, "output_text": issued})
			return
		}
		if body["previous_response_id"] != responseID || body["input"].([]any)[0].(map[string]any)["encrypted_content"] != encrypted {
			t.Error("continuation ID or encrypted content changed")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, delta := range []string{issued[:12], issued[12:]} {
			encoded, _ := json.Marshal(map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": delta})
			fmt.Fprintf(w, "data: %s\n\n", encoded)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()
	eventStore, err := store.Open(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	proxy := NewProxy(sessionTestConfig(), eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	defer proxy.Close()
	for index, session := range []string{"session_0123456789abcdef", "session_0123456789abcdef", "different_session_0123456789"} {
		body := map[string]any{"input": "alice@example.com"}
		if index > 0 {
			body = map[string]any{"previous_response_id": responseID, "input": []any{map[string]any{"type": "reasoning", "encrypted_content": encrypted}}}
		}
		encoded, _ := json.Marshal(body)
		request := httptest.NewRequest(http.MethodPost, "http://gateway/HPSIBEG$"+upstream.URL+"/v1/responses", strings.NewReader(string(encoded)))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer test-credential")
		request.Header.Set(sessionHeader, session)
		recorder := httptest.NewRecorder()
		proxy.ServeHTTP(recorder, request)
		if recorder.Code != 200 || strings.Contains(recorder.Body.String(), "alice@example.com") != (index < 2) {
			t.Fatalf("request %d: unexpected restoration: %s", index, recorder.Body.String())
		}
	}
	events, err := eventStore.Events(context.Background(), 10, "")
	if err != nil || len(events) != 3 {
		t.Fatalf("events=%+v err=%v", events, err)
	}
	if events[1].RedactionCount != 0 || events[1].RestoreCount != 1 || events[1].RestoreStatus != "restored" || events[0].RestoreCount != 0 || events[0].RestoreStatus != "unresolved" {
		t.Fatalf("continuation statistics crossed request boundaries: %+v", events)
	}
	logged, _ := json.Marshal(events)
	for _, secret := range []string{"alice@example.com", "test-credential", "session_0123456789abcdef", encrypted} {
		if strings.Contains(string(logged), secret) {
			t.Fatal("session mapping material persisted in event logs")
		}
	}
}
