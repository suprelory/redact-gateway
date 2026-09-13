package redact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type SSEStreamRestorer struct {
	buffer        []byte
	events        *sseEventRestorer
	maxEventBytes int
	framer        sseFramer
	finished      bool
	failure       error
}

func NewSSEStreamRestorer(context *Context) *SSEStreamRestorer {
	return NewSSEStreamRestorerWithLimit(context, 32*1024*1024)
}

func NewSSEStreamRestorerWithLimit(context *Context, maxEventBytes int) *SSEStreamRestorer {
	if maxEventBytes <= 0 {
		maxEventBytes = 32 * 1024 * 1024
	}
	return &SSEStreamRestorer{
		events:        newSSEEventRestorer(context),
		maxEventBytes: maxEventBytes,
	}
}

func (r *SSEStreamRestorer) Push(chunk []byte) ([]byte, error) {
	if r.failure != nil {
		return nil, r.failure
	}
	if r.finished {
		return nil, fmt.Errorf("SSE restorer already finished")
	}
	var output bytes.Buffer
	for len(chunk) > 0 {
		size := min(len(chunk), 32*1024)
		r.buffer = append(r.buffer, chunk[:size]...)
		chunk = chunk[size:]
		produced, err := r.consume(false)
		output.Write(produced)
		if err != nil {
			r.fail(err)
			return output.Bytes(), err
		}
	}
	return output.Bytes(), nil
}

func (r *SSEStreamRestorer) Finish() ([]byte, error) {
	if r.failure != nil || r.finished {
		return nil, r.failure
	}
	r.finished = true
	var output bytes.Buffer
	complete, err := r.consume(true)
	output.Write(complete)
	if err != nil {
		r.fail(err)
		return output.Bytes(), err
	}
	if len(r.buffer) > 0 {
		produced, err := r.events.ingest(string(r.buffer))
		output.WriteString(produced)
		r.buffer = nil
		if err != nil {
			r.fail(err)
			return output.Bytes(), err
		}
	}
	produced, err := r.events.finish()
	output.WriteString(produced)
	if err != nil {
		r.fail(err)
		return output.Bytes(), err
	}
	return output.Bytes(), nil
}

func (r *SSEStreamRestorer) fail(err error) {
	r.failure = err
	r.buffer = nil
	clear(r.events.channels)
	r.events.retained = 0
}

func (r *SSEStreamRestorer) consume(final bool) ([]byte, error) {
	var output bytes.Buffer
	for {
		end, separatorLength := r.framer.next(r.buffer, final)
		if end < 0 {
			// A separator can still be incomplete at the end of a read.
			if len(r.buffer) > r.maxEventBytes && (len(r.buffer)-r.maxEventBytes > 3 || len(bytes.TrimRight(r.buffer, "\r\n")) > r.maxEventBytes) {
				return output.Bytes(), ErrSSELimit
			}
			return output.Bytes(), nil
		}
		if end > r.maxEventBytes {
			return output.Bytes(), ErrSSELimit
		}
		raw := string(r.buffer[:end])
		r.buffer = r.buffer[end+separatorLength:]
		if len(r.buffer) == 0 {
			r.buffer = nil
		} else if end > 64*1024 {
			r.buffer = bytes.Clone(r.buffer)
		}
		r.framer = sseFramer{}
		produced, err := r.events.ingest(raw)
		output.WriteString(produced)
		if err != nil {
			return output.Bytes(), err
		}
	}
}

// ErrorClass reports application failures carried inside an HTTP 200 stream.
// It never copies upstream error messages, which may contain sensitive data.
func (r *SSEStreamRestorer) ErrorClass() string { return r.events.errorClass }

type sseFramer struct {
	position, lineStart, previousEnd int
}

