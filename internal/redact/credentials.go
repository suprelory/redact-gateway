package redact

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Signatures are derived from Cosy's portable Gitleaks rules and compiled by
// Go's RE2 engine. Capture offsets are taken directly from submatch indices.
// Source versions and licenses are recorded in data/ and third_party/.
//
//go:embed data/gitleaks-rules.json
var credentialRulesJSON []byte

type credentialRule struct {
	id           string
	pattern      *regexp.Regexp
	secretGroup  int
	entropy      float64
	keywords     []string
	allowRegexes []*regexp.Regexp
	stopwords    []string
}

var credentialRules = loadCredentialRules()

func loadCredentialRules() []credentialRule {
	var data struct {
		Rules []struct {
			ID           string   `json:"id"`
			Pattern      string   `json:"pattern"`
			SecretGroup  int      `json:"secret_group"`
			Entropy      float64  `json:"entropy"`
			Keywords     []string `json:"keywords"`
			AllowRegexes []string `json:"allow_regexes"`
			Stopwords    []string `json:"stopwords"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(credentialRulesJSON, &data); err != nil {
		panic(err)
	}
	compiled := make([]credentialRule, 0, len(data.Rules))
	ids := make(map[string]bool)
	for _, rule := range data.Rules {
		pattern, err := regexp.Compile(rule.Pattern)
		if err != nil || rule.ID == "" || rule.SecretGroup < 0 || rule.SecretGroup > pattern.NumSubexp() || ids[rule.ID] {
			panic(fmt.Sprintf("invalid credential rule %s: %v", rule.ID, err))
		}
		ids[rule.ID] = true
		entry := credentialRule{id: rule.ID, pattern: pattern, secretGroup: rule.SecretGroup,
			entropy: rule.Entropy, keywords: rule.Keywords, stopwords: rule.Stopwords}
		for _, expression := range rule.AllowRegexes {
			entry.allowRegexes = append(entry.allowRegexes, regexp.MustCompile(expression))
		}
		compiled = append(compiled, entry)
	}
	return compiled
}

func collectCredentials(text string) []Match {
	lower := strings.ToLower(text)
	var matches []Match
	for _, rule := range credentialRules {
		if len(rule.keywords) > 0 && !containsKeyword(lower, rule.keywords) {
			continue
		}
		for _, indexes := range rule.pattern.FindAllStringSubmatchIndex(text, -1) {
			start, end := indexes[rule.secretGroup*2], indexes[rule.secretGroup*2+1]
			if start < 0 || start == end {
				continue
			}
			secret := text[start:end]
			if rule.entropy > 0 && shannonEntropy(secret) < rule.entropy {
				continue
			}
			allowed := false
			for _, stopword := range rule.stopwords {
				// A real credential containing "example" is still sensitive.
				allowed = allowed || strings.EqualFold(secret, stopword)
			}
			for _, expression := range rule.allowRegexes {
				allowed = allowed || expression.MatchString(secret)
			}
			if !allowed {
				matches = append(matches, Match{Start: start, End: end, Label: credentialLabel(rule.id), Priority: 120})
			}
		}
	}
	return matches
}

func containsKeyword(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func credentialLabel(id string) string {
	switch {
	case strings.HasPrefix(id, "github-"):
		return "GITHUB"
	case strings.HasPrefix(id, "gitlab-"):
		return "GITLAB"
	case strings.HasPrefix(id, "aws-"):
		return "AWSKEY"
	case strings.HasPrefix(id, "jwt"):
		return "JWT"
	case id == "private-key":
		return "PRIVATEKEY"
	default:
		return "CREDENTIAL"
	}
}

func isCredentialField(key string) bool {
	normalized := strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == '.' || r == ' ' {
			return -1
		}
		return r
	}, strings.ToLower(key))
	switch normalized {
	case "apikey", "apitoken", "accesskey", "accesskeyid", "accesskeysecret", "secretaccesskey",
		"accesstoken", "refreshtoken", "clientsecret", "password", "passwd", "secret", "token",
		"authorization", "privatekey", "credential", "credentials":
		return true
	}
	return false
}
