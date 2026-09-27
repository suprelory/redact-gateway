package redact

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrSensitiveNumber = errors.New("sensitive numeric value")

// Reject numeric PII instead of forwarding it or changing its JSON type.
// The error contains a sanitized field path and rule label, never the value.
func validateSensitiveNumber(value any, flags DetectorFlags, fieldPath string) error {
	if !flags.Phone && !flags.Identity && !flags.Bank {
		return nil
	}
	var text string
	switch number := value.(type) {
	case json.Number:
		text = string(number)
	case float64:
		text = strconv.FormatFloat(number, 'g', -1, 64)
	case float32:
		text = strconv.FormatFloat(float64(number), 'g', -1, 32)
	case int:
		text = strconv.FormatInt(int64(number), 10)
	case int64:
		text = strconv.FormatInt(number, 10)
	case int32:
		text = strconv.FormatInt(int64(number), 10)
	case int16:
		text = strconv.FormatInt(int64(number), 10)
	case int8:
		text = strconv.FormatInt(int64(number), 10)
	case uint:
		text = strconv.FormatUint(uint64(number), 10)
	case uint64:
		text = strconv.FormatUint(number, 10)
	case uint32:
		text = strconv.FormatUint(uint64(number), 10)
	case uint16:
		text = strconv.FormatUint(uint64(number), 10)
	case uint8:
		text = strconv.FormatUint(uint64(number), 10)
	default:
		return nil
	}
	digits, ok := sensitiveIntegerDigits(text)
	if !ok {
		return nil
	}
	for _, match := range FindSensitiveMatches(digits, DetectorFlags{Phone: flags.Phone, Identity: flags.Identity, Bank: flags.Bank}) {
		if match.Start == 0 && match.End == len(digits) {
			return fmt.Errorf("%w: %s at %s; send this value as a JSON string", ErrSensitiveNumber, match.Label, fieldPath)
		}
	}
	return nil
}

// Normalize only positive integers of a length relevant to P/I/B. Work on
// decimal digits, so exponent notation cannot bypass detection or lose integer
// precision. Bound the exponent before allocating any padding.
func sensitiveIntegerDigits(text string) (string, bool) {
	if text == "" || text[0] == '-' {
		return "", false
	}
	mantissa, exponent := text, 0
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		mantissa = text[:index]
		var err error
		exponent, err = strconv.Atoi(text[index+1:])
		if err != nil || exponent > len(text)+19 || exponent < -len(text)-19 {
			return "", false
		}
	}
	fraction := 0
	if index := strings.IndexByte(mantissa, '.'); index >= 0 {
		fraction = len(mantissa) - index - 1
		mantissa = mantissa[:index] + mantissa[index+1:]
	}
	for index := range len(mantissa) {
		if !isASCIIDigit(mantissa[index]) {
			return "", false
		}
	}
	digits := strings.TrimLeft(mantissa, "0")
	if digits == "" {
		return "", false
	}
	shift := exponent - fraction
	length := len(digits) + shift
	if length < 11 || length > 19 {
		return "", false
	}
	if shift >= 0 {
		return digits + strings.Repeat("0", shift), true
	}
	for index := length; index < len(digits); index++ {
		if digits[index] != '0' {
			return "", false
		}
	}
	return digits[:length], true
}
