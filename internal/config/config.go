package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
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

	var configuredHosts []string
	for _, host := range strings.Split(os.Getenv("REDACT_ALLOWED_HOSTS"), ",") {
		if strings.TrimSpace(host) != "" {
			configuredHosts = append(configuredHosts, host)
		}
	}
	var err error
	cfg.AllowedHosts, err = NormalizeAllowedHosts(configuredHosts)
	if err != nil {
		return Config{}, fmt.Errorf("REDACT_ALLOWED_HOSTS: %w", err)
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

// NormalizeAllowedHosts validates and canonicalizes host names used by the
// upstream allowlist. Empty input is valid and means all public hosts are
// allowed, matching the gateway's existing behavior.
func NormalizeAllowedHosts(hosts []string) ([]string, error) {
	normalized := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for index, raw := range hosts {
		host := strings.ToLower(strings.TrimSpace(raw))
		host = strings.TrimSuffix(host, ".")
		if host == "" {
			return nil, fmt.Errorf("entry %d is empty", index+1)
		}
		if err := validateAllowedHost(host); err != nil {
			return nil, fmt.Errorf("entry %d (%q): %w", index+1, raw, err)
		}
		if _, exists := seen[host]; exists {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	return normalized, nil
}

func validateAllowedHost(host string) error {
	if len(host) > 253 {
		return fmt.Errorf("host name is too long")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if strings.ContainsAny(host, "/?#@:*") {
		return fmt.Errorf("must be a host name without scheme, port, path, or wildcard")
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return fmt.Errorf("contains an invalid DNS label")
		}
		for _, character := range label {
			if !(unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-') {
				return fmt.Errorf("contains an invalid DNS character")
			}
		}
	}
	return nil
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
