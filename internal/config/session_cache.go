package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func loadSessionCacheConfig(cfg *Config) error {
	cfg.SessionCacheEnabled = envBool("REDACT_SESSION_CACHE_ENABLED", false)
	ttlSeconds := 1800
	settings := []struct {
		name     string
		fallback int
		target   *int
	}{
		{"REDACT_SESSION_CACHE_MAX_SESSIONS", 128, &cfg.SessionCacheMaxSessions},
		{"REDACT_SESSION_CACHE_MAX_ENTRIES", 16384, &cfg.SessionCacheMaxEntries},
		{"REDACT_SESSION_CACHE_MAX_BYTES", 16 * 1024 * 1024, &cfg.SessionCacheMaxBytes},
		{"REDACT_SESSION_CACHE_TTL_SECONDS", 1800, &ttlSeconds},
	}
	for _, setting := range settings {
		value := setting.fallback
		if raw := strings.TrimSpace(os.Getenv(setting.name)); raw != "" {
			var err error
			value, err = strconv.Atoi(raw)
			if err != nil || value <= 0 {
				return fmt.Errorf("%s must be a positive integer", setting.name)
			}
		}
		*setting.target = value
	}
	if cfg.SessionCacheMaxBytes < 1024 {
		return fmt.Errorf("REDACT_SESSION_CACHE_MAX_BYTES must be at least 1024")
	}
	if int64(ttlSeconds) > int64((1<<63-1)/time.Second) {
		return fmt.Errorf("REDACT_SESSION_CACHE_TTL_SECONDS is too large")
	}
	cfg.SessionCacheTTL = time.Duration(ttlSeconds) * time.Second
	return nil
}
