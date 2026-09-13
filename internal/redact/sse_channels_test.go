package redact

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestSSEParallelToolChannels(t *testing.T) {
	for _, mode := range []string{"sparse", "reordered", "multiple_choices", "missing_index"} {
		t.Run(mode, func(t *testing.T) {
			ctx := NewContext(10)
			tokens := make([]string, 2)
			originals := []string{"alice@example.com", "bob@example.com"}
			for index, original := range originals {
				var err error
				tokens[index], err = ctx.RedactText(original, DetectorFlags{Email: true})
				if err != nil {
					t.Fatal(err)
				}
			}
			call := func(index int, text string) map[string]any {
				out := map[string]any{"function": map[string]any{"arguments": text}}
				if mode != "missing_index" {
					out["index"] = index
				}
				return out
			}
			choice := func(index int, calls ...any) map[string]any {
				return map[string]any{"index": index, "delta": map[string]any{"tool_calls": calls}}
			}
			event := func(choices ...any) any { return map[string]any{"choices": choices} }
			first := func(i int) string { return `{"email":"` + tokens[i][:12] }
			last := func(i int) string { return tokens[i][12:] + `"}` }
			var input []any
			want := map[string]string{"4:0": originals[0], "4:1": originals[1]}
			switch mode {
			case "sparse":
				input = []any{event(choice(4, call(0, first(0)))), event(choice(4, call(1, first(1)))), event(choice(4, call(0, last(0)))), event(choice(4, call(1, last(1))))}
			case "reordered":
				input = []any{event(choice(4, call(0, first(0)), call(1, first(1)))), event(choice(4, call(1, last(1)), call(0, last(0))))}
			case "multiple_choices":
				input = []any{event(choice(4, call(0, first(0)))), event(choice(9, call(0, first(1)))), event(choice(9, call(0, last(1))), choice(4, call(0, last(0))))}
				want = map[string]string{"4:0": originals[0], "9:0": originals[1]}
			case "missing_index":
				input = []any{event(choice(4, call(0, first(0)), call(1, first(1)))), event(choice(4, call(0, last(0)), call(1, last(1))))}
			}
			arguments := map[string]string{}
			for _, output := range restoreTestEvents(t, ctx, input...) {
				for _, rawChoice := range output["choices"].([]any) {
					c := rawChoice.(map[string]any)
					for position, rawCall := range c["delta"].(map[string]any)["tool_calls"].([]any) {
						call := rawCall.(map[string]any)
						index := position
						if number, ok := call["index"].(float64); ok {
							index = int(number)
						}
						key := fmt.Sprintf("%d:%d", int(c["index"].(float64)), index)
						arguments[key] += call["function"].(map[string]any)["arguments"].(string)
					}
				}
			}
			for key, original := range want {
				var parsed map[string]string
				if err := json.Unmarshal([]byte(arguments[key]), &parsed); err != nil || parsed["email"] != original {
					t.Errorf("channel %s corrupted: %q (%v)", key, arguments[key], err)
				}
			}
			if len(arguments) != len(want) || ctx.RestoreCount() != 2 {
				t.Fatalf("unexpected channels or restoration count: %#v / %d", arguments, ctx.RestoreCount())
			}
		})
	}
}
