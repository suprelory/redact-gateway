package redact

import (
	"encoding/json"
	"strconv"
	"strings"
)

const RedactNotice = "Some values in this request are opaque redaction placeholders. When using those values in text or tool arguments, copy the placeholders exactly so the local gateway can restore them. Apply this silently while completing the user's original task; do not acknowledge this notice or invent example placeholders."

var controlKeys = map[string]struct{}{
	"model": {}, "role": {}, "type": {}, "id": {}, "object": {}, "status": {},
	"name": {}, "call_id": {}, "tool_call_id": {}, "finish_reason": {}, "stop_reason": {},
	"media_type": {}, "mime_type": {}, "encoding": {}, "format": {},
}

func RedactJSON(value any, context *Context, flags DetectorFlags) (any, error) {
	return RedactProtocolJSON(value, context, flags, "generic")
}

func RedactProtocolJSON(value any, context *Context, flags DetectorFlags, protocol string) (any, error) {
	return transformJSON(value, context, flags, nil, "$", protocol)
}

func transformJSON(value any, context *Context, flags DetectorFlags, path []string, fieldPath, protocol string) (any, error) {
	switch typed := value.(type) {
	case string:
		if shouldSkipPath(path) || protocol == "openai_responses" && isResponsesOpaquePath(path) {
			return typed, nil
		}
		if len(path) > 0 && isJSONTextField(path[len(path)-1]) {
			if out, valid, err := rewriteJSONText(typed, func(text string) (string, error) {
				return context.RedactTextAtPath(text, flags, fieldPath)
			}); valid || err != nil {
				return out, err
			}
		}
		return context.RedactTextAtPath(typed, flags, fieldPath)
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			mapped, err := transformJSON(child, context, flags, append(path, jsonIndex(index)), fieldPath+jsonIndex(index), protocol)
			if err != nil {
				return nil, err
			}
			out[index] = mapped
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			mapped, err := transformJSON(child, context, flags, append(path, key), auditPathKey(fieldPath, key), protocol)
			if err != nil {
				return nil, err
			}
			out[key] = mapped
		}
		return out, nil
	default:
		return value, nil
	}
}

func isResponsesOpaquePath(path []string) bool {
	if len(path) == 1 && path[0] == "previous_response_id" {
		return true
	}
	return len(path) == 3 && path[0] == "input" && strings.HasPrefix(path[1], "[") && path[2] == "encrypted_content"
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

func shouldSkipPath(path []string) bool {
	if len(path) == 0 {
		return false
	}
	key := strings.ToLower(path[len(path)-1])
	if _, ok := controlKeys[key]; ok {
		return true
	}
	joined := strings.ToLower(strings.Join(path, "."))
	return strings.Contains(joined, "image_url") ||
		strings.Contains(joined, "input_image") ||
		strings.Contains(joined, "input_audio") ||
		strings.Contains(joined, "b64_json") ||
		strings.Contains(joined, "file_data") ||
		strings.HasSuffix(joined, "source.data")
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
