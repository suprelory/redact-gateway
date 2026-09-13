package config

import (
	"testing"
	"time"
)

func clearSessionCacheEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{"ENABLED", "TTL_SECONDS", "MAX_SESSIONS", "MAX_ENTRIES", "MAX_BYTES"} {
		t.Setenv("REDACT_SESSION_CACHE_"+name, "")
	}
}

func TestSessionCacheConfigDefaultsAndOverrides(t *testing.T) {
	clearSessionCacheEnv(t)
	var cfg Config
	if err := loadSessionCacheConfig(&cfg); err != nil || cfg.SessionCacheEnabled || cfg.SessionCacheTTL != 30*time.Minute || cfg.SessionCacheMaxSessions != 128 || cfg.SessionCacheMaxEntries != 16384 || cfg.SessionCacheMaxBytes != 16*1024*1024 {
		t.Fatalf("defaults=%+v err=%v", cfg, err)
	}
	t.Setenv("REDACT_SESSION_CACHE_ENABLED", "1")
	t.Setenv("REDACT_SESSION_CACHE_TTL_SECONDS", "60")
	t.Setenv("REDACT_SESSION_CACHE_MAX_SESSIONS", "4")
	t.Setenv("REDACT_SESSION_CACHE_MAX_ENTRIES", "100")
	t.Setenv("REDACT_SESSION_CACHE_MAX_BYTES", "8192")
	if err := loadSessionCacheConfig(&cfg); err != nil || !cfg.SessionCacheEnabled || cfg.SessionCacheTTL != time.Minute || cfg.SessionCacheMaxSessions != 4 || cfg.SessionCacheMaxEntries != 100 || cfg.SessionCacheMaxBytes != 8192 {
		t.Fatalf("overrides=%+v err=%v", cfg, err)
	}
}

func TestSessionCacheConfigRejectsInvalidLimits(t *testing.T) {
	for _, test := range []struct{ name, value string }{
		{"TTL_SECONDS", "0"}, {"TTL_SECONDS", "9223372037"}, {"MAX_SESSIONS", "-1"},
		{"MAX_ENTRIES", "many"}, {"MAX_BYTES", "1023"}, {"MAX_BYTES", "0"},
	} {
		t.Run(test.name+test.value, func(t *testing.T) {
			clearSessionCacheEnv(t)
			t.Setenv("REDACT_SESSION_CACHE_"+test.name, test.value)
			if err := loadSessionCacheConfig(&Config{}); err == nil {
				t.Fatal("invalid limit was silently accepted")
			}
		})
	}
}
