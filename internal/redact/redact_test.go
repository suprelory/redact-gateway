package redact

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestAuditPaths(t *testing.T) {
	t.Parallel()
	secret := "alice@example.com"
	tests := []struct {
		name  string
		input any
		want  []string
	}{
		{"root string", secret, []string{"$"}},
		{"nested arrays", []any{[]any{secret, secret}}, []string{"$[0][0]", "$[0][1]"}},
		{"object keys", map[string]any{
			"a.b": secret, "[0]": secret, "": secret, "0": secret, `a"b`: secret,
			"safe_key": secret, "中文": secret,
		}, []string{`$["a.b"]`, `$["[0]"]`, `$[""]`, `$["0"]`, `$["a\"b"]`, "$.safe_key", `$["中文"]`}},
		{"tools and results", map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "input": map[string]any{"email": secret}},
			map[string]any{"type": "tool_result", "content": secret},
		}}, []string{"$.content[0].input.email", "$.content[1].content"}},
		{"no matches", map[string]any{"content": "hello"}, []string{}},
		{"protected fields", map[string]any{"model": secret, "image_url": map[string]any{"url": secret}}, []string{}},
		{"sensitive key", map[string]any{secret: map[string]any{"text": secret}}, []string{`$["<redacted-key>"].text`}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context := NewContext(20)
			output, err := RedactJSON(test.input, context, DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			sort.Strings(test.want)
			if got := context.RedactionFields(); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("paths = %#v, want %#v", got, test.want)
			}
			if restored := RestoreJSON(output, context); !reflect.DeepEqual(restored, test.input) {
				t.Fatalf("payload changed after round trip: %#v", restored)
			}
		})
	}
}

func TestAuditFieldSet(t *testing.T) {
	t.Parallel()
	context := NewContext(10)
	for _, path := range []string{"$.z", "$.a", "$.z"} {
		if _, err := context.RedactTextAtPath("alice@example.com", DetectorFlags{Email: true}, path); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"$.a", "$.z"}
	if got := context.RedactionFields(); !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
	if context.Hits()["EMAIL"] != 3 || context.RedactionCount() != 3 {
		t.Fatalf("repeated matches not counted: %#v", context.Hits())
	}
	copy := context.RedactionFields()
	copy[0] = "changed"
	if !reflect.DeepEqual(context.RedactionFields(), want) {
		t.Fatal("returned paths share mutable storage with context")
	}
}

func TestStructuredDetectorsAndRoundTrip(t *testing.T) {
	t.Parallel()
	context := NewContext(100)
	input := "mail alice@example.com phone 13800138000 id 11010519491231002X card 4111111111111111"
	redacted, err := context.RedactText(input, DetectorFlags{Email: true, Phone: true, Identity: true, Bank: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"alice@example.com", "13800138000", "11010519491231002X", "4111111111111111"} {
		if strings.Contains(redacted, raw) {
			t.Fatalf("redacted output still contains %q: %s", raw, redacted)
		}
	}
	if got := context.RestoreText(redacted); got != input {
		t.Fatalf("round trip mismatch:\nwant %s\n got %s", input, got)
	}
}

func TestPriorityKeepsConnectionStringWhole(t *testing.T) {
	t.Parallel()
	context := NewContext(10)
	input := "postgres://root:secret@10.0.0.8:5432/app"
	redacted, err := context.RedactText(input, DetectorFlags{Gitleaks: true, HighEntropy: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(redacted, "{{RG_CONNSTR_") || strings.Count(redacted, "{{RG_") != 1 {
		t.Fatalf("connection string should be one match: %s", redacted)
	}
}

func TestJSONWalkSkipsControlAndMultimodalFields(t *testing.T) {
	t.Parallel()
	context := NewContext(20)
	input := map[string]any{
		"model":     "alice@example.com",
		"messages":  []any{map[string]any{"role": "user", "content": "alice@example.com"}},
		"image_url": map[string]any{"url": "https://example.com/alice@example.com.png"},
		"tool":      map[string]any{"arguments": `{"email":"alice@example.com"}`},
	}
	output, err := RedactJSON(input, context, DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(output)
	text := string(encoded)
	if !strings.Contains(text, `"model":"alice@example.com"`) {
		t.Fatalf("model field changed: %s", text)
	}
	if !strings.Contains(text, "https://example.com/alice@example.com.png") {
		t.Fatalf("image URL changed: %s", text)
	}
	if strings.Count(text, "{{RG_EMAIL_") != 2 {
		t.Fatalf("expected prompt and tool argument redactions: %s", text)
	}
	wantFields := []string{"$.messages[0].content", "$.tool.arguments"}
	if fields := context.RedactionFields(); !reflect.DeepEqual(fields, wantFields) {
		t.Fatalf("redaction fields = %#v, want %#v", fields, wantFields)
	}
}

func TestExistingPlaceholderIsNotRedacted(t *testing.T) {
	t.Parallel()
	context := NewContext(10)
	placeholder := "{{RG_EMAIL_ABCDEFGHIJKLMNOP}}"
	output, err := context.RedactText(placeholder+" alice@example.com", DetectorFlags{Email: true, HighEntropy: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(output, placeholder+" ") {
		t.Fatalf("existing placeholder changed: %s", output)
	}
}
