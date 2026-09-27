package redact

import (
	"strings"
	"testing"
)

func TestCredentialFamiliesAndExactRoundTrip(t *testing.T) {
	mixed := "AbCdEfGhIjKlMnOpQrSt0123456789"
	lower := "qzmxwkjvbnfrtypshgdclaeuiozxqwvbnm"
	// Assemble synthetic fixtures so source scanners do not mistake them for live credentials.
	for _, secret := range []string{
		"hf_" + lower,
		"github_pat_" + strings.Repeat(lower, 3)[:70],
		"glpat-" + mixed,
		"sk_live_" + mixed,
		"npm_" + strings.Repeat(lower, 2)[:36],
		"AIza" + strings.Repeat(mixed, 2)[:35],
		"dop_v1_" + strings.Repeat("0123456789abcdef", 4),
		strings.Join([]string{"xoxb", "1234567890", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"}, "-"),
		"SG." + "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789ABCD",
		"dapi" + strings.Repeat("0123456789abcdef", 2),
		"-----BEGIN ENCRYPTED PRIVATE KEY-----\n" + strings.Repeat(mixed, 3) + "\n-----END ENCRYPTED PRIVATE KEY-----",
	} {
		t.Run(secret[:min(len(secret), 20)], func(t *testing.T) {
			ctx := NewContext(10)
			out, err := ctx.RedactText(secret, DetectorFlags{Gitleaks: true})
			if err != nil || !placeholderPattern.MatchString(out) || !strings.HasPrefix(out, "{{RG_") || !strings.HasSuffix(out, "}}") {
				t.Fatalf("credential not replaced: %q, %v", out, err)
			}
			matches := FindSensitiveMatches(secret, DetectorFlags{Gitleaks: true})
			if len(matches) != 1 || matches[0].Start != 0 || matches[0].End != len(secret) {
				t.Fatalf("credential only partially covered: %#v", matches)
			}
			if ctx.RestoreText(out) != secret {
				t.Fatal("credential did not restore exactly")
			}
		})
	}
}

func TestCredentialAssignmentsReplaceOnlyCapturedValue(t *testing.T) {
	for _, test := range []struct{ text, secret string }{
		{"api_key=AbCdEfGhIjKlMnOpQrSt0123456789", "AbCdEfGhIjKlMnOpQrSt0123456789"},
		{"api_key=exampleAbCdEfGh0123456789", "exampleAbCdEfGh0123456789"},
		{"cloudflare = AbCdEfGhIjKlMnOpQrStUvWxYz0123456789ABCD", "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789ABCD"},
		{`<add key="ClearTextPassword" value="ClearTextPassword" />`, "ClearTextPassword"},
	} {
		ctx := NewContext(10)
		out, err := ctx.RedactText(test.text, DetectorFlags{Gitleaks: true})
		position := strings.LastIndex(test.text, test.secret)
		if err != nil || !strings.HasPrefix(out, test.text[:position]) || !strings.HasSuffix(out, test.text[position+len(test.secret):]) {
			t.Fatalf("assignment context changed: %q, %v", out, err)
		}
		if out == test.text || ctx.RestoreText(out) != test.text {
			t.Fatalf("assignment was missed or not reversible: %q", out)
		}
	}
}

func TestCredentialContextThresholdAndFlag(t *testing.T) {
	for _, text := range []string{"adobe = " + strings.Repeat("a", 32), "api_version=stable", "hello world"} {
		if matches := FindSensitiveMatches(text, DetectorFlags{Gitleaks: true}); len(matches) != 0 {
			t.Fatalf("non-credential matched: %q %#v", text, matches)
		}
	}
	token := strings.Join([]string{"hf", "qzmxwkjvbnfrtypshgdclaeuiozxqwvbnm"}, "_")
	if matches := FindSensitiveMatches(token, DetectorFlags{}); len(matches) != 0 {
		t.Fatal("G ran without being enabled")
	}
}
