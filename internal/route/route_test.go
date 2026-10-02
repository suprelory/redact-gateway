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

func TestConfiguredFlags(t *testing.T) {
	requested, err := ParseFlags("EPPE")
	if err != nil || requested.Raw != "EPPE" {
		t.Fatalf("URL parsing changed the requested flags: %+v, %v", requested, err)
	}
	for _, test := range []struct{ input, want string }{
		{"", ""}, {"EPPE", "PE"}, {"GEBISPH", AllFlagLetters},
	} {
		got, err := ParseEnabledFlags(test.input)
		if err != nil || got.Raw != test.want {
			t.Fatalf("ParseEnabledFlags(%q) = %+v, %v; want %q", test.input, got, err, test.want)
		}
		if test.input == "" && got != (Flags{}) {
			t.Fatal("an empty configured selection must disable every detector")
		}
	}
	for _, invalid := range []string{"Z", "e", "E P", "E,P"} {
		if _, err := ParseEnabledFlags(invalid); err == nil {
			t.Fatalf("accepted invalid selection %q", invalid)
		}
	}
}

func TestRequestFlagsRespectEnabledRules(t *testing.T) {
	for _, test := range []struct{ requested, enabled, want string }{
		{"", AllFlagLetters, AllFlagLetters}, {"", "PE", "PE"},
		{AllFlagLetters, "", ""}, {"E", "P", ""}, {"HPSE", "PE", "PE"},
	} {
		requested, _ := ParseFlags(test.requested)
		enabled, _ := ParseEnabledFlags(test.enabled)
		want, _ := ParseEnabledFlags(test.want)
		if got := requested.Intersect(enabled); got != want {
			t.Errorf("%q intersect %q = %+v, want %+v", test.requested, test.enabled, got, want)
		}
	}
}
