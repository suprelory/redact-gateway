package config

import (
	"reflect"
	"testing"
)

func TestNormalizeAllowedHosts(t *testing.T) {
	hosts, err := NormalizeAllowedHosts([]string{" API.OpenAI.com. ", "api.anthropic.com", "api.openai.com"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api.openai.com", "api.anthropic.com"}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("hosts = %#v, want %#v", hosts, want)
	}
}

func TestNormalizeAllowedHostsRejectsInvalidEntries(t *testing.T) {
	for _, value := range []string{"https://api.openai.com", "api.openai.com:443", "*.example.com", "bad host"} {
		if _, err := NormalizeAllowedHosts([]string{value}); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
