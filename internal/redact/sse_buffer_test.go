package redact

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func pushTestEvent(t *testing.T, restorer *SSEStreamRestorer, event any) []map[string]any {
	t.Helper()
	out, err := restorer.Push(testSSEEvent(t, event))
	if err != nil {
		t.Fatal(err)
	}
	return parseTestEvents(t, string(out))
}

func TestSSEEmitsPrefixesAndIndependentChannelsImmediately(t *testing.T) {
	ctx := NewContext(10)
	token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
	if err != nil {
		t.Fatal(err)
	}
	r := NewSSEStreamRestorer(ctx)
	delta := func(index int, text string) any {
		return map[string]any{"type": "response.output_text.delta", "output_index": index, "delta": text}
	}
	first := pushTestEvent(t, r, delta(0, "Code: {"))
	if len(first) != 1 || first[0]["delta"] != "Code: " {
		t.Fatalf("safe prefix was delayed: %#v", first)
	}
	other := pushTestEvent(t, r, delta(1, "independent text"))
	if len(other) != 1 || other[0]["delta"] != "independent text" {
		t.Fatalf("independent channel was delayed: %#v", other)
	}
	comment, err := r.Push([]byte(": heartbeat\n\n"))
	if err != nil || string(comment) != ": heartbeat\n\n" {
		t.Fatalf("heartbeat was delayed: %q (%v)", comment, err)
	}
	last := pushTestEvent(t, r, delta(0, token[1:]))
	if len(last) != 1 || last[0]["delta"] != "alice@example.com" {
		t.Fatalf("split token was not restored: %#v", last)
	}
}

