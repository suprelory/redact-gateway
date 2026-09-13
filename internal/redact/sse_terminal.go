package redact

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

type streamScope struct {
	kind                            string
	owner                           string
	choice, block, content, summary int
}

func streamEventType(object map[string]any, eventName string) string {
	if kind, _ := object["type"].(string); kind != "" {
		return kind
	}
	return eventName
}

func streamIndex(object map[string]any, key string, fallback int) int {
	if number, ok := object[key].(json.Number); ok {
		if value, err := number.Int64(); err == nil && value >= 0 {
			return int(value)
		}
	}
	return fallback
}

func responseOwner(object map[string]any) string {
	if index := streamIndex(object, "output_index", -1); index >= 0 {
		return fmt.Sprintf("output:%d", index)
	}
	id, _ := object["item_id"].(string)
	if id == "" {
		if item, ok := object["item"].(map[string]any); ok {
			id, _ = item["id"].(string)
		}
	}
	if id != "" {
		// Bound routing metadata even if a compatible upstream sends a huge ID.
		sum := sha256.Sum256([]byte(id))
		return fmt.Sprintf("item:%x", sum)
	}
	return "output:0"
}

func scopeForField(data any, eventName string, field streamField) streamScope {
	object, _ := data.(map[string]any)
	scope := streamScope{kind: streamEventType(object, eventName), owner: responseOwner(object), block: streamIndex(object, "index", 0), content: streamIndex(object, "content_index", 0), summary: streamIndex(object, "summary_index", 0)}
	if len(field.path) >= 2 && field.path[0].key == "choices" {
		choice := object["choices"].([]any)[field.path[1].index].(map[string]any)
		scope.kind, scope.choice = "chat", streamIndex(choice, "index", field.path[1].index)
	}
	if field.channel == "anthropic:completion" {
		scope.kind = "completion"
	}
	return scope
}

func (s streamScope) ends(data any, eventName string) bool {
	object, ok := data.(map[string]any)
	if !ok {
		return false
	}
	kind := streamEventType(object, eventName)
	switch kind {
	case "response.completed", "response.failed", "response.incomplete", "error", "message_stop":
		return true
	case "message_delta":
		if delta, ok := object["delta"].(map[string]any); ok {
			if reason, _ := delta["stop_reason"].(string); reason != "" {
				return true
			}
		}
	case "content_block_stop":
		return s.kind == "content_block_delta" && s.block == streamIndex(object, "index", 0)
	}
	if s.kind == "chat" {
		if choices, ok := object["choices"].([]any); ok {
			for position, raw := range choices {
				if choice, ok := raw.(map[string]any); ok && streamIndex(choice, "index", position) == s.choice {
					if reason, _ := choice["finish_reason"].(string); reason != "" {
						return true
					}
				}
			}
		}
	}
	if s.kind == "completion" {
		reason, _ := object["stop_reason"].(string)
		return reason != ""
	}
	if !strings.HasPrefix(s.kind, "response.") || responseOwner(object) != s.owner {
		return false
	}
	switch kind {
	case "response.output_item.done":
		return true
	case "response.content_part.done":
		return s.content == streamIndex(object, "content_index", 0)
	case "response.reasoning_summary_part.done":
		return s.summary == streamIndex(object, "summary_index", 0)
	}
	return strings.HasSuffix(kind, ".done") && s.kind == strings.TrimSuffix(kind, ".done")+".delta" && s.content == streamIndex(object, "content_index", 0) && s.summary == streamIndex(object, "summary_index", 0)
}

func (s streamScope) snapshot(data any, eventName string) (string, bool) {
	object, ok := data.(map[string]any)
	if !ok || !s.ends(data, eventName) {
		return "", false
	}
	kind := streamEventType(object, eventName)
	if s.kind == strings.TrimSuffix(kind, ".done")+".delta" {
		key := "text"
		if kind == "response.function_call_arguments.done" {
			key = "arguments"
		}
		if kind == "response.refusal.done" {
			key = "refusal"
		}
		value, ok := object[key].(string)
		return value, ok
	}
	switch kind {
	case "response.content_part.done", "response.reasoning_summary_part.done":
		part, _ := object["part"].(map[string]any)
		return s.partSnapshot(part)
	case "response.output_item.done":
		item, _ := object["item"].(map[string]any)
		return s.itemSnapshot(item)
	case "response.completed", "response.failed", "response.incomplete":
		response, _ := object["response"].(map[string]any)
		output, _ := response["output"].([]any)
		for index, raw := range output {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			identity := map[string]any{"item": item}
			if s.owner == fmt.Sprintf("output:%d", index) || responseOwner(identity) == s.owner {
				return s.itemSnapshot(item)
			}
		}
	}
	return "", false
}

func (s streamScope) partSnapshot(part map[string]any) (string, bool) {
	key := "text"
	if s.kind == "response.refusal.delta" {
		key = "refusal"
	}
	value, ok := part[key].(string)
	return value, ok
}

func (s streamScope) itemSnapshot(item map[string]any) (string, bool) {
	if s.kind == "response.function_call_arguments.delta" {
		value, ok := item["arguments"].(string)
		return value, ok
	}
	key, index := "content", s.content
	if strings.HasPrefix(s.kind, "response.reasoning_summary_") {
		key, index = "summary", s.summary
	}
	parts, _ := item[key].([]any)
	if index < 0 || index >= len(parts) {
		return "", false
	}
	part, _ := parts[index].(map[string]any)
	return s.partSnapshot(part)
}
