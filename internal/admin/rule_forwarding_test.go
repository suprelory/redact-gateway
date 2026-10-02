package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/suprelory/redact-gateway/internal/route"
)

func ruleEchoUpstream(t *testing.T) (string, <-chan string) {
	t.Helper()
	received := make(chan string, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			http.Error(writer, "read failed", http.StatusBadRequest)
			return
		}
		received <- string(body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	t.Cleanup(upstream.Close)
	return upstream.URL, received
}

func TestEachRuleCanBeDisabledAndReenabledThroughAPI(t *testing.T) {
	for _, sample := range []struct{ flag, field, value, label string }{
		{"H", "value", "qzmxwkjvbnfrtypshgdclaeuiozxqwvbnm", "ENTROPY"},
		{"P", "value", "13800138000", "PHONE"},
		{"S", "value", "sk-abcdefghijklmnopqrstuvwxyz1234567890", "APIKEY"},
		{"I", "value", "11010519491231002X", "IDCARD"},
		{"B", "value", "4111111111111111", "CARD"},
		{"E", "value", "alice@example.com", "EMAIL"},
		{"G", "password", "correct horse battery staple", "CREDENTIAL"},
	} {
		t.Run(sample.flag, func(t *testing.T) {
			handler, proxy, eventStore := newRulesServer(t, t.TempDir())
			upstream, received := ruleEchoUpstream(t)
			input := map[string]string{sample.field: sample.value}
			body, _ := json.Marshal(input)
			for index, enabled := range []string{sample.flag, "", sample.flag} {
				gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings",
					`{"enabled_rules":"`+enabled+`"}`, http.StatusOK)
				prefix := route.AllFlagLetters
				if index == 0 {
					prefix = "" // Empty URL flags inherit the configured selection.
				}
				request := httptest.NewRequest(http.MethodPost, "http://gateway/"+prefix+"$"+upstream+"/v1", strings.NewReader(string(body)))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				proxy.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					t.Fatalf("enabled=%q: status=%d, body=%s", enabled, response.Code, response.Body.String())
				}
				forwarded := <-received
				wantCount := 0
				if enabled != "" {
					wantCount = 1
				}
				if strings.Contains(forwarded, sample.value) != (wantCount == 0) || strings.Contains(forwarded, "{{RG_") != (wantCount == 1) {
					t.Fatalf("enabled=%q: unexpected upstream body: %s", enabled, forwarded)
				}
				var restored map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &restored); err != nil || !reflect.DeepEqual(restored, input) {
					t.Fatalf("response was not restored: %s, err=%v", response.Body.String(), err)
				}
				events, err := eventStore.Events(context.Background(), 1, "")
				if err != nil || len(events) != 1 {
					t.Fatalf("events=%v, err=%v", events, err)
				}
				event := events[0]
				if event.Flags != enabled || event.RedactionCount != wantCount || event.RestoreCount != wantCount || event.RuleHits[sample.label] != wantCount {
					t.Fatalf("enabled=%q: incorrect audit record: %+v", enabled, event)
				}
			}
		})
	}
}

func TestURLSelectionAndNestedJSONRespectRuleSettings(t *testing.T) {
	handler, proxy, eventStore := newRulesServer(t, t.TempDir())
	upstream, received := ruleEchoUpstream(t)
	gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", `{"enabled_rules":"E"}`, http.StatusOK)
	const body = `{"phone":13800138000,"password":"ordinary words","content":"{\"email\":\"alice@example.com\"}"}`
	for _, selection := range []struct{ requested, effective string }{{"", "E"}, {"PE", "E"}, {"P", ""}} {
		request := httptest.NewRequest(http.MethodPost, "http://gateway/"+selection.requested+"$"+upstream+"/v1", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		proxy.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("disabled phone rule still blocked numeric input: %d, %s", response.Code, response.Body.String())
		}
		forwarded := <-received
		if !strings.Contains(forwarded, "13800138000") || !strings.Contains(forwarded, "ordinary words") ||
			strings.Contains(forwarded, "alice@example.com") != (selection.effective == "") {
			t.Fatalf("requested=%q: unexpected upstream body: %s", selection.requested, forwarded)
		}
		events, err := eventStore.Events(context.Background(), 1, "")
		if err != nil || len(events) != 1 || events[0].Flags != selection.effective {
			t.Fatalf("incorrect effective flags: events=%v, err=%v", events, err)
		}
	}
}

