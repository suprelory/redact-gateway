package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func placeholderForms() map[string]func(string) string {
	return map[string]func(string) string{
		"exact":             func(s string) string { return s },
		"bare":              func(s string) string { return s[2 : len(s)-2] },
		"single braces":     func(s string) string { return s[1 : len(s)-1] },
		"one closing brace": func(s string) string { return s[:len(s)-1] },
		"no closing braces": func(s string) string { return s[:len(s)-2] },
		"no opening braces": func(s string) string { return s[2:] },
		"escaped":           func(s string) string { return strings.NewReplacer("{", `\{`, "}", `\}`).Replace(s) },
		"double escaped":    func(s string) string { return strings.NewReplacer("{", `\\{`, "}", `\\}`).Replace(s) },
		"triple escaped":    func(s string) string { return strings.NewReplacer("{", `\\\{`, "}", `\\\}`).Replace(s) },
		"label lowercase":   func(s string) string { return strings.Replace(s, "EMAIL", "email", 1) },
		"all lowercase":     strings.ToLower,
		"label underscore":  func(s string) string { return strings.Replace(s, "EMAIL", "EM_AIL", 1) },
	}
}

func TestKnownPlaceholderFormsAcrossEveryStreamSplit(t *testing.T) {
	for name, form := range placeholderForms() {
		t.Run(name, func(t *testing.T) {
			for split := 1; ; split++ {
				ctx := NewContext(10)
				token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
				if err != nil {
					t.Fatal(err)
				}
				input := "before " + form(token) + " after"
				if split >= len(input) {
					break
				}
				var parts []any
				for _, part := range []string{input[:split], input[split:]} {
					parts = append(parts, map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": part})
				}
				var joined strings.Builder
				for _, event := range restoreTestEvents(t, ctx, parts...) {
					joined.WriteString(event["delta"].(string))
				}
				if joined.String() != "before alice@example.com after" {
					t.Fatalf("split %d: %q", split, joined.String())
				}
				degraded := 1
				if name == "exact" {
					degraded = 0
				}
				if ctx.RestoreCount() != 1 || ctx.RestoreUniqueCount() != 1 || ctx.DegradedCount() != degraded || ctx.UnresolvedCount() != 0 {
					t.Fatalf("split %d: counts=%d/%d/%d/%d", split, ctx.RestoreCount(), ctx.RestoreUniqueCount(), ctx.DegradedCount(), ctx.UnresolvedCount())
				}
			}
		})
	}
}

func TestUnknownPlaceholderFormsAreNotGuessed(t *testing.T) {
	for name, form := range placeholderForms() {
		t.Run(name, func(t *testing.T) {
			ctx := NewContext(10)
			token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			unknown := strings.Replace(token, "EMAIL", "PHONE", 1)
			input := form(unknown)
			if got := ctx.RestoreText(input); got != input || ctx.RestoreCount() != 0 || ctx.DegradedCount() != 0 || ctx.UnresolvedCount() != 1 {
				t.Fatalf("unknown or renamed token changed: %q", got)
			}
		})
	}
}

func TestRestorationPreservesPathsAndIdentifierBoundaries(t *testing.T) {
	ctx := NewContext(10)
	token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, shape := range []string{token, strings.ToLower(token), token[1 : len(token)-1]} {
		input := `C:\` + shape + `\file`
		if got := ctx.RestoreText(input); got != `C:\alice@example.com\file` {
			t.Fatalf("path separator changed: %q", got)
		}
	}
	bare := token[2 : len(token)-2]
	for _, input := range []string{"prefix_" + bare, bare + "_backup", bare + "A", "{{RG_TYPE_TOKEN}}", "HTTP_status"} {
		if got := ctx.RestoreText(input); got != input {
			t.Fatalf("ordinary identifier changed: %q", got)
		}
	}
	if ctx.RestoreCount() != 3 || ctx.UnresolvedCount() != 0 {
		t.Fatal("ordinary text affected diagnostic counts")
	}
}

func TestKnownPlaceholderFormsAreProtectedOnInput(t *testing.T) {
	for name, form := range placeholderForms() {
		t.Run(name, func(t *testing.T) {
			ctx := NewContext(10)
			token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			input := form(token)
			got, err := ctx.RedactText(input, DetectorFlags{HighEntropy: true, Email: true, Gitleaks: true})
			if err != nil || got != input || ctx.RedactionCount() != 1 {
				t.Fatalf("existing token remasked: %q (%v)", got, err)
			}
		})
	}
}

func TestRestorationDoesNotRescanInsertedOriginal(t *testing.T) {
	ctx := NewContext(10)
	token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := "literal example " + token
	outer, err := ctx.tokenFor("SECRET", raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := ctx.RestoreText(outer); got != raw || ctx.RestoreCount() != 1 {
		t.Fatalf("original was recursively rewritten: %q", got)
	}
}

func TestBareIdentifiersStayIntactAcrossStreamSplits(t *testing.T) {
	for _, format := range []string{"prefix_%s suffix", "prefix%s", "%s_backup", "%sA"} {
		for split := 1; ; split++ {
			ctx := NewContext(10)
			token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			input := strings.Replace(format, "%s", token[2:len(token)-2], 1)
			if split >= len(input) {
				break
			}
			var parts []any
			for _, part := range []string{input[:split], input[split:]} {
				parts = append(parts, map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": part})
			}
			var got strings.Builder
			for _, event := range restoreTestEvents(t, ctx, parts...) {
				got.WriteString(event["delta"].(string))
			}
			if got.String() != input || ctx.RestoreCount() != 0 {
				t.Fatalf("split %d changed an identifier: %q", split, got.String())
			}
		}
	}
}

func TestChangedPlaceholderIDIsNotRecovered(t *testing.T) {
	ctx := NewContext(10)
	token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	last := byte('A')
	if token[len(token)-3] == last {
		last = 'B'
	}
	unknown := token[:len(token)-3] + string(last) + "}}"
	if got := ctx.RestoreText(unknown); got != unknown || ctx.RestoreCount() != 0 || ctx.UnresolvedCount() != 1 {
		t.Fatalf("changed identifier was guessed: %q", got)
	}
}

func TestPlaceholderFormsInsideToolJSON(t *testing.T) {
	for name, form := range placeholderForms() {
		t.Run(name, func(t *testing.T) {
			ctx := NewContext(10)
			token, err := ctx.tokenFor("EMAIL", "fake \"quoted\" value\\path\nnext line")
			if err != nil {
				t.Fatal(err)
			}
			args, _ := json.Marshal(map[string]string{"value": form(token)})
			var events []any
			for _, part := range []string{string(args[:len(args)/2]), string(args[len(args)/2:])} {
				events = append(events, map[string]any{"type": "response.function_call_arguments.delta", "output_index": 0, "delta": part})
			}
			var restored strings.Builder
			for _, event := range restoreTestEvents(t, ctx, events...) {
				restored.WriteString(event["delta"].(string))
			}
			var decoded map[string]string
			if err := json.Unmarshal([]byte(restored.String()), &decoded); err != nil || decoded["value"] != "fake \"quoted\" value\\path\nnext line" {
				t.Fatalf("tool value corrupted: %q (%v)", restored.String(), err)
			}
		})
	}
}
