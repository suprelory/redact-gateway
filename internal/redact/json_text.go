package redact

import (
	"encoding/json"
	"strings"
)

func isJSONTextField(key string) bool {
	return key == "arguments" || key == "partial_json"
}

func restoreString(text string, context *Context, jsonText bool) string {
	if !jsonText {
		return context.RestoreText(text)
	}
	if out, valid, _ := rewriteJSONText(text, func(value string) (string, error) {
		return context.RestoreText(value), nil
	}); valid {
		return out
	}
	// Incremental arguments need not contain a whole JSON document. Mappings
	// hold decoded values, so inserting one inside a JSON string needs escaping.
	return context.restoreText(text, true)
}

func escapeJSONString(text string) string {
	encoded, _ := json.Marshal(text)
	return string(encoded[1 : len(encoded)-1])
}

// rewriteJSONText transforms string values inside a serialized JSON document.
// Keep keys, whitespace, number precision and untouched escape sequences intact.
// Decoding each value before masking makes mappings independent of whether a
// secret arrived in ordinary text or in a stringified tool argument.
func rewriteJSONText(text string, transform func(string) (string, error)) (string, bool, error) {
	if !json.Valid([]byte(text)) {
		return text, false, nil
	}
	var out strings.Builder
	last := 0
	for index := 0; index < len(text); index++ {
		if text[index] != '"' {
			continue
		}
		start := index
		index++
		for text[index] != '"' {
			if text[index] == '\\' {
				index++
			}
			index++
		}
		end := index + 1
		next := end
		for next < len(text) && strings.ContainsRune(" \t\r\n", rune(text[next])) {
			next++
		}
		if next < len(text) && text[next] == ':' {
			continue // Object keys follow the same policy as ordinary JSON.
		}
		var value string
		if err := json.Unmarshal([]byte(text[start:end]), &value); err != nil {
			return "", true, err
		}
		mapped, err := transform(value)
		if err != nil {
			return "", true, err
		}
		if mapped != value {
			out.WriteString(text[last:start])
			out.WriteByte('"')
			out.WriteString(escapeJSONString(mapped))
			out.WriteByte('"')
			last = end
		}
	}
	if last == 0 {
		return text, true, nil
	}
	out.WriteString(text[last:])
	return out.String(), true, nil
}