func TestDisabledNumericRulesDoNotBlockRequests(t *testing.T) {
	handler, proxy, _ := newRulesServer(t, t.TempDir())
	upstream, received := ruleEchoUpstream(t)
	for _, sample := range []struct{ flag, number string }{
		{"P", "13800138000"}, {"I", "110105194912310038"}, {"B", "4111111111111111"},
	} {
		for _, enabled := range []string{sample.flag, ""} {
			gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings",
				`{"enabled_rules":"`+enabled+`"}`, http.StatusOK)
			body := `{"value":` + sample.number + `}`
			request := httptest.NewRequest(http.MethodPost, "http://gateway/"+sample.flag+"$"+upstream+"/v1", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			proxy.ServeHTTP(response, request)
			if enabled == "" {
				if response.Code != http.StatusOK {
					t.Fatalf("disabled %s: status=%d, body=%s", sample.flag, response.Code, response.Body.String())
				}
				if forwarded := <-received; forwarded != body {
					t.Fatalf("disabled %s changed numeric value: %s", sample.flag, forwarded)
				}
			} else {
				if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "numeric_sensitive_value") {
					t.Fatalf("enabled %s failed to block numeric value: %d, %s", sample.flag, response.Code, response.Body.String())
				}
				select {
				case <-received:
					t.Fatal("numeric PII reached upstream while its rule was enabled")
				default:
				}
			}
		}
	}
}

type pausedRuleBody struct {
	io.ReadCloser
	started chan struct{}
	resume  chan struct{}
	once    sync.Once
}

func (body *pausedRuleBody) Read(buffer []byte) (int, error) {
	body.once.Do(func() {
		close(body.started)
		<-body.resume
	})
	return body.ReadCloser.Read(buffer)
}

func TestRuleChangePreservesInFlightDetectionAndRestoration(t *testing.T) {
	handler, proxy, eventStore := newRulesServer(t, t.TempDir())
	upstream, received := ruleEchoUpstream(t)
	gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", `{"enabled_rules":"E"}`, http.StatusOK)
	request := httptest.NewRequest(http.MethodPost, "http://gateway/$"+upstream+"/v1", strings.NewReader(`{"value":"alice@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	body := &pausedRuleBody{ReadCloser: request.Body, started: make(chan struct{}), resume: make(chan struct{})}
	request.Body = body
	response := httptest.NewRecorder()
	done := make(chan struct{})
	var resume sync.Once
	defer func() {
		resume.Do(func() { close(body.resume) })
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("request did not finish")
		}
	}()
	go func() {
		proxy.ServeHTTP(response, request)
		close(done)
	}()
	select {
	case <-body.started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach its body")
	}
	gatewaySettingsRequest(t, handler, http.MethodPut, "/api/v1/settings", `{"enabled_rules":""}`, http.StatusOK)
	resume.Do(func() { close(body.resume) })
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not finish after the rule update")
	}
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "alice@example.com") {
		t.Fatalf("in-flight response was not restored: %d, %s", response.Code, response.Body.String())
	}
	if forwarded := <-received; strings.Contains(forwarded, "alice@example.com") || !strings.Contains(forwarded, "{{RG_EMAIL_") {
		t.Fatalf("in-flight request lost its rule snapshot: %s", forwarded)
	}
	events, err := eventStore.Events(context.Background(), 1, "")
	if err != nil || len(events) != 1 || events[0].Flags != "E" || events[0].RestoreCount != 1 || proxy.EnabledRules().Raw != "" {
		t.Fatalf("unexpected snapshot audit or current settings: events=%v, rules=%q, err=%v", events, proxy.EnabledRules().Raw, err)
	}
}
