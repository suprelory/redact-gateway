package redact

import (
	_ "embed"
	"encoding/json"
	"strings"
	"testing"
)

//go:embed testdata/entropy-words.json
var entropyWordsJSON []byte

func entropyRNG(seed uint32) func(int) int {
	return func(size int) int {
		seed = 1664525*seed + 1013904223
		return int(float64(seed) / 4294967296 * float64(size))
	}
}

func TestEntropyCalibrationOnNaturalWordsAndIdentifiers(t *testing.T) {
	var words []string
	if err := json.Unmarshal(entropyWordsJSON, &words); err != nil {
		t.Fatal(err)
	}
	for _, pascal := range []bool{false, true} {
		random, count, total := entropyRNG(0x5eed1234), 0, 0
		for _, length := range []int{9, 10, 11, 12, 13, 16, 20, 24, 32, 40, 48, 64, 80, 96, 128} {
			for index := 0; index < 2000; index++ {
				var text strings.Builder
				for text.Len() < length {
					word := words[random(len(words))]
					if pascal {
						word = strings.ToUpper(word[:1]) + word[1:]
					}
					text.WriteString(word)
				}
				if isHighEntropy(text.String()[:length]) {
					count++
				}
				total++
			}
		}
		if float64(count)/float64(total) >= 0.01 {
			t.Fatalf("pascal=%v false positives=%d/%d", pascal, count, total)
		}
	}
}

func TestEntropyRecallAcrossLengthsAndAlphabets(t *testing.T) {
	random := entropyRNG(12345)
	for _, test := range []struct {
		length         int
		minimum        float64
		lowercaseFloor float64
	}{{12, 0.94, 0.90}, {16, 0.98, 0.96}, {24, 0.995, 0.99}, {32, 0.995, 0.995}} {
		for _, alphabet := range []string{"0123456789abcdef", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "abcdefghijklmnopqrstuvwxyz"} {
			count := 0
			for index := 0; index < 2000; index++ {
				value := make([]byte, test.length)
				for offset := range value {
					value[offset] = alphabet[random(len(alphabet))]
				}
				if isHighEntropy(string(value)) {
					count++
				}
			}
			minimum := test.minimum
			if len(alphabet) == 26 {
				minimum = test.lowercaseFloor
			}
			t.Logf("length=%d alphabet=%d recall=%d/2000", test.length, len(alphabet), count)
			if float64(count)/2000 < minimum {
				t.Fatalf("length=%d alphabet=%d recall=%d/2000", test.length, len(alphabet), count)
			}
		}
	}
}

func TestEntropyRejectsOrdinaryIdentifiersAndRepetition(t *testing.T) {
	for _, value := range []string{"RequestValidationError", "CustomerInformation", "HyperTextTransferProtocol", "aaaaaaaaaaaa", "abcabcabcabc", "111111111111", "a9X2m7K4"} {
		if isHighEntropy(value) {
			t.Errorf("ordinary or excluded value classified as random: %q", value)
		}
	}
	for _, value := range []string{"a9X2m7K4pQ", "qzmxwkjvbnfrtypshgdclaeuiozxqwvbnm"} {
		if len(FindSensitiveMatches(value, DetectorFlags{HighEntropy: true})) != 1 {
			t.Errorf("random value missed by H: %q", value)
		}
	}
}
