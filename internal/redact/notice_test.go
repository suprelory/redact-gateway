package redact

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestNoticePreservesMessageShapeAndDoesNotRepeat(t *testing.T) {
	for _, test := range []struct{ protocol, body string }{
		{"openai_responses", `{"instructions":"Keep the reply brief","input":"original task"}`},
		{"openai_responses", `{"input":[{"role":"developer","content":"Keep the reply brief"},{"role":"user","content":[{"type":"input_text","text":"original task"},{"type":"input_image","image_url":"https://example.com/image.png"}]}]}`},
		{"openai_chat", `{"messages":[{"role":"system","content":"Keep the reply brief"},{"role":"user","content":"earlier task"},{"role":"assistant","content":"earlier answer"},{"role":"user","content":"original task"}]}`},
		{"anthropic_messages", `{"system":"Keep the reply brief","messages":[{"role":"user","content":[{"type":"text","text":"original task"},{"type":"tool_result","tool_use_id":"call_1","content":"tool output"}]}]}`},
		{"anthropic_messages", `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"call_1","content":"original task"}]}]}`},
	} {
		t.Run(test.protocol+test.body[:10], func(t *testing.T) {
			var body any
			if err := json.Unmarshal([]byte(test.body), &body); err != nil {
				t.Fatal(err)
			}
			if !InjectNotice(body, test.protocol) {
				t.Fatal("notice was not attached to a user message")
			}
			first, _ := json.Marshal(body)
			InjectNotice(body, test.protocol)
			second, _ := json.Marshal(body)
			if string(first) != string(second) || strings.Count(string(second), RedactNotice) != 1 || !strings.Contains(string(second), "original task") || strings.Contains(RedactNotice, "{{RG_") {
				t.Fatalf("notice changed original task or repeated: %s", second)
			}
			if strings.Contains(test.body, "Keep the reply brief") && !strings.Contains(string(second), `"content":"Keep the reply brief"`) && !strings.Contains(string(second), `"instructions":"Keep the reply brief"`) && !strings.Contains(string(second), `"system":"Keep the reply brief"`) {
				t.Fatal("higher-priority instructions changed")
			}
		})
	}
}

func TestNoticeDoesNotOverwriteUnsupportedContent(t *testing.T) {
	for _, input := range []string{`{"messages":[{"role":"user","content":{"custom":"preserve"}}]}`, `{"input":[{"type":"function_call_output","call_id":"call_1","output":"preserve"}]}`} {
		var body, original any
		json.Unmarshal([]byte(input), &body)
		json.Unmarshal([]byte(input), &original)
		InjectNotice(body, "openai_responses")
		if !reflect.DeepEqual(body, original) {
			t.Fatalf("unsupported content changed: %#v", body)
		}
	}
}
