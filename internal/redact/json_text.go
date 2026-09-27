package redact

import (
	"encoding/json"
	"strings"
)

func isJSONTextField(key string) bool {
	return key == "arguments" || key == "partial_json"
}

func isJSONContainerText(text string) bool {
	text = strings.TrimSpace(text)
	return len(text) > 0 && (text[0] == '{' || text[0] == '[')
}

func restoreString(text string, context *Context, jsonText bool) string {
	return restoreStringDepth(text, context, jsonText, 0, 0)
}

func restoreStringDepth(text string, context *Context, jsonText bool, depth, encodedDepth int) string {
	if isJSONContainerText(text) || jsonText {
		if encodedDepth >= maxEncodedJSONDepth {
			return text
		}
		out, valid, err := rewriteJSONText(text, depth, jsonTextTransform{
			stringValue: func(value, key string, nestedDepth int) (string, error) {
				return restoreStringDepth(value, context, isJSONTextField(key), nestedDepth, encodedDepth+1), nil
			},
		})
		if err != nil {
			return text
		}
		if valid {
			return out
		}
	}
	// Incremental arguments need not contain a whole JSON document. Mappings
	// hold decoded values, so inserting one inside a JSON string needs escaping.
	return context.restoreText(text, jsonText)
}

func escapeJSONString(text string) string {
	encoded, _ := json.Marshal(text)
	return string(encoded[1 : len(encoded)-1])
}

type jsonTextTransform struct {
	stringValue func(value, key string, depth int) (string, error)
	numberValue func(json.Number) error
}

// Rewrite only changed string value tokens. Object keys, whitespace, number
// lexemes and untouched escapes retain their original bytes. The validated
// token walk provides field context without reserializing the document.
func rewriteJSONText(text string, depth int, transform jsonTextTransform) (string, bool, error) {
	if err := checkJSONTextDepth(text, depth); err != nil {
		return "", true, err
	}
	if !json.Valid([]byte(text)) {
		return text, false, nil
	}
	walker := jsonTextWalker{text: text, transform: transform}
	if err := walker.value("", depth); err != nil {
		return "", true, err
	}
	if walker.last == 0 {
		return text, true, nil
	}
	walker.out.WriteString(text[walker.last:])
	return walker.out.String(), true, nil
}

// Check before json.Valid, whose own, much larger nesting limit otherwise
// makes an over-deep encoded document look like ordinary, unparsed text.
func checkJSONTextDepth(text string, depth int) error {
	quoted := false
	for index := 0; index < len(text); index++ {
		if quoted {
			if text[index] == '\\' {
				index++
			} else if text[index] == '"' {
				quoted = false
			}
			continue
		}
		switch text[index] {
		case '"':
			quoted = true
		case '{', '[':
			depth++
			if depth > maxJSONDepth {
				return ErrJSONDepthLimit
			}
		case '}', ']':
			depth--
		}
	}
	return nil
}

type jsonTextWalker struct {
	text      string
	position  int
	last      int
	out       strings.Builder
	transform jsonTextTransform
}

func (w *jsonTextWalker) space() {
	for w.position < len(w.text) && strings.ContainsRune(" \t\r\n", rune(w.text[w.position])) {
		w.position++
	}
}

func (w *jsonTextWalker) quoted() (string, int, int) {
	start := w.position
	w.position++
	for w.text[w.position] != '"' {
		if w.text[w.position] == '\\' {
			w.position++
		}
		w.position++
	}
	w.position++
	var value string
	_ = json.Unmarshal([]byte(w.text[start:w.position]), &value) // Validated above.
	return value, start, w.position
}

func (w *jsonTextWalker) value(key string, depth int) error {
	w.space()
	switch w.text[w.position] {
	case '{', '[':
		object := w.text[w.position] == '{'
		closing := byte(']')
		if object {
			closing = '}'
		}
		w.position++
		w.space()
		if w.text[w.position] == closing {
			w.position++
			return nil
		}
		for {
			childKey := key
			if object {
				childKey, _, _ = w.quoted()
				w.space()
				w.position++ // Colon.
			}
			if err := w.value(childKey, depth+1); err != nil {
				return err
			}
			w.space()
			if w.text[w.position] == closing {
				w.position++
				return nil
			}
			w.position++ // Comma.
			w.space()
		}
	case '"':
		value, start, end := w.quoted()
		mapped, err := w.transform.stringValue(value, key, depth)
		if err != nil {
			return err
		}
		if mapped != value {
			w.out.WriteString(w.text[w.last:start])
			w.out.WriteByte('"')
			w.out.WriteString(escapeJSONString(mapped))
			w.out.WriteByte('"')
			w.last = end
		}
	case 't', 'n':
		w.position += 4
	case 'f':
		w.position += 5
	default:
		start := w.position
		for w.position < len(w.text) && !strings.ContainsRune(",]} \t\r\n", rune(w.text[w.position])) {
			w.position++
		}
		if w.transform.numberValue != nil {
			return w.transform.numberValue(json.Number(w.text[start:w.position]))
		}
	}
	return nil
}
