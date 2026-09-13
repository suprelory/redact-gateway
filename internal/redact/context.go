package redact

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"regexp"
	"strings"
)

var placeholderPattern = regexp.MustCompile(`\{\{RG_[A-Z][A-Z0-9]{0,31}_[A-Z2-7]{16}\}\}`)

var ErrRedactionLimit = errors.New("redaction limit exceeded")

type Context struct {
	max         int
	rawToToken  map[string]string
	tokenToRaw  map[string]string
	hits        map[string]int
	restoreHits int
}

func NewContext(max int) *Context {
	return &Context{
		max:        max,
		rawToToken: make(map[string]string),
		tokenToRaw: make(map[string]string),
		hits:       make(map[string]int),
	}
}

func (c *Context) RedactText(text string, flags DetectorFlags) (string, error) {
	matches := FindSensitiveMatches(text, flags)
	if len(matches) == 0 {
		return text, nil
	}

	var out strings.Builder
	out.Grow(len(text) + len(matches)*20)
	position := 0
	for _, match := range matches {
		out.WriteString(text[position:match.Start])
		raw := text[match.Start:match.End]
		token, err := c.tokenFor(match.Label, raw)
		if err != nil {
			return "", err
		}
		out.WriteString(token)
		c.hits[match.Label]++
		position = match.End
	}
	out.WriteString(text[position:])
	return out.String(), nil
}

func (c *Context) RestoreText(text string) string {
	return placeholderPattern.ReplaceAllStringFunc(text, func(token string) string {
		if raw, ok := c.tokenToRaw[token]; ok {
			c.restoreHits++
			return raw
		}
		return token
	})
}

func (c *Context) Hits() map[string]int {
	out := make(map[string]int, len(c.hits))
	for key, value := range c.hits {
		out[key] = value
	}
	return out
}

func (c *Context) RedactionCount() int {
	total := 0
	for _, count := range c.hits {
		total += count
	}
	return total
}

func (c *Context) RestoreCount() int {
	return c.restoreHits
}

func (c *Context) tokenFor(label, raw string) (string, error) {
	if token, ok := c.rawToToken[raw]; ok {
		return token, nil
	}
	if len(c.rawToToken) >= c.max {
		return "", ErrRedactionLimit
	}
	for {
		bytes := make([]byte, 10)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		id := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
		token := "{{RG_" + sanitizeLabel(label) + "_" + id + "}}"
		if existing, exists := c.tokenToRaw[token]; exists && existing != raw {
			continue
		}
		c.rawToToken[raw] = token
		c.tokenToRaw[token] = raw
		return token, nil
	}
}

func sanitizeLabel(label string) string {
	label = strings.ToUpper(label)
	var out strings.Builder
	for _, r := range label {
		if (r >= 'A' && r <= 'Z') || (out.Len() > 0 && r >= '0' && r <= '9') {
			out.WriteRune(r)
		}
		if out.Len() == 32 {
			break
		}
	}
	if out.Len() == 0 {
		return "SECRET"
	}
	return out.String()
}
