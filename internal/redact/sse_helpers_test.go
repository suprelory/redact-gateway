package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func testSSEEvent(t *testing.T, event any) []byte {
	t.Helper()
	raw, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	return []byte("data: " + string(raw) + "\n\n")
}

func parseTestEvents(t *testing.T, wire string) []map[string]any {
	t.Helper()
	var events []map[string]any
	for _, line := range strings.Split(wire, "\n") {
		if data, ok := strings.CutPrefix(line, "data: "); ok && data != "[DONE]" {
			var event map[string]any
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				t.Fatal(err)
			}
			events = append(events, event)
		}
	}
	return events
}

func restoreTestEvents(t *testing.T, ctx *Context, events ...any) []map[string]any {
	t.Helper()
	restorer := NewSSEStreamRestorer(ctx)
	var wire strings.Builder
	for _, event := range events {
		out, err := restorer.Push(testSSEEvent(t, event))
		if err != nil {
			t.Fatal(err)
		}
		wire.Write(out)
	}
	tail, err := restorer.Finish()
	if err != nil {
		t.Fatal(err)
	}
	wire.Write(tail)
	return parseTestEvents(t, wire.String())
}
