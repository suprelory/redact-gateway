package redact

import (
	"strconv"
	"strings"
)

// jsonFragmentUnit removes one JSON string-encoding layer for token matching.
// Only ASCII matters to the placeholder grammar; other escaped code points are
// represented by a boundary byte. A zero width means an unfinished escape.
func jsonFragmentUnit(text string, at int) (byte, int) {
	if text[at] != '\\' {
		return text[at], 1
	}
	if at+1 == len(text) {
		return 0, 0
	}
	switch text[at+1] {
	case '"', '\\', '/':
		return text[at+1], 2
	case 'b', 'f', 'n', 'r', 't':
		return ' ', 2 // A non-identifier boundary, never part of a token.
	case 'u':
		end := at + 6
		if end > len(text) {
			end = len(text)
		}
		for _, digit := range text[at+2 : end] {
			if !(digit >= '0' && digit <= '9' || digit >= 'a' && digit <= 'f' || digit >= 'A' && digit <= 'F') {
				return '\\', 1
			}
		}
		if end < at+6 {
			return 0, 0
		}
		code, _ := strconv.ParseUint(text[at+2:end], 16, 16)
		if code < 128 {
			return byte(code), 6
		}
		return 0xff, 6
	default:
		// Preserve malformed escapes for compatible upstreams; do not consume
		// the following character or accidentally decode a second layer.
		return '\\', 1
	}
}

func jsonFragmentView(text string) (string, int) {
	if !strings.Contains(text, `\`) {
		return text, len(text)
	}
	var view strings.Builder
	view.Grow(len(text))
	for at := 0; at < len(text); {
		value, width := jsonFragmentUnit(text, at)
		if width == 0 {
			return view.String(), at
		}
		view.WriteByte(value)
		at += width
	}
	return view.String(), len(text)
}

func jsonPlaceholderSuffixLength(text string) int {
	view, end := jsonFragmentView(text)
	hold := possiblePlaceholderSuffixLength(view)
	if hold == 0 {
		return len(text) - end
	}
	raw := 0
	for index := 0; index < len(view)-hold; index++ {
		_, width := jsonFragmentUnit(text, raw)
		raw += width
	}
	return len(text) - raw
}

func jsonPlaceholderCandidates(text string) []placeholderCandidate {
	view, _ := jsonFragmentView(text)
	candidates := placeholderCandidates(view)
	// Translate only candidate boundaries back to the original serialized text.
	// This avoids an offset array proportional to the entire argument document.
	raw, logical := 0, 0
	for index := range candidates {
		candidate := &candidates[index]
		start, end := candidate.start, candidate.end
		for logical < start {
			_, width := jsonFragmentUnit(text, raw)
			raw += width
			logical++
		}
		candidate.start = raw
		for logical < end {
			_, width := jsonFragmentUnit(text, raw)
			raw += width
			logical++
		}
		candidate.end = raw
	}
	return candidates
}
