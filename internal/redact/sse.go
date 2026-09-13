package redact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type SSEStreamRestorer struct {
	context *Context
	buffer  []byte
	events  *sseEventRestorer
}

func NewSSEStreamRestorer(context *Context) *SSEStreamRestorer {
	return &SSEStreamRestorer{
		context: context,
		events:  newSSEEventRestorer(context),
	}
}

func (r *SSEStreamRestorer) Push(chunk []byte) ([]byte, error) {
	r.buffer = append(r.buffer, chunk...)
	var output bytes.Buffer
	for {
		end, separatorLength := nextSSEEvent(r.buffer)
		if end < 0 {
			break
		}
		raw := string(r.buffer[:end])
		r.buffer = r.buffer[end+separatorLength:]
		produced, err := r.events.ingest(raw)
		if err != nil {
			return nil, err
		}
		output.WriteString(produced)
	}
	return output.Bytes(), nil
}

func (r *SSEStreamRestorer) Finish() ([]byte, error) {
	var output bytes.Buffer
	if len(r.buffer) > 0 {
		produced, err := r.events.ingest(string(r.buffer))
		if err != nil {
			return nil, err
		}
		output.WriteString(produced)
		r.buffer = nil
	}
	produced, err := r.events.finish()
	if err != nil {
		return nil, err
	}
	output.WriteString(produced)
	return output.Bytes(), nil
}

func nextSSEEvent(buffer []byte) (int, int) {
	lf := bytes.Index(buffer, []byte("\n\n"))
	crlf := bytes.Index(buffer, []byte("\r\n\r\n"))
	switch {
	case lf < 0 && crlf < 0:
		return -1, 0
	case lf < 0:
		return crlf, 4
	case crlf < 0:
		return lf, 2
	case lf < crlf:
		return lf, 2
	default:
		return crlf, 4
	}
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

type queuedEvent struct {
	parsed  sseEvent
	data    any
	safe    bool
	pending int
	direct  string
}

type fieldRecord struct {
	event *queuedEvent
	path  []pathPart
}

type channelState struct {
	text     string
	records  []fieldRecord
	jsonText bool
}

type sseEventRestorer struct {
	context  *Context
	channels map[string]*channelState
	queue    []*queuedEvent
}

func newSSEEventRestorer(context *Context) *sseEventRestorer {
	return &sseEventRestorer{
		context:  context,
		channels: make(map[string]*channelState),
	}
}

func (r *sseEventRestorer) ingest(raw string) (string, error) {
	parsed := parseSSEEvent(raw)
	if parsed.dataText == "" || parsed.dataText == "[DONE]" {
		r.queue = append(r.queue, &queuedEvent{safe: true, direct: parsed.raw + "\n\n"})
		return r.drain(false)
	}

	var data any
	decoder := json.NewDecoder(strings.NewReader(parsed.dataText))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		direct := parsed.serialize(r.context.RestoreText(parsed.dataText))
		r.queue = append(r.queue, &queuedEvent{safe: true, direct: direct})
		return r.drain(false)
	}

	fields := streamFields(data, parsed.eventName)
	excluded := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		excluded[pathKey(field.path)] = struct{}{}
	}
	restoreCompleteStrings(data, r.context, excluded, nil)
	event := &queuedEvent{parsed: parsed, data: data, safe: len(fields) == 0, pending: len(fields)}
	r.queue = append(r.queue, event)
	affected := make(map[string]struct{})
	for _, field := range fields {
		channel := r.channels[field.channel]
		if channel == nil {
			channel = &channelState{jsonText: field.jsonText}
			r.channels[field.channel] = channel
		}
		channel.text += field.source
		channel.records = append(channel.records, fieldRecord{event: event, path: field.path})
		affected[field.channel] = struct{}{}
	}
	for channel := range affected {
		if err := r.maybeFlushChannel(channel, false); err != nil {
			return "", err
		}
	}
	return r.drain(false)
}

func (r *sseEventRestorer) finish() (string, error) {
	for channel := range r.channels {
		if err := r.maybeFlushChannel(channel, true); err != nil {
			return "", err
		}
	}
	return r.drain(true)
}

func (r *sseEventRestorer) maybeFlushChannel(name string, force bool) error {
	channel := r.channels[name]
	if channel == nil || len(channel.records) == 0 {
		return nil
	}
	if !force && possiblePlaceholderSuffixLength(channel.text) > 0 {
		return nil
	}
	restored := restoreString(channel.text, r.context, channel.jsonText)
	for _, record := range channel.records {
		if err := setStringAt(record.event.data, record.path, ""); err != nil {
			return err
		}
	}
	last := channel.records[len(channel.records)-1]
	if err := setStringAt(last.event.data, last.path, restored); err != nil {
		return err
	}
	for _, record := range channel.records {
		record.event.pending--
		if record.event.pending == 0 {
			record.event.safe = true
		}
	}
	channel.text = ""
	channel.records = nil
	return nil
}

