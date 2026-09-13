package route

import (
	"fmt"
	"net/url"
	"strings"
)

const AllFlagLetters = "HPSIBEG"

type Flags struct {
	Raw         string `json:"raw"`
	HighEntropy bool   `json:"high_entropy"`
	Phone       bool   `json:"phone"`
	Secret      bool   `json:"secret"`
	Identity    bool   `json:"identity"`
	Bank        bool   `json:"bank"`
	Email       bool   `json:"email"`
	Gitleaks    bool   `json:"gitleaks"`
}

type ProxyRoute struct {
	Flags    Flags
	Upstream *url.URL
}

func ParseRequestURI(requestURI string) (ProxyRoute, error) {
	if strings.HasPrefix(requestURI, "http://") || strings.HasPrefix(requestURI, "https://") {
		absolute, err := url.Parse(requestURI)
		if err != nil {
			return ProxyRoute{}, fmt.Errorf("invalid absolute request URI: %w", err)
		}
		requestURI = absolute.RequestURI()
	}
	raw := strings.TrimPrefix(requestURI, "/")
	separator := strings.IndexByte(raw, '$')
	if separator < 0 {
		return ProxyRoute{}, fmt.Errorf("expected /<flags>$<upstream-url>")
	}

	flags, err := ParseFlags(raw[:separator])
	if err != nil {
		return ProxyRoute{}, err
	}
	target := raw[separator+1:]
	if target == "" {
		return ProxyRoute{}, fmt.Errorf("upstream URL is required")
	}
	upstream, err := url.Parse(target)
	if err != nil {
		return ProxyRoute{}, fmt.Errorf("invalid upstream URL: %w", err)
	}
	if upstream.Scheme != "http" && upstream.Scheme != "https" {
		return ProxyRoute{}, fmt.Errorf("upstream scheme must be http or https")
	}
	if upstream.Hostname() == "" {
		return ProxyRoute{}, fmt.Errorf("upstream host is required")
	}
	if upstream.User != nil {
		return ProxyRoute{}, fmt.Errorf("upstream URL must not contain user information")
	}
	if upstream.Fragment != "" {
		return ProxyRoute{}, fmt.Errorf("upstream URL must not contain a fragment")
	}
	return ProxyRoute{Flags: flags, Upstream: upstream}, nil
}

func ParseFlags(raw string) (Flags, error) {
	if raw == "" {
		raw = AllFlagLetters
	}
	flags := Flags{Raw: raw}
	seen := map[rune]bool{}
	for _, letter := range raw {
		if seen[letter] {
			continue
		}
		seen[letter] = true
		switch letter {
		case 'H':
			flags.HighEntropy = true
		case 'P':
			flags.Phone = true
		case 'S':
			flags.Secret = true
		case 'I':
			flags.Identity = true
		case 'B':
			flags.Bank = true
		case 'E':
			flags.Email = true
		case 'G':
			flags.Gitleaks = true
		default:
			return Flags{}, fmt.Errorf("unknown detector flag %q", string(letter))
		}
	}
	return flags, nil
}
