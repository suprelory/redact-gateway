package route

import "testing"

func TestParseRequestURI(t *testing.T) {
	t.Parallel()
	parsed, err := ParseRequestURI("/HPSE$https://api.openai.com/v1/chat/completions?trace=1")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Flags.Raw != "HPSE" || !parsed.Flags.HighEntropy || !parsed.Flags.Phone || !parsed.Flags.Secret || !parsed.Flags.Email {
		t.Fatalf("unexpected flags: %+v", parsed.Flags)
	}
	if got := parsed.Upstream.String(); got != "https://api.openai.com/v1/chat/completions?trace=1" {
		t.Fatalf("unexpected upstream: %s", got)
	}
}

func TestEmptyFlagsEnableAll(t *testing.T) {
	t.Parallel()
	parsed, err := ParseRequestURI("/$https://api.anthropic.com/v1/messages")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Flags.Raw != AllFlagLetters || !parsed.Flags.Gitleaks || !parsed.Flags.Bank {
		t.Fatalf("unexpected all flags: %+v", parsed.Flags)
	}
}

func TestRejectsUnknownFlagAndUserInfo(t *testing.T) {
	t.Parallel()
	if _, err := ParseRequestURI("/Z$https://api.openai.com/v1"); err == nil {
		t.Fatal("expected unknown flag error")
	}
	if _, err := ParseRequestURI("/$https://user:pass@example.com/v1"); err == nil {
		t.Fatal("expected user info error")
	}
}
