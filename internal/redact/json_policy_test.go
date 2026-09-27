package redact

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestJSONExemptionsStayWithinProtocolPaths(t *testing.T) {
	for _, test := range []struct {
		name     string
		protocol string
		input    string
		fields   []string
	}{
		{"generic control names", "generic", `{"model":"alice@example.com","data":{"id":"alice@example.com","name":"alice@example.com","type":"alice@example.com","format":"alice@example.com"},"metadata":{"image_url_description":"alice@example.com"},"source":{"data":"alice@example.com"}}`, []string{"$.model", "$.data.id", "$.data.name", "$.data.type", "$.data.format", "$.metadata.image_url_description", "$.source.data"}},
		{"chat controls and business data", "openai_chat", `{"model":"alice@example.com","messages":[{"role":"user","name":"alice@example.com","content":"alice@example.com"}],"data":{"id":"alice@example.com","name":"alice@example.com"},"image_url":{"url":"alice@example.com"}}`, []string{"$.messages[0].content", "$.data.id", "$.data.name", "$.image_url.url"}},
		{"chat media", "openai_chat", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"alice@example.com"},"caption":"alice@example.com"},{"type":"input_audio","input_audio":{"data":"alice@example.com","format":"wav"},"transcript":"alice@example.com"},{"type":"file","file":{"file_data":"alice@example.com","filename":"alice@example.com"}}]}]}`, []string{"$.messages[0].content[0].caption", "$.messages[0].content[1].transcript", "$.messages[0].content[2].file.filename"}},
		{"chat assistant history only", "openai_chat", `{"messages":[{"role":"assistant","reasoning":"alice@example.com","reasoning_content":{"text":"alice@example.com"},"reasoning_details":[{"signature":"alice@example.com"}],"content":"alice@example.com"},{"role":"user","reasoning_content":"alice@example.com"},{"role":"tool","reasoning_details":["alice@example.com"]}],"metadata":{"reasoning":"alice@example.com"}}`, []string{"$.messages[0].content", "$.messages[1].reasoning_content", "$.messages[2].reasoning_details[0]", "$.metadata.reasoning"}},
		{"chat tool calls", "openai_chat", `{"messages":[{"role":"assistant","tool_calls":[{"id":"alice@example.com","type":"function","function":{"name":"alice@example.com","arguments":"{\"name\":\"alice@example.com\"}"}}]},{"role":"tool","tool_call_id":"alice@example.com","content":"alice@example.com"}]}`, []string{"$.messages[0].tool_calls[0].function.arguments", "$.messages[1].content"}},
		{"anthropic assistant history only", "anthropic_messages", `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"alice@example.com","signature":{"data":"alice@example.com"}},{"type":"redacted_thinking","data":"alice@example.com"},{"type":"text","text":"alice@example.com"}]},{"role":"user","content":[{"type":"thinking","thinking":"alice@example.com","signature":"alice@example.com"}]}]}`, []string{"$.messages[0].content[2].text", "$.messages[1].content[0].thinking", "$.messages[1].content[0].signature"}},
		{"anthropic document sources", "anthropic_messages", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"base64","data":"alice@example.com","media_type":"image/png"}},{"type":"document","source":{"type":"url","url":"alice@example.com"},"title":"alice@example.com"},{"type":"document","source":{"type":"text","data":"alice@example.com"}}]}],"metadata":{"source":{"data":"alice@example.com"}}}`, []string{"$.messages[0].content[1].title", "$.messages[0].content[2].source.data", "$.metadata.source.data"}},
		{"anthropic nested media", "anthropic_messages", `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"alice@example.com","content":[{"type":"image","source":{"type":"url","url":"alice@example.com"}},{"type":"text","text":"alice@example.com"}]}]}]}`, []string{"$.messages[0].content[0].content[1].text"}},
		{"responses state and arguments", "openai_responses", `{"previous_response_id":"alice@example.com","input":[{"type":"reasoning","encrypted_content":"alice@example.com","summary":[{"text":"alice@example.com"}]},{"type":"compaction","encrypted_content":"alice@example.com"},{"type":"function_call","id":"alice@example.com","call_id":"alice@example.com","name":"alice@example.com","arguments":"{\"id\":\"alice@example.com\"}"},{"type":"message","role":"user","encrypted_content":"alice@example.com","content":[{"type":"input_image","image_url":"alice@example.com"},{"type":"input_text","text":"alice@example.com"}]}],"data":{"encrypted_content":"alice@example.com"}}`, []string{"$.input[2].arguments", "$.input[3].encrypted_content", "$.input[3].content[1].text", "$.data.encrypted_content"}},
		{"responses user state names", "openai_responses", `{"input":[{"type":"reasoning","role":"user","encrypted_content":"alice@example.com"}],"metadata":{"previous_response_id":"alice@example.com"}}`, []string{"$.input[0].encrypted_content", "$.metadata.previous_response_id"}},
		{"array-shaped object keys", "openai_chat", `{"messages":{"[0]":{"role":"assistant","reasoning_content":"alice@example.com","name":"alice@example.com"}}}`, []string{`$.messages["[0]"].reasoning_content`, `$.messages["[0]"].name`}},
		{"media types from another protocol", "anthropic_messages", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"alice@example.com"}},{"type":"input_file","file_data":"alice@example.com"},{"type":"input_audio","input_audio":{"data":"alice@example.com"}}]}]}`, []string{"$.messages[0].content[0].image_url.url", "$.messages[0].content[1].file_data", "$.messages[0].content[2].input_audio.data"}},
		{"tool block from another protocol", "openai_chat", `{"messages":[{"role":"user","content":[{"type":"tool_use","id":"alice@example.com","name":"alice@example.com"}]}]}`, []string{"$.messages[0].content[0].id", "$.messages[0].content[0].name"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var input any
			if err := json.Unmarshal([]byte(test.input), &input); err != nil {
				t.Fatal(err)
			}
			ctx := NewContext(100)
			out, err := RedactProtocolJSON(input, ctx, DetectorFlags{Email: true}, test.protocol)
			if err != nil {
				t.Fatal(err)
			}
			sort.Strings(test.fields)
			if got := ctx.RedactionFields(); !reflect.DeepEqual(got, test.fields) {
				t.Fatalf("fields = %#v, want %#v", got, test.fields)
			}
			if restored := RestoreJSON(out, ctx); !reflect.DeepEqual(restored, input) {
				t.Fatalf("round trip changed: %#v", restored)
			}
		})
	}
}

func TestSchemaDescriptionsAndValuesAreScanned(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{"alice@example.com": map[string]any{
			"type": "string", "format": "alice@example.com", "description": "alice@example.com",
			"default": "alice@example.com", "examples": []any{"alice@example.com"},
		}},
		"required": []any{"alice@example.com"},
		"$defs":    map[string]any{"inner": map[string]any{"type": "string", "default": "alice@example.com"}},
		"default":  map[string]any{"type": "alice@example.com", "name": "alice@example.com"},
	}
	for _, protocol := range []string{"openai_chat", "openai_responses", "anthropic_messages"} {
		t.Run(protocol, func(t *testing.T) {
			tool := map[string]any{"name": "alice@example.com", "description": "alice@example.com"}
			switch protocol {
			case "openai_chat":
				tool["parameters"] = schema
				tool = map[string]any{"type": "function", "function": tool}
			case "openai_responses":
				tool["type"], tool["parameters"] = "function", schema
			case "anthropic_messages":
				tool["input_schema"] = schema
			}
			input := map[string]any{"tools": []any{tool}}
			ctx := NewContext(100)
			out, err := RedactProtocolJSON(input, ctx, DetectorFlags{Email: true}, protocol)
			if err != nil {
				t.Fatal(err)
			}
			if ctx.RedactionCount() != 7 {
				t.Fatalf("expected seven descriptions/examples/defaults, got %d: %#v", ctx.RedactionCount(), ctx.RedactionFields())
			}
			encoded, _ := json.Marshal(out)
			if !strings.Contains(string(encoded), `"required":["alice@example.com"]`) || !strings.Contains(string(encoded), `"format":"alice@example.com"`) {
				t.Fatalf("schema references changed: %s", encoded)
			}
			if !reflect.DeepEqual(RestoreJSON(out, ctx), input) {
				t.Fatal("schema round trip changed")
			}
		})
	}
}

func TestCredentialFieldContext(t *testing.T) {
	input := map[string]any{
		"api_key": "low-entropy-value", "clientSecret": "short", "access_token": []any{"one", "two"},
		"data":    map[string]any{"password": "easy password", "name": "safe"},
		"content": `{"api_key":"short","data":{"password":"simple"},"tokens":["safe"]}`,
	}
	for _, enabled := range []bool{true, false} {
		ctx := NewContext(20)
		out, err := RedactJSON(input, ctx, DetectorFlags{Gitleaks: enabled})
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if enabled {
			want = 7
		}
		if ctx.RedactionCount() != want {
			t.Fatalf("field detection enabled=%v: %d hits, want %d", enabled, ctx.RedactionCount(), want)
		}
		if !reflect.DeepEqual(RestoreJSON(out, ctx), input) {
			t.Fatal("credential field round trip changed")
		}
	}
}
