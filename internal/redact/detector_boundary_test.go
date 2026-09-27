package redact

import "testing"

func TestStructuredDetectorValidation(t *testing.T) {
	for _, test := range []struct {
		value string
		flags DetectorFlags
		match bool
	}{
		{"+14155552671", DetectorFlags{Phone: true}, true},
		{"+86 138 0013 8000", DetectorFlags{Phone: true}, true},
		{"+1 (415) 555-2671", DetectorFlags{Phone: true}, true},
		{"+123456789012345", DetectorFlags{Phone: true}, true},
		{"+1234567890123456", DetectorFlags{Phone: true}, false},
		{"+123 456 789 012 345 6", DetectorFlags{Phone: true}, false},
		{"9+123456789012", DetectorFlags{Phone: true}, false},
		{"+01234567890", DetectorFlags{Phone: true}, false},
		{"+1234567", DetectorFlags{Phone: true}, false},
		{"0000000000000000", DetectorFlags{Bank: true}, false},
		{"0000 0000 0000 0000", DetectorFlags{Bank: true}, false},
		{"999999999999999999", DetectorFlags{Bank: true}, false},
		{"4111111111111111", DetectorFlags{Bank: true}, true},
		{"4111 1111 1111 1111", DetectorFlags{Bank: true}, true},
		{"4111111111111112", DetectorFlags{Bank: true}, false},
		{"11010519491231002X", DetectorFlags{Identity: true}, true},
		{"11010519491231002x", DetectorFlags{Identity: true}, true},
		{"110105194912310038", DetectorFlags{Identity: true}, true},
		{"110105200002290021", DetectorFlags{Identity: true}, true},
		{"990105194912310023", DetectorFlags{Identity: true}, false},
		{"110000194912310022", DetectorFlags{Identity: true}, false},
		{"110105199902310029", DetectorFlags{Identity: true}, false},
		{"110105210001010023", DetectorFlags{Identity: true}, false},
		{"110105194912310003", DetectorFlags{Identity: true}, false},
	} {
		t.Run(test.value, func(t *testing.T) {
			matches := FindSensitiveMatches(test.value, test.flags)
			if (len(matches) > 0) != test.match {
				t.Fatalf("matches = %#v, want match=%v", matches, test.match)
			}
			if test.match && (len(matches) != 1 || matches[0].Start != 0 || matches[0].End != len(test.value)) {
				t.Fatalf("expected full coverage, got %#v", matches)
			}
		})
	}
}