func (s *sseFramer) next(buffer []byte, final bool) (int, int) {
	for s.position < len(buffer) {
		position := s.position
		if buffer[position] != '\n' && buffer[position] != '\r' {
			s.position++
			continue
		}
		length := 1
		if buffer[position] == '\r' {
			if position+1 == len(buffer) && !final {
				break
			} // CRLF may be split across reads.
			if position+1 < len(buffer) && buffer[position+1] == '\n' {
				length = 2
			}
		}
		if position == s.lineStart {
			return s.previousEnd, position + length - s.previousEnd
		}
		s.previousEnd = position
		s.lineStart = position + length
		s.position = position + length
	}
	return -1, 0
}

type sseEvent struct {
	raw       string
	nonData   []string
	insertAt  int
	dataText  string
	eventName string
}

func parseSSEEvent(raw string) sseEvent {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	event := sseEvent{raw: normalized, insertAt: -1}
	var dataLines []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "data:"):
			if event.insertAt < 0 {
				event.insertAt = len(event.nonData)
			}
			dataLines = append(dataLines, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		case strings.HasPrefix(line, "event:"):
			event.eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			event.nonData = append(event.nonData, line)
		default:
			event.nonData = append(event.nonData, line)
		}
	}
	event.dataText = strings.Join(dataLines, "\n")
	return event
}

func (e sseEvent) serialize(dataText string) string {
	if e.insertAt < 0 {
		return e.raw + "\n\n"
	}
	lines := append([]string(nil), e.nonData...)
	dataLines := strings.Split(dataText, "\n")
	formatted := make([]string, len(dataLines))
	for index, line := range dataLines {
		formatted[index] = "data: " + line
	}
	lines = append(lines, make([]string, len(formatted))...)
	copy(lines[e.insertAt+len(formatted):], lines[e.insertAt:])
	copy(lines[e.insertAt:], formatted)
	return strings.Join(lines, "\n") + "\n\n"
}

type pathPart struct {
	key     string
	index   int
	isIndex bool
}

func keyPart(key string) pathPart  { return pathPart{key: key} }
func indexPart(index int) pathPart { return pathPart{index: index, isIndex: true} }

type streamField struct {
	path     []pathPart
	channel  string
	source   string
	jsonText bool
}

func streamFields(data any, eventName string) []streamField {
	object, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	var fields []streamField
	if choices, ok := object["choices"].([]any); ok {
		for choiceIndex, rawChoice := range choices {
			choice, ok := rawChoice.(map[string]any)
			if !ok {
				continue
			}
			channelIndex := choiceIndex
			if value, ok := choice["index"].(json.Number); ok {
				if parsed, err := value.Int64(); err == nil {
					channelIndex = int(parsed)
				}
			}
			if delta, ok := choice["delta"].(map[string]any); ok {
				collectStringLeaves(delta,
					[]pathPart{keyPart("choices"), indexPart(choiceIndex), keyPart("delta")},
					fmt.Sprintf("chat:%d:delta", channelIndex), &fields, nil, nil)
			}
			if text, ok := choice["text"].(string); ok {
				fields = append(fields, streamField{
					path:    []pathPart{keyPart("choices"), indexPart(choiceIndex), keyPart("text")},
					channel: fmt.Sprintf("chat:%d:text", channelIndex), source: text,
				})
			}
		}
	}

	typeName, _ := object["type"].(string)
	if typeName == "" {
		typeName = eventName
	}
	switch delta := object["delta"].(type) {
	case string:
		if strings.Contains(strings.ToLower(typeName), "delta") {
			fields = append(fields, streamField{
				path:    []pathPart{keyPart("delta")},
				channel: "delta:" + typeName + ":" + streamIdentity(object), source: delta,
				jsonText: typeName == "response.function_call_arguments.delta",
			})
		}
	case map[string]any:
		collectStringLeaves(delta, []pathPart{keyPart("delta")}, "delta:"+typeName+":"+streamIdentity(object), &fields, nil, nil)
	}
	if completion, ok := object["completion"].(string); ok {
		fields = append(fields, streamField{path: []pathPart{keyPart("completion")}, channel: "anthropic:completion", source: completion})
	}
	return fields
}