func TestSSEFlushesPendingTextBeforeTerminalEvents(t *testing.T) {
	for _, terminal := range []string{"response.output_text.done", "response.completed", "response.incomplete", "response.failed", "error", "DONE", "EOF"} {
		t.Run(terminal, func(t *testing.T) {
			r := NewSSEStreamRestorer(NewContext(10))
			pushTestEvent(t, r, map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": "tail {"})
			var wire []byte
			var err error
			switch terminal {
			case "EOF":
				wire, err = r.Finish()
			case "DONE":
				wire, err = r.Push([]byte("data: [DONE]\n\n"))
			default:
				wire, err = r.Push(testSSEEvent(t, map[string]any{"type": terminal, "output_index": 0}))
			}
			if err != nil {
				t.Fatal(err)
			}
			events := parseTestEvents(t, string(wire))
			if len(events) == 0 || events[0]["type"] != "response.output_text.delta" || events[0]["delta"] != "{" {
				t.Fatalf("pending tail did not precede termination: %s", wire)
			}
			if terminal == "DONE" && !strings.HasSuffix(string(wire), "data: [DONE]\n\n") {
				t.Fatalf("DONE was reordered: %s", wire)
			}
			if tail, err := r.Finish(); err != nil || len(tail) != 0 || len(r.events.channels) != 0 {
				t.Fatalf("tail emitted twice: %s (%v)", tail, err)
			}
		})
	}
}

func TestSSEResponsesSnapshotsCompleteUnsentTokenTails(t *testing.T) {
	for _, terminal := range []string{"response.output_text.done", "response.content_part.done", "response.output_item.done", "response.completed"} {
		t.Run(terminal, func(t *testing.T) {
			ctx := NewContext(10)
			token, err := ctx.RedactText("alice@example.com", DetectorFlags{Email: true})
			if err != nil {
				t.Fatal(err)
			}
			r := NewSSEStreamRestorer(ctx)
			first := pushTestEvent(t, r, map[string]any{"type": "response.output_text.delta", "output_index": 0, "item_id": "msg_1", "content_index": 1, "delta": "Code: " + token[:15]})
			full := "Code: " + token + "!"
			part := map[string]any{"type": "output_text", "text": full}
			item := map[string]any{"id": "msg_1", "content": []any{map[string]any{"text": "other content"}, part}}
			event := map[string]any{"type": terminal, "output_index": 0, "content_index": 1}
			switch terminal {
			case "response.output_text.done":
				event["text"] = full
			case "response.content_part.done":
				event["part"] = part
			case "response.output_item.done":
				event["item"] = item
			case "response.completed":
				event["response"] = map[string]any{"output": []any{item}}
			}
			last := pushTestEvent(t, r, event)
			if len(first) != 1 || len(last) != 2 || first[0]["delta"].(string)+last[0]["delta"].(string) != "Code: alice@example.com!" || last[1]["type"] != terminal {
				t.Fatalf("snapshot left a prefix or duplicated text: %#v / %#v", first, last)
			}
			if ctx.RestoreCount() != 2 || ctx.RestoreUniqueCount() != 1 {
				t.Fatalf("unexpected snapshot accounting: %d/%d", ctx.RestoreCount(), ctx.RestoreUniqueCount())
			}
		})
	}
}

func TestSSEMismatchedSnapshotDoesNotRewriteCommittedText(t *testing.T) {
	r := NewSSEStreamRestorer(NewContext(10))
	pushTestEvent(t, r, map[string]any{"type": "response.output_text.delta", "delta": "sent {"})
	last := pushTestEvent(t, r, map[string]any{"type": "response.output_text.done", "text": "different snapshot"})
	if len(last) != 2 || last[0]["delta"] != "{" || last[1]["text"] != "different snapshot" {
		t.Fatalf("conflicting snapshot changed sent content: %#v", last)
	}
}

func TestSSEFlushDoesNotDuplicateChatMetadataOrOtherText(t *testing.T) {
	r := NewSSEStreamRestorer(NewContext(10))
	first := map[string]any{
		"id": "chat_1", "usage": map[string]any{"total_tokens": 1},
		"choices": []any{map[string]any{"index": 0, "logprobs": map[string]any{"content": []any{}}, "finish_reason": nil,
			"delta": map[string]any{"role": "assistant", "content": "visible text", "tool_calls": []any{
				map[string]any{"index": 0, "id": "call_0", "type": "function", "function": map[string]any{"name": "lookup", "arguments": "{"}},
				map[string]any{"index": 1, "id": "call_1", "type": "function", "function": map[string]any{"name": "save", "arguments": "{"}},
			}}}},
	}
	pushTestEvent(t, r, first)
	last := pushTestEvent(t, r, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}}, "usage": map[string]any{"total_tokens": 2}})
	if len(last) != 3 {
		t.Fatalf("expected two channel tails and terminal event: %#v", last)
	}
	arguments := map[int]string{}
	for _, event := range last[:2] {
		if _, ok := event["usage"]; ok {
			t.Fatal("usage was replayed")
		}
		choice := event["choices"].([]any)[0].(map[string]any)
		if choice["finish_reason"] != nil || choice["logprobs"] != nil {
			t.Fatal("terminal metadata was replayed")
		}
		delta := choice["delta"].(map[string]any)
		if delta["role"] != nil || delta["content"] != "" {
			t.Fatalf("unrelated delta was replayed: %#v", delta)
		}
		for _, raw := range delta["tool_calls"].([]any) {
			call := raw.(map[string]any)
			function := call["function"].(map[string]any)
			if call["id"] != nil || function["name"] != nil {
				t.Fatal("tool identity or name was replayed")
			}
			arguments[int(call["index"].(float64))] += function["arguments"].(string)
		}
	}
	if arguments[0] != "{" || arguments[1] != "{" {
		t.Fatalf("tool tails duplicated: %#v", arguments)
	}
}

func TestSSEFinalDeltaKeepsPrefixBeforeTail(t *testing.T) {
	r := NewSSEStreamRestorer(NewContext(10))
	last := pushTestEvent(t, r, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "final {"}, "finish_reason": "stop"}}})
	if len(last) != 1 {
		t.Fatalf("final text was split around its finish marker: %#v", last)
	}
	choice := last[0]["choices"].([]any)[0].(map[string]any)
	if choice["delta"].(map[string]any)["content"] != "final {" || choice["finish_reason"] != "stop" || len(r.events.channels) != 0 {
		t.Fatalf("final delta order changed: %#v", last)
	}
}

func TestSSESyntheticSequenceNumbersStayOrdered(t *testing.T) {
	ctx := NewContext(10)
	events := restoreTestEvents(t, ctx,
		map[string]any{"type": "response.output_text.delta", "sequence_number": 0, "output_index": 0, "delta": "{"},
		map[string]any{"type": "response.output_text.delta", "sequence_number": 1, "output_index": 1, "delta": "{"},
		map[string]any{"type": "response.completed", "sequence_number": 2},
		map[string]any{"type": "extra", "sequence_number": 3},
	)
	if len(events) != 6 {
		t.Fatalf("unexpected event count: %d", len(events))
	}
	for index, event := range events {
		if event["sequence_number"] != float64(index) {
			t.Fatalf("event %d has wrong sequence: %#v", index, event)
		}
	}
}

