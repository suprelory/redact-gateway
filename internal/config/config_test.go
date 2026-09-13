package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadOrCreateAdminTokenMinimumLength(t *testing.T) {
	dataDir := t.TempDir()
	if _, err := loadOrCreateAdminToken(dataDir, "12345678901"); err == nil {
		t.Fatal("expected an 11-character token to be rejected")
	}
	token, err := loadOrCreateAdminToken(dataDir, "123456789012")
	if err != nil {
		t.Fatalf("12-character token rejected: %v", err)
	}
	if token != "123456789012" {
		t.Fatalf("token = %q, want configured token", token)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "admin-token"), []byte("123456789012\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadOrCreateAdminToken(dataDir, "")
	if err != nil {
		t.Fatalf("12-character persisted token rejected: %v", err)
	}
	if loaded != "123456789012" {
		t.Fatalf("loaded token = %q, want persisted token", loaded)
	}
}

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
