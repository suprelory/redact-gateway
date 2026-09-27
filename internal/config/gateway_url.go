package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// NormalizeGatewayURL validates the public base URL used by the console and
// client examples. An empty value lets the console infer the address.
func NormalizeGatewayURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.ContainsAny(raw, "\\?#") || strings.ContainsFunc(raw, unicode.IsSpace) {
		return "", fmt.Errorf("gateway URL must not contain whitespace, backslashes, a query, or a fragment")
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("gateway URL must be a complete HTTP or HTTPS URL")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("gateway URL must not contain credentials")
	}
	if err := validateAllowedHost(strings.TrimSuffix(parsed.Hostname(), ".")); err != nil {
		return "", fmt.Errorf("invalid gateway host: %w", err)
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return "", fmt.Errorf("gateway URL port must be between 1 and 65535")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", fmt.Errorf("gateway URL port must be between 1 and 65535")
		}
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