func TestSSEResponsesContentAndSummaryChannelsStayIndependent(t *testing.T) {
	ctx := NewContext(10)
	r := NewSSEStreamRestorer(ctx)
	for _, kind := range []string{"response.output_text.delta", "response.reasoning_summary_text.delta"} {
		for index := range 2 {
			field := "content_index"
			if strings.Contains(kind, "summary") {
				field = "summary_index"
			}
			pushTestEvent(t, r, map[string]any{"type": kind, "output_index": 2, field: index, "delta": "{"})
		}
	}
	ended := pushTestEvent(t, r, map[string]any{"type": "response.reasoning_summary_text.done", "output_index": 2, "summary_index": 1, "text": "{"})
	if len(ended) != 2 || ended[0]["type"] != "response.reasoning_summary_text.delta" || ended[0]["summary_index"] != float64(1) || len(r.events.channels) != 3 {
		t.Fatalf("done flushed an unrelated channel: %#v", ended)
	}
	ended = pushTestEvent(t, r, map[string]any{"type": "response.output_item.done", "output_index": 2})
	if len(ended) != 4 || len(r.events.channels) != 0 || r.events.retained != 0 {
		t.Fatalf("item tails were not all released: %#v", ended)
	}
}

func TestSSEAnthropicBlockStopsOnlyFlushTheirOwnTail(t *testing.T) {
	r := NewSSEStreamRestorer(NewContext(10))
	for index := range 2 {
		pushTestEvent(t, r, map[string]any{"type": "content_block_delta", "index": index, "delta": map[string]any{"type": "text_delta", "text": "{"}})
	}
	last := pushTestEvent(t, r, map[string]any{"type": "content_block_stop", "index": 1})
	if len(last) != 2 || last[0]["index"] != float64(1) || last[1]["type"] != "content_block_stop" || len(r.events.channels) != 1 {
		t.Fatalf("block stop mixed channels: %#v", last)
	}
	last = pushTestEvent(t, r, map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}})
	if len(last) != 2 || last[0]["index"] != float64(0) || last[1]["type"] != "message_delta" || len(r.events.channels) != 0 {
		t.Fatalf("message ending lost tail: %#v", last)
	}
}

func TestSSEArgumentDoneCompletesEscapedToken(t *testing.T) {
	ctx := NewContext(10)
	token, err := ctx.tokenFor("SECRET", testPrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	args := `{"key":"` + escapeJSONString(placeholderForms()["triple escaped"](token)) + `"}`
	events := restoreTestEvents(t, ctx,
		map[string]any{"type": "response.function_call_arguments.delta", "output_index": 0, "delta": args[:len(args)/2]},
		map[string]any{"type": "response.function_call_arguments.done", "output_index": 0, "arguments": args},
	)
	var wire strings.Builder
	for _, event := range events {
		if delta, ok := event["delta"].(string); ok {
			wire.WriteString(delta)
		}
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(wire.String()), &decoded); err != nil || decoded["key"] != testPrivateKey {
		t.Fatalf("snapshot completion corrupted arguments: %s (%v)", wire.String(), err)
	}
}

func TestSSELimitsPreserveEarlierOutput(t *testing.T) {
	for _, mode := range []string{"event", "channels", "retained"} {
		t.Run(mode, func(t *testing.T) {
			r := NewSSEStreamRestorerWithLimit(NewContext(10), 512)
			valid := testSSEEvent(t, map[string]any{"type": "response.output_text.delta", "output_index": 0, "delta": "accepted"})
			bad := map[string]any{"type": "response.output_text.delta", "output_index": 1, "delta": "{"}
			switch mode {
			case "event":
				bad["delta"] = strings.Repeat("x", 1024)
			case "channels":
				r.events.maxChannels = 1
			case "retained":
				r.events.maxBufferedBytes = 256
				bad["padding"] = strings.Repeat("x", 200)
			}
			out, err := r.Push(append(valid, testSSEEvent(t, bad)...))
			if !errors.Is(err, ErrSSELimit) || !strings.Contains(string(out), "accepted") || strings.Contains(string(out), "padding") {
				t.Fatalf("limit or earlier output lost: %s (%v)", out, err)
			}
			if len(r.buffer) != 0 || len(r.events.channels) != 0 || r.events.retained != 0 {
				t.Fatal("failed stream retained buffers")
			}
			if out, err := r.Finish(); len(out) != 0 || !errors.Is(err, ErrSSELimit) {
				t.Fatalf("failed stream resumed: %s (%v)", out, err)
			}
		})
	}
}

func TestSSEFramingAcrossEveryTransportByte(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		t.Run(fmt.Sprintf("%q", newline), func(t *testing.T) {
			input := "event: response.output_text.delta" + newline + "data: {\"type\":\"response.output_text.delta\"," + newline + "data: \"delta\":\"中文🙂\"}" + newline + newline + "data: [DONE]" + newline + newline
			r := NewSSEStreamRestorer(NewContext(10))
			var wire strings.Builder
			for index := range len(input) {
				out, err := r.Push([]byte(input[index : index+1]))
				if err != nil {
					t.Fatal(err)
				}
				wire.Write(out)
			}
			tail, err := r.Finish()
			if err != nil {
				t.Fatal(err)
			}
			wire.Write(tail)
			events := parseTestEvents(t, wire.String())
			if len(events) != 1 || events[0]["delta"] != "中文🙂" || strings.Count(wire.String(), "[DONE]") != 1 {
				t.Fatalf("framing corrupted: %s", wire.String())
			}
		})
	}
}