func collectStringLeaves(value any, base []pathPart, channelPrefix string, fields *[]streamField, local, logical []pathPart) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := appendPath(local, keyPart(key))
			nextLogical := appendPath(logical, keyPart(key))
			if text, ok := child.(string); ok {
				if isStreamMetadataKey(key) {
					continue
				}
				*fields = append(*fields, streamField{
					path: appendPath(base, next...), channel: channelPrefix + ":" + pathKey(nextLogical), source: text,
					jsonText: isJSONTextField(key),
				})
				continue
			}
			collectStringLeaves(child, base, channelPrefix, fields, next, nextLogical)
		}
	case []any:
		for index, child := range typed {
			channelIndex := index
			if len(local) > 0 && local[len(local)-1].key == "tool_calls" {
				// Sparse chunks can put different tools at array position zero.
				// Only the logical channel uses the protocol index; writes still
				// target the original position in this event's JSON array.
				if call, ok := child.(map[string]any); ok {
					if number, ok := call["index"].(json.Number); ok {
						if parsed, err := number.Int64(); err == nil && parsed >= 0 {
							channelIndex = int(parsed)
						}
					}
				}
			}
			collectStringLeaves(child, base, channelPrefix, fields,
				appendPath(local, indexPart(index)), appendPath(logical, indexPart(channelIndex)))
		}
	}
}

func restoreCompleteStrings(value any, context *Context, excluded map[string]struct{}, path []pathPart) any {
	switch typed := value.(type) {
	case string:
		if _, skip := excluded[pathKey(path)]; skip {
			return typed
		}
		jsonText := len(path) > 0 && isJSONTextField(path[len(path)-1].key)
		return restoreString(typed, context, jsonText)
	case map[string]any:
		for key, child := range typed {
			next := appendPath(path, keyPart(key))
			typed[key] = restoreCompleteStrings(child, context, excluded, next)
		}
	case []any:
		for index, child := range typed {
			next := appendPath(path, indexPart(index))
			typed[index] = restoreCompleteStrings(child, context, excluded, next)
		}
	}
	return value
}

func setStringAt(root any, path []pathPart, value string) error {
	if len(path) == 0 {
		return fmt.Errorf("empty JSON path")
	}
	current := root
	for _, part := range path[:len(path)-1] {
		if part.isIndex {
			array, ok := current.([]any)
			if !ok || part.index < 0 || part.index >= len(array) {
				return fmt.Errorf("invalid JSON array path")
			}
			current = array[part.index]
		} else {
			object, ok := current.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid JSON object path")
			}
			current = object[part.key]
		}
	}
	last := path[len(path)-1]
	if last.isIndex {
		array, ok := current.([]any)
		if !ok || last.index < 0 || last.index >= len(array) {
			return fmt.Errorf("invalid JSON array target")
		}
		array[last.index] = value
		return nil
	}
	object, ok := current.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid JSON object target")
	}
	object[last.key] = value
	return nil
}

func streamIdentity(object map[string]any) string {
	owner := responseOwner(object)
	return fmt.Sprintf("%s:content:%d:summary:%d:block:%d", owner, streamIndex(object, "content_index", 0), streamIndex(object, "summary_index", 0), streamIndex(object, "index", 0))
}

func isStreamMetadataKey(key string) bool {
	switch key {
	case "role", "type", "id", "object", "status", "finish_reason", "stop_reason", "name":
		return true
	default:
		return false
	}
}

func pathKey(path []pathPart) string {
	var out strings.Builder
	for index, part := range path {
		if index > 0 {
			out.WriteByte('.')
		}
		if part.isIndex {
			fmt.Fprintf(&out, "%d", part.index)
		} else {
			out.WriteString(part.key)
		}
	}
	return out.String()
}

func appendPath(path []pathPart, parts ...pathPart) []pathPart {
	out := make([]pathPart, 0, len(path)+len(parts))
	out = append(out, path...)
	out = append(out, parts...)
	return out
}
