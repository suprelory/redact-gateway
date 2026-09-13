package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultMaxBody       = 16 * 1024 * 1024
	defaultMaxRedactions = 16_384
)

type Config struct {
	ListenAddr        string
	AdminAddr         string
	DataDir           string
	AdminToken        string
	AllowedHosts      []string
	AllowPrivateHosts bool
	MaxBodyBytes      int64
	MaxRedactions     int
	LogRetentionDays  int
	CORSOrigin        string
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddr:        envString("REDACT_LISTEN_ADDR", "127.0.0.1:8787"),
		AdminAddr:         envString("REDACT_ADMIN_ADDR", "127.0.0.1:8788"),
		DataDir:           envString("REDACT_DATA_DIR", defaultDataDir()),
		AllowPrivateHosts: envBool("REDACT_ALLOW_PRIVATE_UPSTREAMS", false),
		MaxBodyBytes:      int64(envInt("REDACT_MAX_BODY_BYTES", defaultMaxBody)),
		MaxRedactions:     envInt("REDACT_MAX_REDACTIONS", defaultMaxRedactions),
		LogRetentionDays:  envInt("REDACT_LOG_RETENTION_DAYS", 30),
		CORSOrigin:        envString("REDACT_CORS_ORIGIN", "*"),
	}

	for _, host := range strings.Split(os.Getenv("REDACT_ALLOWED_HOSTS"), ",") {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" {
			cfg.AllowedHosts = append(cfg.AllowedHosts, host)
		}
	}

	if cfg.MaxBodyBytes <= 0 || cfg.MaxRedactions <= 0 {
		return Config{}, fmt.Errorf("body and redaction limits must be positive")
	}
	if cfg.LogRetentionDays < 0 {
		return Config{}, fmt.Errorf("REDACT_LOG_RETENTION_DAYS cannot be negative")
	}
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return Config{}, fmt.Errorf("create data directory: %w", err)
	}

	token, err := loadOrCreateAdminToken(cfg.DataDir, strings.TrimSpace(os.Getenv("REDACT_ADMIN_TOKEN")))
	if err != nil {
		return Config{}, err
	}
	cfg.AdminToken = token
	return cfg, nil
}

func defaultDataDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "redact-gateway")
	}
	return ".redact-gateway"
}

func loadOrCreateAdminToken(dataDir, configured string) (string, error) {
	if configured != "" {
		if len(configured) < 16 {
			return "", fmt.Errorf("REDACT_ADMIN_TOKEN must contain at least 16 characters")
		}
		return configured, nil
	}

	path := filepath.Join(dataDir, "admin-token")
	if data, err := os.ReadFile(path); err == nil {
		if token := strings.TrimSpace(string(data)); len(token) >= 16 {
			return token, nil
		}
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate admin token: %w", err)
	}
	token := "rg_admin_" + base64.RawURLEncoding.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write admin token: %w", err)
	}
	return token, nil
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
