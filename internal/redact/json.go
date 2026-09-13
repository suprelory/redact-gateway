package redact

import (
	"encoding/json"
	"strings"
)

const RedactNotice = "Sensitive values are redacted before forwarding, including messages, tool inputs, and tool results. You may see {{RG_TYPE_TOKEN}} placeholders; treat them as opaque and preserve them exactly. Placeholders you emit in text or tool calls are restored by the local gateway."

var controlKeys = map[string]struct{}{
	"model": {}, "role": {}, "type": {}, "id": {}, "object": {}, "status": {},
	"name": {}, "call_id": {}, "tool_call_id": {}, "finish_reason": {}, "stop_reason": {},
	"media_type": {}, "mime_type": {}, "encoding": {}, "format": {},
}

func RedactJSON(value any, context *Context, flags DetectorFlags) (any, error) {
	return transformJSON(value, context, flags, nil)
}

func transformJSON(value any, context *Context, flags DetectorFlags, path []string) (any, error) {
	switch typed := value.(type) {
	case string:
		if shouldSkipPath(path) {
			return typed, nil
		}
		return context.RedactText(typed, flags)
	case []any:
		out := make([]any, len(typed))
		for index, child := range typed {
			mapped, err := transformJSON(child, context, flags, append(path, jsonIndex(index)))
			if err != nil {
				return nil, err
			}
			out[index] = mapped
		}
		return out, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			mapped, err := transformJSON(child, context, flags, append(path, key))
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

func RestoreJSON(value any, context *Context) any {
	switch typed := value.(type) {
	case string:
		return context.RestoreText(typed)
	case []any:
		for index := range typed {
			typed[index] = RestoreJSON(typed[index], context)
		}
		return typed
	case map[string]any:
		for key := range typed {
			typed[key] = RestoreJSON(typed[key], context)
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
			object["input"] = RedactNotice + "\n\n" + input
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
		message["content"] = RedactNotice + "\n\n" + content
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
				block["text"] = RedactNotice + "\n\n" + text
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
		message["content"] = RedactNotice
		return true
	}
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
	data, _ := json.Marshal(index)
	return string(data)
}
