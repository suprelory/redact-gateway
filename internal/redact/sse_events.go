package redact

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"sort"
	"strconv"
	"strings"
)

var ErrSSELimit = errors.New("SSE buffer limit exceeded")

const (
	maxSSEChannels      = 1024
	maxSSEBufferedBytes = 4 * 1024 * 1024
)

type channelState struct {
	text        string
	template    string
	path        []pathPart
	scope       streamScope
	jsonText    bool
	previous    byte
	hasPrevious bool
	committed   int
	prefixHash  hash.Hash
}

func (c *channelState) restore(text string, context *Context) string {
	source := text
	view := text
	if c.jsonText {
		view, _ = jsonFragmentView(text)
	}
	guard := c.hasPrevious && identifierByte(c.previous) && len(view) > 0 && (view[0] == 'R' || view[0] == 'r')
	if guard {
		text = string(c.previous) + text
	}
	restored := restoreString(text, context, c.jsonText)
	if guard {
		restored = restored[1:]
	}
	if len(source) > 0 {
		if len(view) > 0 {
			c.previous, c.hasPrevious = view[len(view)-1], true
		}
		c.committed += len(source)
		if c.prefixHash == nil {
			c.prefixHash = sha256.New()
		}
		c.prefixHash.Write([]byte(source))
	}
	return restored
}

func (c *channelState) snapshotTail(snapshot string) (string, bool) {
	if c.committed > len(snapshot) {
		return "", false
	}
	if c.committed == 0 {
		return snapshot, true
	}
	sum := sha256.Sum256([]byte(snapshot[:c.committed]))
	if c.prefixHash == nil || !bytes.Equal(sum[:], c.prefixHash.Sum(nil)) {
		return "", false
	}
	return snapshot[c.committed:], true
}

type sseEventRestorer struct {
	context          *Context
	channels         map[string]*channelState
	errorClass       string
	retained         int
	maxChannels      int
	maxBufferedBytes int
	sequenceShift    int64
	lastSequence     int64
	hasSequence      bool
}

func newSSEEventRestorer(context *Context) *sseEventRestorer {
	return &sseEventRestorer{context: context, channels: make(map[string]*channelState), maxChannels: maxSSEChannels, maxBufferedBytes: maxSSEBufferedBytes}
}

func decodeSSEJSON(text string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var data, extra any
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid SSE JSON suffix")
	}
	return data, nil
}

func (r *sseEventRestorer) ingest(raw string) (string, error) {
	parsed := parseSSEEvent(raw)
	if parsed.dataText == "[DONE]" {
		tail, err := r.finish()
		return tail + parsed.raw + "\n\n", err
	}
	if parsed.dataText == "" {
		return parsed.raw + "\n\n", nil
	}
	data, err := decodeSSEJSON(parsed.dataText)
	if err != nil {
		return parsed.serialize(r.context.RestoreText(parsed.dataText)), nil
	}
	if r.errorClass == "" {
		r.errorClass = streamErrorClass(data, parsed.eventName)
	}
	fields := streamFields(data, parsed.eventName)
	excluded := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		excluded[pathKey(field.path)] = struct{}{}
	}
	// Capture authoritative full values before restoring their string leaves.
	snapshots := make(map[string]string)
	for name, channel := range r.channels {
		if snapshot, ok := channel.scope.snapshot(data, parsed.eventName); ok {
			snapshots[name] = snapshot
		}
	}
	data = restoreCompleteStrings(data, r.context, excluded, nil)
	var pending []string
	for _, field := range fields {
		channel := r.channels[field.channel]
		if channel == nil {
			if len(r.channels) >= r.maxChannels {
				return "", ErrSSELimit
			}
			channel = &channelState{jsonText: field.jsonText, scope: scopeForField(data, parsed.eventName, field)}
			r.channels[field.channel] = channel
			r.retained += len(field.channel)
		}
		text := channel.text + field.source
		hold := possiblePlaceholderSuffixLength(text)
		if channel.jsonText {
			hold = jsonPlaceholderSuffixLength(text)
		}
		if channel.scope.ends(data, parsed.eventName) {
			// Some Chat/legacy completion streams attach the final text and
			// finish reason to the same event. Keep that text in its original
			// event so a synthesized suffix cannot precede its own prefix.
			hold = 0
		}
		confirmed := text[:len(text)-hold]
		if err := setStringAt(data, field.path, channel.restore(confirmed, r.context)); err != nil {
			return "", err
		}
		r.retained -= len(channel.text) + len(channel.template)
		channel.text, channel.template = strings.Clone(text[len(text)-hold:]), ""
		channel.path = field.path
		r.retained += len(channel.text)
		if hold > 0 {
			pending = append(pending, field.channel)
		}
	}
	if len(pending) > 0 {
		encoded, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		blank, err := decodeSSEJSON(string(encoded))
		if err != nil {
			return "", err
		}
		for _, field := range fields {
			if err := setStringAt(blank, field.path, ""); err != nil {
				return "", err
			}
		}
		clearSyntheticMetadata(blank)
		encoded, err = json.Marshal(blank)
		if err != nil {
			return "", err
		}
		template := parsed.serialize(string(encoded))
		for _, name := range pending {
			r.channels[name].template = template
			r.retained += len(template)
		}
	}
	if r.retained > r.maxBufferedBytes {
		return "", ErrSSELimit
	}
	tail, err := r.flush(func(c *channelState) bool { return c.scope.ends(data, parsed.eventName) }, snapshots, data)
	if err != nil {
		return tail, err
	}
	encoded, err := r.encode(data, false, nil)
	if err != nil {
		return tail, err
	}
	return tail + parsed.serialize(encoded), nil
}

