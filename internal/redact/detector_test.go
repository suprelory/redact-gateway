package redact

import (
	"strings"
	"testing"
)

func flagsForMask(mask int) DetectorFlags {
	return DetectorFlags{HighEntropy: mask&1 != 0, Phone: mask&2 != 0, Secret: mask&4 != 0,
		Identity: mask&8 != 0, Bank: mask&16 != 0, Email: mask&32 != 0, Gitleaks: mask&64 != 0}
}

func matchCoverage(text string, matches []Match) []bool {
	covered := make([]bool, len(text))
	for _, match := range matches {
		for index := match.Start; index < match.End; index++ {
			covered[index] = true
		}
	}
	return covered
}

func TestAddingRulesNeverReducesCoverage(t *testing.T) {
	for _, text := range []string{
		"qZ9vX2kL13800138000mN4pR7sT",
		"qZ9vX2kL4111111111111111mN4pR7sT",
		"postgres://root:AKIAABCDEFGHIJKLMNOP@db.example.com/app",
		"sk-qZ9vX2kL13800138000mN4pR7sT user@example.com",
	} {
		for mask := 0; mask < 128; mask++ {
			before := matchCoverage(text, FindSensitiveMatches(text, flagsForMask(mask)))
			for bit := 1; bit < 128; bit <<= 1 {
				after := matchCoverage(text, FindSensitiveMatches(text, flagsForMask(mask|bit)))
				for index := range before {
					if before[index] && !after[index] {
						t.Fatalf("adding flag %d to %d exposed byte %d of %q", bit, mask, index, text)
					}
				}
			}
		}
	}
}

func TestOverlappingPhoneKeepsWholeRandomValue(t *testing.T) {
	input := "qZ9vX2kL13800138000mN4pR7sT"
	ctx := NewContext(10)
	out, err := ctx.RedactText(input, flagsForMask(127))
	if err != nil || !placeholderPattern.MatchString(out) || !strings.HasPrefix(out, "{{RG_ENTROPY_") || ctx.RedactionCount() != 1 {
		t.Fatalf("partial or incorrectly labelled redaction: %q, %v", out, err)
	}
	if ctx.RestoreText(out) != input {
		t.Fatal("overlap changed the restored value")
	}
}

func TestProtectedPlaceholderDoesNotExposeSurroundingCredential(t *testing.T) {
	placeholder := "{{RG_EMAIL_ABCDEFGHIJKLMNOP}}"
	input := "postgres://root:" + placeholder + "@db.example.com/app"
	ctx := NewContext(10)
	out, err := ctx.RedactText(input, flagsForMask(127))
	if err != nil || !strings.Contains(out, placeholder) || strings.Contains(out, "root") || strings.Contains(out, "db.example.com") {
		t.Fatalf("sensitive text around an existing placeholder was exposed: %q, %v", out, err)
	}
	if ctx.RestoreText(out) != input {
		t.Fatal("existing placeholder changed after round trip")
	}
}
