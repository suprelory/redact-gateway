package admin

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/gateway"
	"github.com/suprelory/redact-gateway/internal/store"
)

func gatewaySettingsRequest(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, "http://gateway"+path, strings.NewReader(body))
	request.Header.Set("X-Redact-Token", "test-token")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: status=%d, want=%d: %s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response
}

func TestGatewayURLSettingsPersistAndKeepIndependentFields(t *testing.T) {
	dataDir := t.TempDir()
	startServer := func() (http.Handler, *store.Store) {
		t.Helper()
		eventStore, err := store.Open(dataDir, 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { eventStore.Close() })
		hosts, found, err := eventStore.LoadAllowedHosts(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			hosts = []string{"api.openai.com"}
		}
		proxy := gateway.NewProxy(config.Config{
			ListenAddr: "0.0.0.0:8787", AllowedHosts: hosts, MaxBodyBytes: 1024, MaxRedactions: 10,
		}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
		t.Cleanup(proxy.Close)
		return NewServer("test-token", proxy, eventStore).Handler(), eventStore
	}
	handler, eventStore := startServer()
	checkSettings := func(body, wantURL string, wantHosts []string) {
		t.Helper()
		method := http.MethodGet
		if body != "" {
			method = http.MethodPut
		}
		response := gatewaySettingsRequest(t, handler, method, "/api/v1/settings", body, http.StatusOK)
		var settings settingsResponse
		if err := json.NewDecoder(response.Body).Decode(&settings); err != nil {
			t.Fatal(err)
		}
		if settings.GatewayURL != wantURL || !reflect.DeepEqual(settings.AllowedHosts, wantHosts) {
			t.Fatalf("settings=%+v; want URL=%q hosts=%v", settings, wantURL, wantHosts)
		}
	}
	checkStatus := func(wantURL string) {
		t.Helper()
		response := gatewaySettingsRequest(t, handler, http.MethodGet, "/api/v1/status", "", http.StatusOK)
		var status statusResponse
		if err := json.NewDecoder(response.Body).Decode(&status); err != nil {
			t.Fatal(err)
		}
		if status.GatewayURL != wantURL || status.ProxyAddr != "0.0.0.0:8787" || status.AllowedHosts != 1 {
			t.Fatalf("unexpected status: %+v", status)
		}
	}
	checkSettings("", "", []string{"api.openai.com"})
	checkSettings(`{"gateway_url":" https://gateway.example.com/relay/// "}`, "https://gateway.example.com/relay", []string{"api.openai.com"})
	checkSettings(`{"allowed_hosts":["api.anthropic.com"]}`, "https://gateway.example.com/relay", []string{"api.anthropic.com"})
	checkStatus("https://gateway.example.com/relay")

	if err := eventStore.Close(); err != nil {
		t.Fatal(err)
	}
	handler, _ = startServer()
	checkSettings("", "https://gateway.example.com/relay", []string{"api.anthropic.com"})
	checkStatus("https://gateway.example.com/relay")
	checkSettings(`{"gateway_url":""}`, "", []string{"api.anthropic.com"})
	checkStatus("")
}

func TestInvalidGatewaySettingsLeaveSavedValuesUntouched(t *testing.T) {
	eventStore, err := store.Open(t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer eventStore.Close()
	proxy := gateway.NewProxy(config.Config{MaxBodyBytes: 1024, MaxRedactions: 10}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	defer proxy.Close()
	handler := NewServer("test-token", proxy, eventStore).Handler()
	gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings",
		`{"allowed_hosts":["api.openai.com"],"gateway_url":"https://gateway.example.com"}`, http.StatusOK)
	for _, body := range []string{
		`{"allowed_hosts":["changed.example.com"],"gateway_url":"ftp://bad.example.com"}`,
		`{"allowed_hosts":["https://bad.example.com"],"gateway_url":"https://changed.example.com"}`,
		`{"gateway_url":123}`, `{"gateway_url":null}`, `{}`, `{"unknown":true}`,
		`{"gateway_url":"https://changed.example.com"} {}`,
	} {
		t.Run(body, func(t *testing.T) {
			gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", body, http.StatusBadRequest)
			url, err := eventStore.LoadGatewayURL(context.Background())
			if err != nil || url != "https://gateway.example.com" {
				t.Fatalf("saved URL=%q, err=%v", url, err)
			}
			hosts, _, err := eventStore.LoadAllowedHosts(context.Background())
			if err != nil || !reflect.DeepEqual(hosts, []string{"api.openai.com"}) || !reflect.DeepEqual(proxy.AllowedHosts(), hosts) {
				t.Fatalf("saved hosts=%v, runtime hosts=%v, err=%v", hosts, proxy.AllowedHosts(), err)
			}
		})
	}
}
