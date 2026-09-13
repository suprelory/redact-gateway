package redact

import (
	"regexp"
	"strings"
)

var (
	placeholderPattern     = regexp.MustCompile(`\{\{RG_[A-Z][A-Z0-9]{0,31}_[A-Z2-7]{16}\}\}`)
	placeholderCorePattern = regexp.MustCompile(`(?i:RG_)([A-Za-z][A-Za-z0-9_]{0,63})_([A-Za-z2-7]{16})`)
	// A bare identifier or one closing brace waits for a boundary or stream end.
	placeholderPrefixPattern = regexp.MustCompile(`(?i)^(?:\\{0,3}\{){0,2}(?:R(?:G(?:_[A-Z0-9_]{0,81})?)?)?(?:\\{0,3}\})?\\{0,3}$`)
)

const maxPlaceholderSpan = 128

type placeholderCandidate struct {
	start, end int
	token      string
}

func placeholderCandidates(text string) []placeholderCandidate {
	indices := placeholderCorePattern.FindAllStringSubmatchIndex(text, -1)
	var candidates []placeholderCandidate
	for _, match := range indices {
		coreStart, coreEnd := match[0], match[1]
		end, _, rightEscaped := closingBraces(text, coreEnd)
		start := openingBraces(text, coreStart, rightEscaped)
		// A bare core embedded in another identifier is not a placeholder.
		if start == coreStart && start > 0 && identifierByte(text[start-1]) {
			continue
		}
		if end == coreEnd && end < len(text) && identifierByte(text[end]) {
			continue
		}
		label := strings.ReplaceAll(strings.ToUpper(text[match[2]:match[3]]), "_", "")
		id := strings.ToUpper(text[match[4]:match[5]])
		if len(label) > 32 {
			// Never truncate a changed label into a different, issued token.
			candidates = append(candidates, placeholderCandidate{start: start, end: end})
			continue
		}
		candidates = append(candidates, placeholderCandidate{start: start, end: end, token: "{{RG_" + label + "_" + id + "}}"})
	}
	return candidates
}

func openingBraces(text string, start int, rightEscaped bool) int {
	if start >= 2 && text[start-2:start] == "{{" {
		// In C:\{{RG_...}}, the backslash is a path separator, not an escape.
		return start - 2
	}
	position, count := start, 0
	for count < 2 && position > 0 && text[position-1] == '{' {
		position--
		for slashes := 0; slashes < 3 && position > 0 && text[position-1] == '\\'; slashes++ {
			position--
		}
		count++
	}
	if count == 1 && !rightEscaped {
		return start - 1
	}
	return position
}

func closingBraces(text string, start int) (int, int, bool) {
	position, count, escaped := start, 0, false
	for count < 2 {
		next := position
		for slashes := 0; slashes < 3 && next < len(text) && text[next] == '\\'; slashes++ {
			next++
		}
		if next >= len(text) || text[next] != '}' {
			break
		}
		escaped = escaped || next > position
		position = next + 1
		count++
	}
	return position, count, escaped
}

func identifierByte(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || value == '_'
}

func possiblePlaceholderSuffixLength(text string) int {
	start := len(text) - maxPlaceholderSpan
	if start < 0 {
		start = 0
	}
	// Keep the earliest possible start, including escaped opening braces.
	for index := start; index < len(text); index++ {
		switch text[index] {
		case 'R', 'r':
			if index > 0 && identifierByte(text[index-1]) {
				continue
			}
		case '{', '\\':
		default:
			continue
		}
		if isPlaceholderPrefix(text[index:]) {
			return len(text) - index
		}
	}
	return 0
}

func isPlaceholderPrefix(value string) bool {
	if value == "" || len(value) > maxPlaceholderSpan {
		return false
	}
	if !strings.ContainsAny(value, "{Rr") && strings.Trim(value, `\`) != "" {
		return false
	}
	return placeholderPrefixPattern.MatchString(value)
}
