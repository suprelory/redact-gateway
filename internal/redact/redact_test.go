package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

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
