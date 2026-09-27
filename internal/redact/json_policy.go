package redact

import "strconv"

// Exemptions belong to actual protocol locations, never to a key name found
// anywhere in business data. Serialized JSON values always use generic policy.
func protectedJSONValue(root any, protocol string, path []string, value any) bool {
	if len(path) == 0 || protocol != "openai_chat" && protocol != "openai_responses" && protocol != "anthropic_messages" || !protocolPathShape(root, path) {
		return false
	}
	if historicalModelState(root, protocol, path) || schemaControlPath(protocol, path) && schemaControlValue(path[len(path)-1], value) {
		return true
	}
	// Protocol identifiers and media payloads are strings. Unexpected containers
	// or numbers in those slots must still undergo normal validation.
	if _, ok := value.(string); !ok {
		return false
	}
	if pathIs(path, "model") {
		return true
	}
	if protocol == "openai_responses" {
		if pathIs(path, "previous_response_id") || pathIs(path, "conversation") || pathIs(path, "conversation", "id") ||
			pathIs(path, "prompt", "id") || pathIs(path, "prompt", "version") {
			return true
		}
		if len(path) == 3 && pathIs(path[:2], "input", "#") {
			item := objectAt(root, path[:2])
			kind, _ := item["type"].(string)
			key := path[2]
			if key == "type" || key == "role" && isMessage(item) {
				return true
			}
			switch kind {
			case "message", "item_reference", "function_call", "function_call_output", "custom_tool_call", "custom_tool_call_output", "computer_call", "computer_call_output", "web_search_call", "file_search_call":
				if key == "id" || key == "status" || key == "call_id" {
					return true
				}
				return key == "name" && (kind == "function_call" || kind == "custom_tool_call")
			}
		}
	} else if len(path) == 3 && pathIs(path[:2], "messages", "#") && isMessage(objectAt(root, path[:2])) {
		if path[2] == "role" || protocol == "openai_chat" && (path[2] == "name" || path[2] == "tool_call_id") {
			return true
		}
	}
	if protocol == "openai_chat" && len(path) >= 3 && pathIs(path[:2], "messages", "#") && objectAt(root, path[:2])["role"] == "assistant" {
		if pathIs(path, "messages", "#", "tool_calls", "#", "id") || pathIs(path, "messages", "#", "tool_calls", "#", "type") ||
			pathIs(path, "messages", "#", "tool_calls", "#", "function", "name") || pathIs(path, "messages", "#", "function_call", "name") {
			return true
		}
	}
	if protocolToolControl(protocol, path) {
		return true
	}

	key := path[len(path)-1]
	parentPath := path[:len(path)-1]
	if contentBlockPath(root, protocol, parentPath) {
		kind, _ := objectAt(root, parentPath)["type"].(string)
		if key == "type" {
			return true
		}
		switch kind {
		case "tool_use", "server_tool_use":
			return protocol == "anthropic_messages" && (key == "id" || key == "name")
		case "tool_result", "web_search_tool_result", "web_fetch_tool_result":
			return protocol == "anthropic_messages" && key == "tool_use_id"
		case "input_image":
			return protocol == "openai_responses" && (key == "image_url" || key == "file_id")
		case "image_url":
			return protocol == "openai_chat" && key == "image_url"
		case "input_file":
			return protocol == "openai_responses" && (key == "file_data" || key == "file_id" || key == "file_url")
		}
	}
	if len(path) >= 2 && contentBlockPath(root, protocol, path[:len(path)-2]) {
		kind, _ := objectAt(root, path[:len(path)-2])["type"].(string)
		container := path[len(path)-2]
		switch {
		case protocol == "openai_chat" && kind == "image_url" && container == "image_url":
			return key == "url" || key == "detail"
		case protocol == "openai_chat" && kind == "input_audio" && container == "input_audio":
			return key == "data" || key == "format"
		case protocol == "openai_chat" && kind == "file" && container == "file":
			return key == "file_data" || key == "file_id"
		case (kind == "image" || kind == "document") && container == "source" && protocol == "anthropic_messages":
			source := objectAt(root, parentPath)
			if key == "type" || key == "media_type" {
				return true
			}
			return source["type"] == "base64" && key == "data" || source["type"] == "url" && key == "url"
		}
	}
	return false
}

func schemaControlValue(key string, value any) bool {
	if _, ok := value.(string); ok {
		return true
	}
	stringsOnly := func(value any) bool {
		values, ok := value.([]any)
		if !ok {
			return false
		}
		for _, entry := range values {
			if _, ok := entry.(string); !ok {
				return false
			}
		}
		return true
	}
	if key == "type" || key == "required" {
		return stringsOnly(value)
	}
	if key == "dependentRequired" {
		dependencies, ok := value.(map[string]any)
		if !ok {
			return false
		}
		for _, names := range dependencies {
			if !stringsOnly(names) {
				return false
			}
		}
		return true
	}
	return false
}

// A map key spelled "[0]" must not impersonate a protocol array element.
func protocolPathShape(root any, path []string) bool {
	value := root
	for _, key := range path {
		switch node := value.(type) {
		case map[string]any:
			if isJSONIndex(key) {
				return false
			}
			value = node[key]
		case []any:
			if !isJSONIndex(key) {
				return false
			}
			index, _ := strconv.Atoi(key[1 : len(key)-1])
			if index >= len(node) {
				return false
			}
			value = node[index]
		default:
			return false
		}
	}
	return true
}

