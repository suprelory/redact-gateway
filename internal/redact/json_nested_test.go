package redact

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNestedJSONUsesDecodedValuesAndEscapesEachLayer(t *testing.T) {
	leaf := ` { "email" : "alice\u0040example.com", "api_key":"short", "n":9007199254740993, "untouched":"\u0061" } `
	encoded, _ := json.Marshal(map[string]any{"payload": leaf, "key": testPrivateKey})
	input := map[string]any{"content": string(encoded)}
	ctx := NewContext(20)
	out, err := RedactJSON(input, ctx, DetectorFlags{Email: true, Gitleaks: true})
	if err != nil {
		t.Fatal(err)
	}
	var outer map[string]string
	if err := json.Unmarshal([]byte(out.(map[string]any)["content"].(string)), &outer); err != nil {
		t.Fatal(err)
	}
	var inner map[string]any
	if err := json.Unmarshal([]byte(outer["payload"]), &inner); err != nil {
		t.Fatal(err)
	}
	if inner["email"] == "alice@example.com" || inner["api_key"] == "short" || !placeholderPattern.MatchString(outer["key"]) {
		t.Fatalf("nested values escaped detection: %#v / %#v", outer, inner)
	}
	if !strings.Contains(outer["payload"], `"n":9007199254740993, "untouched":"\u0061"`) {
		t.Fatalf("untouched JSON lexemes changed: %s", outer["payload"])
	}
	if !reflect.DeepEqual(ctx.RedactionFields(), []string{"$.content"}) || ctx.RedactionCount() != 3 {
		t.Fatalf("nested JSON audit path or counts changed: %#v / %d", ctx.RedactionFields(), ctx.RedactionCount())
	}
	restored := RestoreJSON(out, ctx).(map[string]any)["content"].(string)
	if err := json.Unmarshal([]byte(restored), &outer); err != nil {
		t.Fatal(err)
	}
	if outer["key"] != testPrivateKey {
		t.Fatalf("private key escaping changed: %q", outer["key"])
	}
	if err := json.Unmarshal([]byte(outer["payload"]), &inner); err != nil || inner["email"] != "alice@example.com" || inner["api_key"] != "short" {
		t.Fatalf("nested round trip failed: %#v / %v", inner, err)
	}
}

func TestJSONNestingLimits(t *testing.T) {
	for _, count := range []int{maxJSONDepth, maxJSONDepth + 1} {
		var value any = "alice@example.com"
		for index := 0; index < count; index++ {
			value = []any{value}
		}
		_, err := RedactJSON(value, NewContext(10), DetectorFlags{Email: true})
		if errors.Is(err, ErrJSONDepthLimit) != (count > maxJSONDepth) {
			t.Fatalf("structural depth %d: %v", count, err)
		}
	}
	for _, count := range []int{maxEncodedJSONDepth, maxEncodedJSONDepth + 1} {
		value := `{"email":"alice\u0040example.com"}`
		for index := 1; index < count; index++ {
			encoded, _ := json.Marshal(map[string]any{"payload": value})
			value = string(encoded)
		}
		ctx := NewContext(10)
		_, err := RedactJSON(map[string]any{"content": value}, ctx, DetectorFlags{Email: true})
		if errors.Is(err, ErrJSONDepthLimit) != (count > maxEncodedJSONDepth) {
			t.Fatalf("encoded depth %d: %v", count, err)
		}
		if count <= maxEncodedJSONDepth && ctx.RedactionCount() != 1 {
			t.Fatal("permitted nested JSON was not scanned")
		}
	}
	for _, count := range []int{maxJSONDepth, 10001} {
		text := strings.Repeat("[", count) + `"alice\u0040example.com"` + strings.Repeat("]", count)
		_, err := RedactJSON(map[string]any{"content": text}, NewContext(10), DetectorFlags{Email: true})
		if !errors.Is(err, ErrJSONDepthLimit) {
			t.Fatalf("encoded structural depth %d was not rejected: %v", count, err)
		}
	}
}

func TestEncodedJSONHasNoProtocolExemptions(t *testing.T) {
	input := map[string]any{"messages": []any{map[string]any{"role": "user", "content": `{"model":"alice\u0040example.com","image_url":{"url":"alice\u0040example.com"},"messages":[{"role":"assistant","reasoning_content":"alice\u0040example.com"}]}`}}}
	ctx := NewContext(20)
	_, err := RedactProtocolJSON(input, ctx, DetectorFlags{Email: true}, "openai_chat")
	if err != nil || ctx.RedactionCount() != 3 {
		t.Fatalf("serialized business data inherited protocol exemptions: %d / %v", ctx.RedactionCount(), err)
	}
}