func (r *sseEventRestorer) drain(force bool) (string, error) {
	var output strings.Builder
	for len(r.queue) > 0 && (r.queue[0].safe || force) {
		event := r.queue[0]
		r.queue = r.queue[1:]
		if event.direct != "" {
			output.WriteString(event.direct)
			continue
		}
		data, err := json.Marshal(event.data)
		if err != nil {
			return "", err
		}
		output.WriteString(event.parsed.serialize(string(data)))
	}
	return output.String(), nil
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
					fmt.Sprintf("chat:%d:delta", channelIndex), &fields, nil)
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
		collectStringLeaves(delta, []pathPart{keyPart("delta")}, "delta:"+typeName+":"+streamIdentity(object), &fields, nil)
	}
	if completion, ok := object["completion"].(string); ok {
		fields = append(fields, streamField{path: []pathPart{keyPart("completion")}, channel: "anthropic:completion", source: completion})
	}
	return fields
}

func collectStringLeaves(value any, base []pathPart, channelPrefix string, fields *[]streamField, local []pathPart) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := appendPath(local, keyPart(key))
			if text, ok := child.(string); ok {
				if isStreamMetadataKey(key) {
					continue
				}
				*fields = append(*fields, streamField{
					path: appendPath(base, next...), channel: channelPrefix + ":" + pathKey(next), source: text,
					jsonText: isJSONTextField(key),
				})
				continue
			}
			collectStringLeaves(child, base, channelPrefix, fields, next)
		}
	case []any:
		for index, child := range typed {
			collectStringLeaves(child, base, channelPrefix, fields, appendPath(local, indexPart(index)))
		}
	}
}

func restoreCompleteStrings(value any, context *Context, excluded map[string]struct{}, path []pathPart) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			next := appendPath(path, keyPart(key))
			if text, ok := child.(string); ok {
				if _, skip := excluded[pathKey(next)]; !skip {
					typed[key] = restoreString(text, context, isJSONTextField(key))
				}
				continue
			}
			restoreCompleteStrings(child, context, excluded, next)
		}
	case []any:
		for index, child := range typed {
			next := appendPath(path, indexPart(index))
			if text, ok := child.(string); ok {
				if _, skip := excluded[pathKey(next)]; !skip {
					typed[index] = context.RestoreText(text)
				}
				continue
			}
			restoreCompleteStrings(child, context, excluded, next)
		}
	}
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
	keys := []string{"output_index", "content_index", "index", "item_id"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := object[key]; ok {
			parts = append(parts, fmt.Sprint(value))
		}
	}
	return strings.Join(parts, ":")
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

func possiblePlaceholderSuffixLength(text string) int {
	start := len(text) - 64
	if start < 0 {
		start = 0
	}
	for index := len(text) - 1; index >= start; index-- {
		if text[index] == '{' && isPlaceholderPrefix(text[index:]) {
			return len(text) - index
		}
	}
	return 0
}

func isPlaceholderPrefix(value string) bool {
	fixed := "{{RG_"
	if len(value) <= len(fixed) {
		return strings.HasPrefix(fixed, value)
	}
	if !strings.HasPrefix(value, fixed) {
		return false
	}
	rest := value[len(fixed):]
	separator := strings.IndexByte(rest, '_')
	if separator < 0 {
		return len(rest) <= 32 && validLabelPrefix(rest)
	}
	label := rest[:separator]
	if len(label) == 0 || len(label) > 32 || !validLabelPrefix(label) {
		return false
	}
	tail := rest[separator+1:]
	idLength := 0
	for idLength < len(tail) && idLength < 16 && isBase32(tail[idLength]) {
		idLength++
	}
	if idLength < len(tail) && idLength < 16 {
		return false
	}
	if idLength < 16 {
		return idLength == len(tail)
	}
	switch tail[idLength:] {
	case "", "}":
		return true
	case "}}":
		return false
	default:
		return false
	}
}

func validLabelPrefix(value string) bool {
	for index := 0; index < len(value); index++ {
		b := value[index]
		if b < 'A' || b > 'Z' {
			if index == 0 || b < '0' || b > '9' {
				return false
			}
		}
	}
	return true
}

func isBase32(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= '2' && value <= '7'
}
