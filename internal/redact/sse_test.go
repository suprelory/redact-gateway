package redact

import (
	"fmt"
	"strings"
	"testing"
)

func TestSSERestoresPlaceholderAcrossEveryLogicalSplit(t *testing.T) {
	raw := "alice@example.com"
	for split := 1; split < 48; split++ {
		context := NewContext(10)
		token, err := context.RedactText(raw, DetectorFlags{Email: true})
		if err != nil {
			t.Fatal(err)
		}
		if split >= len(token) {
			break
		}
		input := fmt.Sprintf(
			"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n"+
				"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n"+
				"data: [DONE]\n\n",
			token[:split], token[split:],
		)
		restorer := NewSSEStreamRestorer(context)
		var output strings.Builder
		for index := 0; index < len(input); index++ {
			part, pushErr := restorer.Push([]byte(input[index : index+1]))
			if pushErr != nil {
				t.Fatalf("logical split %d transport byte %d: %v", split, index, pushErr)
			}
			output.Write(part)
		}
		tail, finishErr := restorer.Finish()
		if finishErr != nil {
			t.Fatalf("logical split %d: %v", split, finishErr)
		}
		output.Write(tail)
		if !strings.Contains(output.String(), raw) || strings.Contains(output.String(), token) {
			t.Fatalf("logical split %d not restored:\n%s", split, output.String())
		}
	}
}

func TestSSERestoresToolArgumentsAcrossEvents(t *testing.T) {
	context := NewContext(10)
	token, err := context.RedactText("13800138000", DetectorFlags{Phone: true})
	if err != nil {
		t.Fatal(err)
	}
	split := len(token) / 2
	input := fmt.Sprintf(
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":%q}}]}}]}\n\n"+
			"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":%q}}]}}]}\n\n",
		token[:split], token[split:],
	)
	restorer := NewSSEStreamRestorer(context)
	output, err := restorer.Push([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	tail, err := restorer.Finish()
	if err != nil {
		t.Fatal(err)
	}
	joined := string(output) + string(tail)
	if !strings.Contains(joined, "13800138000") || strings.Contains(joined, token) {
		t.Fatalf("tool arguments not restored: %s", joined)
	}
}

func TestSSEPreservesUnknownPlaceholder(t *testing.T) {
	context := NewContext(10)
	unknown := "{{RG_EMAIL_ABCDEFGHIJKLMNOP}}"
	input := fmt.Sprintf("data: {\"delta\":%q,\"type\":\"response.output_text.delta\"}\n\n", unknown)
	restorer := NewSSEStreamRestorer(context)
	output, err := restorer.Push([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	tail, err := restorer.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output)+string(tail), unknown) {
		t.Fatalf("unknown placeholder changed: %s%s", output, tail)
	}
}
