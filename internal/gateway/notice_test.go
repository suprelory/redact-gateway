package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/redact"
	"github.com/suprelory/redact-gateway/internal/route"
)

func TestRequestNoticeRequiresRestorableMappings(t *testing.T) {
	for _, input := range []string{"hello", "{{RG_TYPE_TOKEN}}", "{{RG_EMAIL_ABCDEFGHIJKLMNOP}}", "alice@example.com"} {
		t.Run(input, func(t *testing.T) {
			proxy := &Proxy{cfg: config.Config{MaxBodyBytes: 1024, MaxRedactions: 10}}
			encoded, _ := json.Marshal(map[string]string{"input": input})
			request := httptest.NewRequest(http.MethodPost, "http://gateway/E$https://example.com/v1/responses", strings.NewReader(string(encoded)))
			request.Header.Set("Content-Type", "application/json")
			proxyRoute, err := route.ParseRequestURI(request.RequestURI)
			if err != nil {
				t.Fatal(err)
			}
			body, _, _, err := proxy.prepareRequestBody(request, proxyRoute, redact.NewContext(10))
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]string
			json.Unmarshal(body, &decoded)
			wantNotice := input == "alice@example.com"
			if strings.Contains(decoded["input"], redact.RedactNotice) != wantNotice || !wantNotice && decoded["input"] != input {
				t.Fatalf("unexpected notice injection: %s", body)
			}
		})
	}
}