func historicalModelState(root any, protocol string, path []string) bool {
	if protocol == "openai_responses" && pathIs(path, "input", "#") {
		item := objectAt(root, path)
		role, _ := item["role"].(string)
		return (item["type"] == "reasoning" || item["type"] == "compaction") && (role == "" || role == "assistant")
	}
	if protocol == "openai_chat" && len(path) == 3 && pathIs(path[:2], "messages", "#") && objectAt(root, path[:2])["role"] == "assistant" {
		return path[2] == "reasoning_content" || path[2] == "reasoning" || path[2] == "reasoning_details"
	}
	if protocol == "anthropic_messages" && pathIs(path, "messages", "#", "content", "#") && objectAt(root, path[:2])["role"] == "assistant" {
		block := objectAt(root, path)
		return block["type"] == "thinking" || block["type"] == "redacted_thinking"
	}
	return false
}

func contentBlockPath(root any, protocol string, path []string) bool {
	if objectAt(root, path) == nil {
		return false
	}
	if protocol == "openai_responses" {
		return pathIs(path, "input", "#", "content", "#") && isMessage(objectAt(root, path[:2]))
	}
	if pathIs(path, "messages", "#", "content", "#") && isMessage(objectAt(root, path[:2])) {
		return true
	}
	if protocol != "anthropic_messages" {
		return false
	}
	if pathIs(path, "system", "#") {
		return true
	}
	// Anthropic tool results and document sources can themselves contain blocks.
	if len(path) > 2 && pathIs(path[len(path)-2:], "content", "#") {
		parent := path[:len(path)-2]
		if contentBlockPath(root, protocol, parent) && objectAt(root, parent)["type"] == "tool_result" {
			return true
		}
		if len(parent) > 0 && parent[len(parent)-1] == "source" && objectAt(root, parent)["type"] == "content" {
			return contentBlockPath(root, protocol, parent[:len(parent)-1]) && objectAt(root, parent[:len(parent)-1])["type"] == "document"
		}
	}
	return false
}

func isMessage(value map[string]any) bool {
	role, _ := value["role"].(string)
	switch role {
	case "system", "developer", "user", "assistant", "tool", "function":
		return true
	}
	return value["type"] == "message"
}

func protocolToolControl(protocol string, path []string) bool {
	if pathIs(path, "tools", "#", "type") || pathIs(path, "tool_choice", "type") {
		return true
	}
	switch protocol {
	case "openai_chat":
		return pathIs(path, "tools", "#", "function", "name") || pathIs(path, "functions", "#", "name") ||
			pathIs(path, "tool_choice", "function", "name") || pathIs(path, "function_call", "name") ||
			pathIs(path, "response_format", "type") || pathIs(path, "response_format", "json_schema", "name")
	case "openai_responses":
		return pathIs(path, "tools", "#", "name") || pathIs(path, "tool_choice", "name") ||
			pathIs(path, "text", "format", "type") || pathIs(path, "text", "format", "name")
	case "anthropic_messages":
		return pathIs(path, "tools", "#", "name") || pathIs(path, "tool_choice", "name") || pathIs(path, "output_config", "format", "type")
	}
	return false
}

func schemaControlPath(protocol string, path []string) bool {
	var roots [][]string
	switch protocol {
	case "openai_chat":
		roots = [][]string{{"tools", "#", "function", "parameters"}, {"functions", "#", "parameters"}, {"response_format", "json_schema", "schema"}}
	case "openai_responses":
		roots = [][]string{{"tools", "#", "parameters"}, {"text", "format", "schema"}}
	case "anthropic_messages":
		roots = [][]string{{"tools", "#", "input_schema"}, {"output_config", "format", "schema"}}
	}
	for _, prefix := range roots {
		if len(path) <= len(prefix) || !pathIs(path[:len(prefix)], prefix...) {
			continue
		}
		tail := path[len(prefix):]
		for index := 0; index < len(tail); {
			switch tail[index] {
			case "type", "format", "$ref", "$dynamicRef", "$schema", "$id", "$anchor", "$dynamicAnchor", "required", "dependentRequired", "pattern":
				return index == len(tail)-1
			case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
				index += 2 // Property/schema names are unchanged JSON keys.
			case "allOf", "anyOf", "oneOf", "prefixItems":
				if index+1 >= len(tail) || !isJSONIndex(tail[index+1]) {
					return false
				}
				index += 2
			case "items":
				index++
				if index < len(tail) && isJSONIndex(tail[index]) {
					index++
				}
			case "additionalProperties", "unevaluatedProperties", "additionalItems", "unevaluatedItems", "contains", "not", "if", "then", "else", "propertyNames", "contentSchema":
				index++
			default:
				// Descriptions, examples, defaults, enum and const are data.
				return false
			}
		}
	}
	return false
}

func pathIs(path []string, pattern ...string) bool {
	if len(path) != len(pattern) {
		return false
	}
	for index, key := range pattern {
		if key == "#" {
			if !isJSONIndex(path[index]) {
				return false
			}
		} else if key != path[index] {
			return false
		}
	}
	return true
}

func isJSONIndex(key string) bool {
	if len(key) < 3 || key[0] != '[' || key[len(key)-1] != ']' {
		return false
	}
	index, err := strconv.Atoi(key[1 : len(key)-1])
	return err == nil && index >= 0
}

func objectAt(root any, path []string) map[string]any {
	value := root
	for _, key := range path {
		switch node := value.(type) {
		case map[string]any:
			value = node[key]
		case []any:
			if !isJSONIndex(key) {
				return nil
			}
			index, _ := strconv.Atoi(key[1 : len(key)-1])
			if index >= len(node) {
				return nil
			}
			value = node[index]
		default:
			return nil
		}
	}
	object, _ := value.(map[string]any)
	return object
}
