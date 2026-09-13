package redact

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type DetectorFlags struct {
	HighEntropy bool
	Phone       bool
	Secret      bool
	Identity    bool
	Bank        bool
	Email       bool
	Gitleaks    bool
}

type Match struct {
	Start    int
	End      int
	Label    string
	Priority int
}

type detector struct {
	label     string
	priority  int
	pattern   *regexp.Regexp
	validator func(string) bool
}

var (
	emailDetector = detector{
		label: "EMAIL", priority: 90,
		pattern: regexp.MustCompile(`[A-Za-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)+`),
	}
	phoneDetector       = detector{label: "PHONE", priority: 88, pattern: regexp.MustCompile(`1[3-9][0-9]{9}`), validator: digitBoundary}
	intlPhoneDetector   = detector{label: "PHONE", priority: 87, pattern: regexp.MustCompile(`\+(?:[0-9][ .()\-]?){7,14}[0-9]`)}
	secretDetector      = detector{label: "APIKEY", priority: 110, pattern: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)}
	identityDetector    = detector{label: "IDCARD", priority: 100, pattern: regexp.MustCompile(`[0-9]{17}[0-9Xx]`), validator: validChinaIDBoundary}
	bankPlainDetector   = detector{label: "CARD", priority: 85, pattern: regexp.MustCompile(`[0-9]{13,19}`), validator: validBankBoundary}
	bankGroupedDetector = detector{label: "CARD", priority: 85, pattern: regexp.MustCompile(`[0-9]{4}(?:[ -][0-9]{4}){2,3}(?:[ -][0-9]{1,3})?`), validator: validBankBoundary}
	protectedDetector   = regexp.MustCompile(`\{\{RG_[A-Z][A-Z0-9]{0,31}_[A-Z2-7]{16}\}\}`)
	highEntropyToken    = regexp.MustCompile(`[A-Za-z0-9]{12,}`)
)

var gitleaksDetectors = []detector{
	{label: "PRIVATEKEY", priority: 130, pattern: regexp.MustCompile(`(?s)-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----.*?-----END (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`)},
	{label: "AWSKEY", priority: 120, pattern: regexp.MustCompile(`\b(?:AKIA|ASIA)[A-Z0-9]{16}\b`)},
	{label: "GITHUB", priority: 120, pattern: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,255}\b`)},
	{label: "GITLAB", priority: 120, pattern: regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`)},
	{label: "JWT", priority: 105, pattern: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\b`)},
	{label: "CONNSTR", priority: 115, pattern: regexp.MustCompile(`(?i)\b(?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqp|mssql)://[^\s"'<>]+`)},
	{label: "BEARER", priority: 95, pattern: regexp.MustCompile(`(?i)\bBearer[ \t]+[A-Za-z0-9._~+/=-]{20,}`)},
}

func FindSensitiveMatches(text string, flags DetectorFlags) []Match {
	protected := regexMatches(text, protectedDetector, "EXISTING", 1000, nil)
	candidates := make([]Match, 0, 16)
	if flags.Secret {
		candidates = append(candidates, collect(text, secretDetector)...)
	}
	if flags.Email {
		candidates = append(candidates, collect(text, emailDetector)...)
	}
	if flags.Identity {
		candidates = append(candidates, collect(text, identityDetector)...)
	}
	if flags.Bank {
		candidates = append(candidates, collect(text, bankPlainDetector)...)
		candidates = append(candidates, collect(text, bankGroupedDetector)...)
	}
	if flags.Phone {
		candidates = append(candidates, collect(text, phoneDetector)...)
		candidates = append(candidates, collect(text, intlPhoneDetector)...)
	}
	if flags.Gitleaks {
		for _, rule := range gitleaksDetectors {
			candidates = append(candidates, collect(text, rule)...)
		}
	}
	if flags.HighEntropy {
		for _, index := range highEntropyToken.FindAllStringIndex(text, -1) {
			value := text[index[0]:index[1]]
			if isHighEntropy(value) {
				candidates = append(candidates, Match{Start: index[0], End: index[1], Label: "ENTROPY", Priority: 10})
			}
		}
	}

	filtered := candidates[:0]
	for _, candidate := range candidates {
		blocked := false
		for _, existing := range protected {
			if overlaps(candidate, existing) {
				blocked = true
				break
			}
		}
		if !blocked {
			filtered = append(filtered, candidate)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority > filtered[j].Priority
		}
		leftLength := filtered[i].End - filtered[i].Start
		rightLength := filtered[j].End - filtered[j].Start
		if leftLength != rightLength {
			return leftLength > rightLength
		}
		return filtered[i].Start < filtered[j].Start
	})

	selected := make([]Match, 0, len(filtered))
	for _, candidate := range filtered {
		conflict := false
		for _, existing := range selected {
			if overlaps(candidate, existing) {
				conflict = true
				break
			}
		}
		if !conflict {
			selected = append(selected, candidate)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Start < selected[j].Start })
	return selected
}

func collect(text string, rule detector) []Match {
	return regexMatches(text, rule.pattern, rule.label, rule.priority, rule.validator)
}

func regexMatches(text string, pattern *regexp.Regexp, label string, priority int, validator func(string) bool) []Match {
	indices := pattern.FindAllStringIndex(text, -1)
	out := make([]Match, 0, len(indices))
	for _, index := range indices {
		value := text[index[0]:index[1]]
		if validator != nil && !validatorWithContext(text, index[0], index[1], validator, value) {
			continue
		}
		out = append(out, Match{Start: index[0], End: index[1], Label: label, Priority: priority})
	}
	return out
}

func validatorWithContext(text string, start, end int, validator func(string) bool, value string) bool {
	if validator == nil {
		return true
	}
	if !validator(value) {
		return false
	}
	if start > 0 && isASCIIDigit(text[start-1]) {
		return false
	}
	if end < len(text) && isASCIIDigit(text[end]) {
		return false
	}
	return true
}

func overlaps(a, b Match) bool {
	return a.Start < b.End && a.End > b.Start
}

func isASCIIDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func digitBoundary(string) bool { return true }

func validChinaIDBoundary(value string) bool {
	if len(value) != 18 {
		return false
	}
	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checks := "10X98765432"
	sum := 0
	for i := 0; i < 17; i++ {
		if !isASCIIDigit(value[i]) {
			return false
		}
		sum += int(value[i]-'0') * weights[i]
	}
	return byte(unicode.ToUpper(rune(value[17]))) == checks[sum%11]
}

func validBankBoundary(value string) bool {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		n := int(digits[i] - '0')
		if double {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return sum%10 == 0
}

func isHighEntropy(value string) bool {
	if len(value) < 12 {
		return false
	}
	allDigits := true
	classes := 0
	hasLower, hasUpper, hasDigit := false, false, false
	counts := map[byte]int{}
	for i := 0; i < len(value); i++ {
		b := value[i]
		counts[b]++
		switch {
		case b >= 'a' && b <= 'z':
			hasLower = true
			allDigits = false
		case b >= 'A' && b <= 'Z':
			hasUpper = true
			allDigits = false
		case b >= '0' && b <= '9':
			hasDigit = true
		}
	}
	if allDigits {
		return false
	}
	for _, present := range []bool{hasLower, hasUpper, hasDigit} {
		if present {
			classes++
		}
	}
	if classes < 2 {
		return false
	}
	entropy := 0.0
	length := float64(len(value))
	for _, count := range counts {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}
	threshold := 3.5
	if len(value) >= 24 {
		threshold = 3.2
	}
	return entropy >= threshold
}