func TestSSEFinishDispatchesLastUnterminatedEventOnlyOnce(t *testing.T) {
	r := NewSSEStreamRestorer(NewContext(10))
	first, err := r.Push([]byte(`data: {"type":"response.output_text.delta","delta":"last {"}`))
	if err != nil || len(first) != 0 {
		t.Fatalf("incomplete event dispatched early: %s (%v)", first, err)
	}
	out, err := r.Finish()
	events := parseTestEvents(t, string(out))
	if err != nil || len(events) != 2 || events[0]["delta"].(string)+events[1]["delta"].(string) != "last {" {
		t.Fatalf("EOF lost text: %s (%v)", out, err)
	}
	if out, err := r.Finish(); err != nil || len(out) != 0 {
		t.Fatalf("EOF repeated: %s (%v)", out, err)
	}
}

func TestSSEDoesNotDropTrailingNonJSONData(t *testing.T) {
	r := NewSSEStreamRestorer(NewContext(10))
	out, err := r.Push([]byte("data: {\"ok\":true} trailing data\n\n"))
	if err != nil || !strings.Contains(string(out), "trailing data") {
		t.Fatalf("non-JSON suffix discarded: %s (%v)", out, err)
	}
}

func TestJSONEncodedPlaceholderVariantsAcrossEveryStreamSplit(t *testing.T) {
	forms := placeholderForms()
	forms["unicode escapes"] = func(token string) string {
		var out strings.Builder
		for _, value := range token {
			fmt.Fprintf(&out, `\u%04x`, value)
		}
		return out.String()
	}
	for name, form := range forms {
		t.Run(name, func(t *testing.T) {
			ctx := NewContext(10)
			raw := "fake \"quoted\" value\\path\nnext line"
			token, err := ctx.tokenFor("EMAIL", raw)
			if err != nil {
				t.Fatal(err)
			}
			encoded := form(token)
			if name != "unicode escapes" {
				encoded = escapeJSONString(encoded)
			}
			args := ` {"value":"` + encoded + `", "path":"C:\\folder", "unchanged":"\u0061", "n":9007199254740993}`
			for split := 1; split < len(args); split++ {
				var parts []any
				for _, part := range []string{args[:split], args[split:]} {
					parts = append(parts, map[string]any{"type": "response.function_call_arguments.delta", "output_index": 0, "delta": part})
				}
				var joined strings.Builder
				for _, event := range restoreTestEvents(t, ctx, parts...) {
					joined.WriteString(event["delta"].(string))
				}
				var decoded map[string]any
				if err := json.Unmarshal([]byte(joined.String()), &decoded); err != nil || decoded["value"] != raw || decoded["path"] != `C:\folder` || !strings.Contains(joined.String(), `"unchanged":"\u0061", "n":9007199254740993`) {
					t.Fatalf("split %d corrupted encoded arguments: %s (%v)", split, joined.String(), err)
				}
			}
		})
	}
}
