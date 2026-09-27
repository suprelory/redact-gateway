package redact

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSensitiveJSONNumbers(t *testing.T) {
	phone := DetectorFlags{Phone: true}
	identity := DetectorFlags{Identity: true}
	bank := DetectorFlags{Bank: true}
	for _, test := range []struct {
		name    string
		value   any
		flags   DetectorFlags
		blocked bool
	}{
		{"phone", json.Number("13800138000"), phone, true},
		{"phone decimal", json.Number("13800138000.00"), phone, true},
		{"phone exponent", json.Number("1.3800138e10"), phone, true},
		{"phone leading fractional zeros", json.Number("0.00013800138e14"), phone, true},
		{"phone shifted zeros", json.Number("1380013800000e-2"), phone, true},
		{"native integer", int64(13800138000), phone, true},
		{"native unsigned", uint64(13800138000), phone, true},
		{"native float", float64(13800138000), phone, true},
		{"numeric identity", json.Number("110105194912310038"), identity, true},
		{"identity exponent", json.Number("1.10105194912310038e17"), identity, true},
		{"card", json.Number("4111111111111111"), bank, true},
		{"card decimal", json.Number("4111111111111111.000"), bank, true},
		{"fraction", json.Number("13800138000.1"), phone, false},
		{"fraction exponent", json.Number("138001380001e-1"), phone, false},
		{"negative", json.Number("-13800138000"), phone, false},
		{"small integer", json.Number("42"), phone, false},
		{"large integer preserved", json.Number("9007199254740993"), phone, false},
		{"huge positive exponent", json.Number("1e99999999999999999999"), phone, false},
		{"huge negative exponent", json.Number("1e-99999999999999999999"), phone, false},
		{"zero", json.Number("0.000e1000000"), bank, false},
		{"repeated digits", json.Number("999999999999999999"), bank, false},
		{"disabled phone", json.Number("13800138000"), DetectorFlags{HighEntropy: true, Gitleaks: true}, false},
		{"different rule", json.Number("13800138000"), bank, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := map[string]any{"data": map[string]any{"value": test.value}}
			out, err := RedactJSON(input, NewContext(10), test.flags)
			if errors.Is(err, ErrSensitiveNumber) != test.blocked {
				t.Fatalf("blocked = %v, want %v: %v", err, test.blocked, err)
			}
			if test.blocked {
				if !strings.Contains(err.Error(), "$.data.value") || !strings.Contains(err.Error(), "JSON string") || strings.Contains(err.Error(), "13800138") || strings.Contains(err.Error(), "1101051949") || strings.Contains(err.Error(), "41111111") {
					t.Fatalf("unsafe or unhelpful rejection: %v", err)
				}
			} else if err != nil || !reflect.DeepEqual(input, out) {
				t.Fatalf("ordinary number changed: %#v / %v", out, err)
			}
		})
	}
}

func TestSensitiveNumbersInSerializedJSON(t *testing.T) {
	for _, text := range []string{
		`{"phone":13800138000}`, `{"phone":1.3800138e10}`, `[{"phone":13800138000.0}]`,
		`{"content":"{\"phone\":13800138000}"}`, `{"content":"{\"password\":\"safe\",\"phone\":13800138000}"}`,
	} {
		_, err := RedactJSON(map[string]any{"content": text}, NewContext(10), DetectorFlags{Phone: true})
		if !errors.Is(err, ErrSensitiveNumber) || !strings.Contains(err.Error(), "$.content") || strings.Contains(err.Error(), "13800138") {
			t.Fatalf("serialized numeric PII was not safely rejected: %v", err)
		}
	}
	ctx := NewContext(10)
	input := ` {"phone":"13800138000", "fraction":13800138000.1, "big":9007199254740993, "scaled":1e999999} `
	out, err := RedactJSON(map[string]any{"content": input}, ctx, DetectorFlags{Phone: true})
	if err != nil {
		t.Fatal(err)
	}
	masked := out.(map[string]any)["content"].(string)
	if !strings.Contains(masked, `"fraction":13800138000.1, "big":9007199254740993, "scaled":1e999999`) || !strings.Contains(masked, "{{RG_PHONE_") {
		t.Fatalf("serialized number lexemes changed: %s", masked)
	}
	if restored := RestoreJSON(out, ctx).(map[string]any)["content"]; restored != input {
		t.Fatalf("serialized round trip changed: %s", restored)
	}
}

func TestSensitiveNumberErrorMasksFieldNames(t *testing.T) {
	_, err := RedactJSON(map[string]any{"alice@example.com": json.Number("13800138000")}, NewContext(10), DetectorFlags{Phone: true})
	if !errors.Is(err, ErrSensitiveNumber) || strings.Contains(err.Error(), "alice@example.com") || !strings.Contains(err.Error(), "<redacted-key>") {
		t.Fatalf("sensitive key leaked in rejection: %v", err)
	}
}
