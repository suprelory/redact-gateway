package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/gateway"
	"github.com/suprelory/redact-gateway/internal/route"
	"github.com/suprelory/redact-gateway/internal/store"
)

func newRulesServer(t *testing.T, dataDir string) (http.Handler, *gateway.Proxy, *store.Store) {
	t.Helper()
	eventStore, err := store.Open(dataDir, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eventStore.Close() })
	proxy := gateway.NewProxy(config.Config{
		AllowPrivateHosts: true, MaxBodyBytes: 1024 * 1024, MaxRedactions: 100,
	}, eventStore, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	t.Cleanup(proxy.Close)
	raw, found, err := eventStore.LoadEnabledRules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if found {
		flags, err := route.ParseEnabledFlags(raw)
		if err != nil {
			t.Fatal(err)
		}
		proxy.SetEnabledRules(flags)
	}
	return NewServer("test-token", proxy, eventStore).Handler(), proxy, eventStore
}

func checkRuleSettings(t *testing.T, handler http.Handler, proxy *gateway.Proxy, want string) {
	t.Helper()
	response := gatewaySettingsRequest(t, handler, http.MethodGet, "/api/v1/settings", "", http.StatusOK)
	var settings settingsResponse
	if err := json.NewDecoder(response.Body).Decode(&settings); err != nil {
		t.Fatal(err)
	}
	if settings.EnabledRules != want || proxy.EnabledRules().Raw != want {
		t.Fatalf("settings=%q, runtime=%q; want %q", settings.EnabledRules, proxy.EnabledRules().Raw, want)
	}
	response = gatewaySettingsRequest(t, handler, http.MethodGet, "/api/v1/rules", "", http.StatusOK)
	var rules struct {
		AllFlags     string     `json:"all_flags"`
		EnabledRules string     `json:"enabled_rules"`
		Rules        []ruleInfo `json:"rules"`
	}
	if err := json.NewDecoder(response.Body).Decode(&rules); err != nil {
		t.Fatal(err)
	}
	if rules.AllFlags != route.AllFlagLetters || rules.EnabledRules != want || len(rules.Rules) != len(route.AllFlagLetters) {
		t.Fatalf("unexpected rule catalogue: %+v", rules)
	}
	for _, rule := range rules.Rules {
		if !rule.Default || rule.Enabled != strings.Contains(want, rule.Flag) {
			t.Fatalf("incorrect rule state: %+v; enabled=%q", rule, want)
		}
	}
}

func TestRuleSettingsPersistIncludingAllDisabled(t *testing.T) {
	for _, selection := range []string{"EPPE", ""} {
		t.Run("selection="+selection, func(t *testing.T) {
			dataDir := t.TempDir()
			handler, proxy, eventStore := newRulesServer(t, dataDir)
			checkRuleSettings(t, handler, proxy, route.AllFlagLetters)
			gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings",
				`{"gateway_url":"https://gateway.example.com","enabled_rules":"`+selection+`"}`, http.StatusOK)
			want, _ := route.ParseEnabledFlags(selection)
			checkRuleSettings(t, handler, proxy, want.Raw)
			// Independent forms must not reset the saved rule switches.
			gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", `{"allowed_hosts":[]}`, http.StatusOK)
			checkRuleSettings(t, handler, proxy, want.Raw)
			if err := eventStore.Close(); err != nil {
				t.Fatal(err)
			}
			handler, proxy, eventStore = newRulesServer(t, dataDir)
			checkRuleSettings(t, handler, proxy, want.Raw)
			gatewayURL, err := eventStore.LoadGatewayURL(context.Background())
			if err != nil || gatewayURL != "https://gateway.example.com" {
				t.Fatalf("unrelated setting changed: URL=%q, err=%v", gatewayURL, err)
			}
		})
	}
}

func TestInvalidRuleSettingsDoNotChangeOtherSettings(t *testing.T) {
	handler, proxy, eventStore := newRulesServer(t, t.TempDir())
	gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings",
		`{"enabled_rules":"PE","allowed_hosts":["api.example.com"]}`, http.StatusOK)
	for _, body := range []string{
		`{"enabled_rules":"EZ","allowed_hosts":[]}`, `{"enabled_rules":"e"}`,
		`{"enabled_rules":true}`, `{"enabled_rules":123}`, `{"enabled_rules":[]}`,
		`{"enabled_rules":null}`, `{"enabled_rules":""} {}`, `{"enabled_rules":"","unknown":true}`,
		`{"enabled_rules":"","gateway_url":"ftp://bad.example.com"}`,
	} {
		gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", body, http.StatusBadRequest)
		checkRuleSettings(t, handler, proxy, "PE")
		rules, found, err := eventStore.LoadEnabledRules(context.Background())
		if err != nil || !found || rules != "PE" || !reflect.DeepEqual(proxy.AllowedHosts(), []string{"api.example.com"}) {
			t.Fatalf("invalid update changed settings: rules=%q, found=%v, hosts=%v, err=%v", rules, found, proxy.AllowedHosts(), err)
		}
	}
}

func TestFailedRuleSaveLeavesRuntimeUnchanged(t *testing.T) {
	dataDir := t.TempDir()
	handler, proxy, eventStore := newRulesServer(t, dataDir)
	gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", `{"enabled_rules":"PE"}`, http.StatusOK)
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "gateway.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER fail_rule_save BEFORE INSERT ON settings
WHEN NEW.key = 'enabled_rules' BEGIN SELECT RAISE(ABORT, 'save failed'); END`); err != nil {
		t.Fatal(err)
	}
	response := gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings",
		`{"enabled_rules":"","allowed_hosts":["changed.example.com"]}`, http.StatusInternalServerError)
	if !strings.Contains(response.Body.String(), "settings_save_failed") {
		t.Fatalf("unexpected save error: %s", response.Body.String())
	}
	checkRuleSettings(t, handler, proxy, "PE")
	rules, _, err := eventStore.LoadEnabledRules(context.Background())
	if err != nil || rules != "PE" || len(proxy.AllowedHosts()) != 0 {
		t.Fatalf("failed save changed settings: rules=%q, hosts=%v, err=%v", rules, proxy.AllowedHosts(), err)
	}
}

func TestRuleSettingsRequireAuthentication(t *testing.T) {
	handler, proxy, _ := newRulesServer(t, t.TempDir())
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		path := "/api/v1/rules"
		if method == http.MethodPut {
			path = "/api/v1/settings"
		}
		request := httptest.NewRequest(method, "http://gateway"+path, strings.NewReader(`{"enabled_rules":""}`))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized || proxy.EnabledRules().Raw != route.AllFlagLetters {
			t.Fatalf("unauthenticated %s: status=%d, rules=%q", method, response.Code, proxy.EnabledRules().Raw)
		}
	}
}
