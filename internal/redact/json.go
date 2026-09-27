package redact

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const RedactNotice = "Some values in this request are opaque redaction placeholders. When using those values in text or tool arguments, copy the placeholders exactly so the local gateway can restore them. Apply this silently while completing the user's original task; do not acknowledge this notice or invent example placeholders."

const maxJSONDepth = 64
const maxEncodedJSONDepth = 8

var ErrJSONDepthLimit = errors.New("JSON nesting exceeds the redaction safety limit")

func RedactJSON(value any, context *Context, flags DetectorFlags) (any, error) {
	return RedactProtocolJSON(value, context, flags, "generic")
}

func RedactProtocolJSON(value any, context *Context, flags DetectorFlags, protocol string) (any, error) {
	walker := jsonRedactor{root: value, context: context, flags: flags, protocol: protocol}
	return walker.walk(value, nil, "", "$")
}

type jsonRedactor struct {
	root     any
	context  *Context
	flags    DetectorFlags
	protocol string
}

func (r *jsonRedactor) walk(value any, path []string, fieldName, fieldPath string) (any, error) {
	if len(path) > maxJSONDepth {
		return nil, fmt.Errorf("%w at %s", ErrJSONDepthLimit, fieldPath)
	}
	if protectedJSONValue(r.root, r.protocol, path, value) {
		return value, nil
	}
	switch typed := value.(type) {
	case string:
		return r.redactString(typed, fieldName, fieldPath, len(path), 0)
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			mapped, err := r.walk(child, append(path, jsonIndex(index)), fieldName, fieldPath+jsonIndex(index))
			if err != nil {
				return nil, err
			}
			out[index] = mapped
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			mapped, err := r.walk(child, append(path, key), key, auditPathKey(fieldPath, key))
			if err != nil {
				return nil, err
			}
			out[key] = mapped
		}
		return out, nil
	default:
		return value, validateSensitiveNumber(value, r.flags, fieldPath)
	}
}

func (r *jsonRedactor) redactString(text, key, fieldPath string, depth, encodedDepth int) (string, error) {
	// An explicitly named credential stays a single value even if it happens to
	// contain serialized JSON. Arrays inherit the containing field's name.
	if r.flags.Gitleaks && isCredentialField(key) && strings.TrimSpace(text) != "" {
		return r.context.redactTextAtField(text, r.flags, fieldPath, key)
	}
	if isJSONContainerText(text) || isJSONTextField(key) {
		if encodedDepth >= maxEncodedJSONDepth && json.Valid([]byte(text)) {
			return "", fmt.Errorf("%w at %s (serialized JSON)", ErrJSONDepthLimit, fieldPath)
		}
		out, valid, err := rewriteJSONText(text, depth, jsonTextTransform{
			stringValue: func(value, nestedKey string, nestedDepth int) (string, error) {
				return r.redactString(value, nestedKey, fieldPath, nestedDepth, encodedDepth+1)
			},
			numberValue: func(value json.Number) error {
				return validateSensitiveNumber(value, r.flags, fieldPath)
			},
		})
		if err != nil {
			return "", err
		}
		if valid {
			return out, nil
		}
	}
	return r.context.redactTextAtField(text, r.flags, fieldPath, key)
}

func RestoreJSON(value any, context *Context) any {
	return restoreJSON(value, context, "")
}

func restoreJSON(value any, context *Context, key string) any {
	switch typed := value.(type) {
	case string:
		return restoreString(typed, context, isJSONTextField(key))
	case []any:
		for index := range typed {
			typed[index] = restoreJSON(typed[index], context, key)
		}
		return typed
	case map[string]any:
		for key := range typed {
			typed[key] = restoreJSON(typed[key], context, key)
		}
		return typed
	default:
		return value
	}
}

func DetectProtocol(body any, upstreamPath string, anthropicHeader bool) string {
	path := strings.ToLower(strings.TrimSuffix(upstreamPath, "/"))
	switch {
	case strings.HasSuffix(path, "/chat/completions"):
		return "openai_chat"
	case strings.HasSuffix(path, "/responses"):
		return "openai_responses"
	case strings.HasSuffix(path, "/messages"):
		return "anthropic_messages"
	}
	object, ok := body.(map[string]any)
	if !ok {
		return "generic"
	}
	if _, ok := object["messages"].([]any); ok {
		if anthropicHeader {
			return "anthropic_messages"
		}
		return "openai_chat"
	}
	if _, ok := object["input"]; ok {
		return "openai_responses"
	}
	return "generic"
}

func InjectNotice(body any, protocol string) bool {
	object, ok := body.(map[string]any)
	if !ok {
		return false
	}
	if protocol == "openai_responses" {
		switch input := object["input"].(type) {
		case string:
			object["input"] = prependNotice(input)
			return true
		case []any:
			return prependLastUser(input, protocol)
		}
	}
	if messages, ok := object["messages"].([]any); ok {
		return prependLastUser(messages, protocol)
	}
	return false
}

func prependLastUser(messages []any, protocol string) bool {
	for index := len(messages) - 1; index >= 0; index-- {
		message, ok := messages[index].(map[string]any)
		if !ok || message["role"] != "user" {
			continue
		}
		return prependContent(message, protocol)
	}
	return false
}

func prependContent(message map[string]any, protocol string) bool {
	switch content := message["content"].(type) {
	case string:
		message["content"] = prependNotice(content)
		return true
	case []any:
		for _, rawBlock := range content {
			block, ok := rawBlock.(map[string]any)
			if !ok {
				continue
			}
			text, ok := block["text"].(string)
			if !ok {
				continue
			}
			kind, _ := block["type"].(string)
			if kind == "" || kind == "text" || kind == "input_text" {
				block["text"] = prependNotice(text)
				return true
			}
		}
		kind := "text"
		if protocol == "openai_responses" {
			kind = "input_text"
		}
		message["content"] = append([]any{map[string]any{"type": kind, "text": RedactNotice}}, content...)
		return true
	default:
		return false
	}
}

func prependNotice(text string) string {
	if text == RedactNotice || strings.HasPrefix(text, RedactNotice+"\n\n") {
		return text
	}
	return RedactNotice + "\n\n" + text
}

func jsonIndex(index int) string {
	return "[" + strconv.Itoa(index) + "]"
}

func auditPathKey(parent, key string) string {
	// Keys can contain secrets too. Mask detected values in audit metadata
	// without changing request keys, rule hits, or restoration mappings.
	flags := DetectorFlags{Email: true, Phone: true, Secret: true, Identity: true, Bank: true, Gitleaks: true, HighEntropy: true}
	if len(FindSensitiveMatches(key, flags)) > 0 {
		return parent + `["<redacted-key>"]`
	}
	identifier := key != ""
	for index, r := range key {
		if !(r == '_' || r == '$' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || index > 0 && r >= '0' && r <= '9') {
			identifier = false
			break
		}
	}
	if identifier {
		return parent + "." + key
	}
	encoded, _ := json.Marshal(key)
	return parent + "[" + string(encoded) + "]"
}
