package redact

import (
	"testing"
)

func TestResponsesOpaqueFieldsKeepProtocolValues(t *testing.T) {
	for _, protocol := range []string{"openai_responses", "generic"} {
		ctx := NewContext(20)
		body := map[string]any{
			"previous_response_id": "resp_Dh2n7aBPm5Jq3N6Lw4Uv8Ee9RzFc0XyK",
			"input":                []any{map[string]any{"type": "reasoning", "encrypted_content": "gAAAAABmZGF0YQ5pOyd7wxEdGhciFzx1OeqSFD0vGzjMK2QYpHVNeRjYAZv0"}},
			"tool":                 map[string]any{"arguments": `{"previous_response_id":"alice@example.com","encrypted_content":"bob@example.com"}`},
		}
		out, err := RedactProtocolJSON(body, ctx, DetectorFlags{HighEntropy: true, Email: true}, protocol)
		if err != nil {
			t.Fatal(err)
		}
		mapped := out.(map[string]any)
		unchanged := mapped["previous_response_id"] == body["previous_response_id"] && mapped["input"].([]any)[0].(map[string]any)["encrypted_content"] == body["input"].([]any)[0].(map[string]any)["encrypted_content"]
		if unchanged != (protocol == "openai_responses") || mapped["tool"].(map[string]any)["arguments"] == body["tool"].(map[string]any)["arguments"] {
			t.Fatalf("opaque protection escaped its protocol or suppressed tool masking: %#v", out)
		}
	}
}
