package gateway

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/suprelory/redact-gateway/internal/redact"
)

const sessionHeader = "X-Redact-Session"

func (p *Proxy) requestContext(request *http.Request, upstream *url.URL) (*redact.Context, error) {
	if p.sessionCache == nil {
		return redact.NewContext(p.cfg.MaxRedactions), nil
	}
	values := request.Header.Values(sessionHeader)
	if len(values) == 0 {
		return redact.NewContext(p.cfg.MaxRedactions), nil
	}
	if len(values) != 1 || !validSessionID(values[0]) {
		return nil, fmt.Errorf("X-Redact-Session must be a single opaque identifier of 16 to 256 ASCII letters, digits, hyphens or underscores")
	}
	forwarded := make(http.Header)
	copyRequestHeaders(forwarded, request.Header)
	key, credentialed := sessionScope(values[0], forwarded, upstream)
	if !credentialed {
		return redact.NewContext(p.cfg.MaxRedactions), nil
	}
	return p.sessionCache.Context(key, p.cfg.MaxRedactions), nil
}

func validSessionID(value string) bool {
	if len(value) < 16 || len(value) > 256 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			return false
		}
	}
	return true
}

// Only a digest of routing and identity material is retained. Length framing
// prevents two different header/value combinations from producing the same
// input to the digest. IP addresses and Host alone never identify a session.
func sessionScope(session string, headers http.Header, upstream *url.URL) ([32]byte, bool) {
	digest := sha256.New()
	add := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		digest.Write(length[:])
		digest.Write([]byte(value))
	}
	add(session)
	add(strings.ToLower(upstream.Scheme))
	add(strings.ToLower(upstream.Host))
	add(upstream.EscapedPath())
	add(upstream.RawQuery)
	credentialed := false
	for _, name := range []string{"Authorization", "X-Api-Key", "Api-Key", "OpenAI-Organization", "OpenAI-Project", "Anthropic-Organization-Id"} {
		add(name)
		values := headers.Values(name)
		add(fmt.Sprint(len(values)))
		for _, value := range values {
			add(value)
			if strings.TrimSpace(value) != "" && (name == "Authorization" || name == "X-Api-Key" || name == "Api-Key") {
				credentialed = true
			}
		}
	}
	var key [32]byte
	copy(key[:], digest.Sum(nil))
	return key, credentialed
}

func (p *Proxy) Close() { p.sessionCache.Close() }