func (r *sseEventRestorer) finish() (string, error) {
	return r.flush(func(*channelState) bool { return true }, nil, nil)
}

func (r *sseEventRestorer) flush(matches func(*channelState) bool, snapshots map[string]string, before any) (string, error) {
	var names []string
	for name, channel := range r.channels {
		if matches(channel) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var output strings.Builder
	for _, name := range names {
		channel := r.channels[name]
		if channel.text != "" && channel.template != "" {
			text := channel.text
			if snapshot, ok := snapshots[name]; ok {
				if tail, consistent := channel.snapshotTail(snapshot); consistent {
					text = tail
				}
			}
			parsed := parseSSEEvent(strings.TrimSuffix(channel.template, "\n\n"))
			data, err := decodeSSEJSON(parsed.dataText)
			if err != nil {
				return output.String(), err
			}
			if err := setStringAt(data, channel.path, channel.restore(text, r.context)); err != nil {
				return output.String(), err
			}
			if text != "" {
				encoded, err := r.encode(data, true, before)
				if err != nil {
					return output.String(), err
				}
				output.WriteString(parsed.serialize(encoded))
			}
		}
		r.retained -= len(name) + len(channel.text) + len(channel.template)
		delete(r.channels, name)
	}
	return output.String(), nil
}

func sequenceNumber(data any) (int64, bool) {
	object, ok := data.(map[string]any)
	if !ok {
		return 0, false
	}
	number, ok := object["sequence_number"].(json.Number)
	if !ok {
		return 0, false
	}
	value, err := number.Int64()
	return value, err == nil && value >= 0
}

func (r *sseEventRestorer) encode(data any, synthetic bool, before any) (string, error) {
	object, objectOK := data.(map[string]any)
	value, numbered := sequenceNumber(data)
	if synthetic && numbered {
		if next, ok := sequenceNumber(before); ok {
			value = next
		} else if r.hasSequence {
			value = r.lastSequence + 1
		}
	}
	if objectOK && numbered {
		const maxSequence = int64(^uint64(0) >> 1)
		if value < 0 || value > maxSequence-r.sequenceShift {
			return "", fmt.Errorf("SSE sequence number overflow")
		}
		object["sequence_number"] = json.Number(strconv.FormatInt(value+r.sequenceShift, 10))
		if synthetic {
			r.sequenceShift++
		} else {
			r.lastSequence, r.hasSequence = value, true
		}
	}
	encoded, err := json.Marshal(data)
	return string(encoded), err
}

func clearSyntheticMetadata(data any) {
	object, ok := data.(map[string]any)
	if !ok {
		return
	}
	delete(object, "usage")
	if choices, ok := object["choices"].([]any); ok {
		for _, raw := range choices {
			if choice, ok := raw.(map[string]any); ok {
				delete(choice, "logprobs")
				choice["finish_reason"] = nil
				clearDeltaMetadata(choice["delta"])
			}
		}
	}
}

func clearDeltaMetadata(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch key {
			case "role", "id", "name", "object", "status", "finish_reason", "stop_reason":
				delete(typed, key)
			default:
				clearDeltaMetadata(child)
			}
		}
	case []any:
		for _, child := range typed {
			clearDeltaMetadata(child)
		}
	}
}

func streamErrorClass(data any, eventName string) string {
	object, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	typeName := streamEventType(object, eventName)
	switch typeName {
	case "error":
		return "upstream_stream_error"
	case "response.failed":
		return "upstream_response_failed"
	case "response.incomplete":
		return "upstream_response_incomplete"
	}
	if response, ok := object["response"].(map[string]any); ok {
		switch response["status"] {
		case "failed":
			return "upstream_response_failed"
		case "incomplete":
			return "upstream_response_incomplete"
		}
	}
	return ""
}
