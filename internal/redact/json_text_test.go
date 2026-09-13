package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

const testPrivateKey = "-----BEGIN PRIVATE KEY-----\nFAKE-TEST-DATA \"quoted\" C:\\test\\key\n-----END PRIVATE KEY-----"

func TestToolArgumentsUseDecodedMappings(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"key": testPrivateKey, "nested": []any{"alice@example.com"}})
	ctx := NewContext(10)
	input := map[string]any{
		"input": []any{map[string]any{"type": "function_call", "arguments": string(args)}},
	}
	masked, err := RedactJSON(input, ctx, DetectorFlags{Gitleaks: true, Email: true})
	if err != nil {
		t.Fatal(err)
	}
	maskedArgs := masked.(map[string]any)["input"].([]any)[0].(map[string]any)["arguments"].(string)
	var values map[string]any
	if err := json.Unmarshal([]byte(maskedArgs), &values); err != nil {
		t.Fatal(err)
	}
	token := values["key"].(string)
	if !placeholderPattern.MatchString(token) || strings.Contains(maskedArgs, "FAKE-TEST-DATA") {
		t.Fatalf("tool arguments not masked: %s", maskedArgs)
	}
	if got := ctx.RestoreText(token); got != testPrivateKey {
		t.Fatalf("mapping contains JSON escapes instead of the original: %q", got)
	}
	response := map[string]any{"output": []any{map[string]any{"type": "function_call", "arguments": maskedArgs}}}
	restored := RestoreJSON(response, ctx).(map[string]any)["output"].([]any)[0].(map[string]any)["arguments"].(string)
	var roundTrip map[string]any
	if err := json.Unmarshal([]byte(restored), &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip["key"] != testPrivateKey || roundTrip["nested"].([]any)[0] != "alice@example.com" {
		t.Fatalf("tool argument values changed: %#v", roundTrip)
	}
	if ctx.RedactionCount() != 2 || len(ctx.RedactionFields()) != 1 || ctx.RedactionFields()[0] != "$.input[0].arguments" {
		t.Fatalf("unexpected audit counts or paths: %d %#v", ctx.RedactionCount(), ctx.RedactionFields())
	}
}

func TestToolJSONPreservesShapeAndUntouchedEscapes(t *testing.T) {
	ctx := NewContext(10)
	input := ` { "email" : "\u0061lice@example.com", "n":9007199254740993, "untouched":"\u0061", "alice@example.com":true } `
	masked, err := RedactJSON(map[string]any{"arguments": input}, ctx, DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	args := masked.(map[string]any)["arguments"].(string)
	if !strings.Contains(args, `"n":9007199254740993, "untouched":"\u0061", "alice@example.com":true`) {
		t.Fatalf("unrelated JSON formatting or values changed: %s", args)
	}
	if ctx.RedactionCount() != 1 {
		t.Fatalf("object keys should not be masked: %d", ctx.RedactionCount())
	}
	if got := RestoreJSON(map[string]any{"arguments": args}, ctx).(map[string]any)["arguments"].(string); !strings.Contains(got, `"email" : "alice@example.com"`) {
		t.Fatalf("decoded token did not restore: %s", got)
	}
}

func TestToolArgumentsEscapeOriginalInJSONAndSSE(t *testing.T) {
	for _, protocol := range []string{"responses", "chat", "anthropic"} {
		t.Run(protocol, func(t *testing.T) {
			for split := 1; split < 48; split++ {
				ctx := NewContext(10)
				token, err := ctx.RedactText(testPrivateKey, DetectorFlags{Gitleaks: true})
				if err != nil {
					t.Fatal(err)
				}
				args := `{"key":"` + token + `","path":"C:\\test"}`
				if split >= len(args) {
					break
				}
				var events []any
				for _, part := range []string{args[:split], args[split:]} {
					switch protocol {
					case "responses":
						events = append(events, map[string]any{"type": "response.function_call_arguments.delta", "output_index": 0, "delta": part})
					case "chat":
						events = append(events, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": part}}}}}}})
					case "anthropic":
						events = append(events, map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "input_json_delta", "partial_json": part}})
					}
				}
				var joined strings.Builder
				for _, event := range restoreTestEvents(t, ctx, events...) {
					switch protocol {
					case "responses":
						joined.WriteString(event["delta"].(string))
					case "chat":
						choice := event["choices"].([]any)[0].(map[string]any)
						call := choice["delta"].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)
						joined.WriteString(call["function"].(map[string]any)["arguments"].(string))
					case "anthropic":
						joined.WriteString(event["delta"].(map[string]any)["partial_json"].(string))
					}
				}
				var parsed map[string]string
				if err := json.Unmarshal([]byte(joined.String()), &parsed); err != nil {
					t.Fatalf("split %d produced invalid tool JSON: %v", split, err)
				}
				if parsed["key"] != testPrivateKey || parsed["path"] != `C:\test` || ctx.RestoreCount() != 1 {
					t.Fatalf("split %d changed arguments or counts: %#v / %d", split, parsed, ctx.RestoreCount())
				}
			}
		})
	}
}

func TestToolArgumentSnapshotsEscapeOnlyOnce(t *testing.T) {
	ctx := NewContext(10)
	token, err := ctx.RedactText(testPrivateKey, DetectorFlags{Gitleaks: true})
	if err != nil {
		t.Fatal(err)
	}
	args := `{"key":"` + token + `"}`
	for _, response := range []map[string]any{
		{"type": "response.function_call_arguments.done", "arguments": args},
		{"type": "response.completed", "response": map[string]any{"output": []any{map[string]any{"type": "function_call", "arguments": args}}}},
	} {
		event := restoreTestEvents(t, ctx, response)[0]
		value, ok := event["arguments"].(string)
		if !ok {
			value = event["response"].(map[string]any)["output"].([]any)[0].(map[string]any)["arguments"].(string)
		}
		var parsed map[string]string
		if err := json.Unmarshal([]byte(value), &parsed); err != nil || parsed["key"] != testPrivateKey {
			t.Fatalf("snapshot arguments changed: %#v / %v", parsed, err)
		}
	}
}
