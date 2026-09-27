package redact

import (
	"regexp"
	"sort"
	"strings"
	"time"
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
	intlPhoneDetector   = detector{label: "PHONE", priority: 87, pattern: regexp.MustCompile(`\+(?:[0-9][ .()\-]{0,2}){7,}[0-9]`), validator: validInternationalPhone}
	secretDetector      = detector{label: "APIKEY", priority: 110, pattern: regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)}
	identityDetector    = detector{label: "IDCARD", priority: 100, pattern: regexp.MustCompile(`[0-9]{17}[0-9Xx]`), validator: validChinaIDBoundary}
	bankPlainDetector   = detector{label: "CARD", priority: 85, pattern: regexp.MustCompile(`[0-9]{13,19}`), validator: validBankBoundary}
	bankGroupedDetector = detector{label: "CARD", priority: 85, pattern: regexp.MustCompile(`[0-9]{4}(?:[ -][0-9]{4}){2,3}(?:[ -][0-9]{1,3})?`), validator: validBankBoundary}
	highEntropyToken    = regexp.MustCompile(`[A-Za-z0-9]{9,}`)
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
	return findSensitiveMatches(text, flags, "")
}

func findSensitiveMatches(text string, flags DetectorFlags, fieldName string) []Match {
	var protected []Match
	for _, candidate := range placeholderCandidates(text) {
		protected = append(protected, Match{Start: candidate.start, End: candidate.end})
	}
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
		if isCredentialField(fieldName) && strings.TrimSpace(text) != "" {
			candidates = append(candidates, Match{Start: 0, End: len(text), Label: "CREDENTIAL", Priority: 80})
		}
		for _, rule := range gitleaksDetectors {
			candidates = append(candidates, collect(text, rule)...)
		}
		candidates = append(candidates, collectCredentials(text)...)
	}
	if flags.HighEntropy {
		for _, index := range highEntropyToken.FindAllStringIndex(text, -1) {
			value := text[index[0]:index[1]]
			if isHighEntropy(value) {
				candidates = append(candidates, Match{Start: index[0], End: index[1], Label: "ENTROPY", Priority: 10})
			}
		}
	}

	return excludeProtectedMatches(mergeMatches(candidates), protected)
}

// mergeMatches preserves the union of every detector's coverage. Priorities
// choose a label only among rules covering the whole union; a smaller match
// must never expose the remainder of a larger credential.
func mergeMatches(matches []Match) []Match {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Start != matches[j].Start {
			return matches[i].Start < matches[j].Start
		}
		if matches[i].End != matches[j].End {
			return matches[i].End > matches[j].End
		}
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return matches[i].Label < matches[j].Label
	})
	merged := make([]Match, 0, len(matches))
	for first := 0; first < len(matches); {
		end, next := matches[first].End, first+1
		for next < len(matches) && matches[next].Start < end {
			end = max(end, matches[next].End)
			next++
		}
		match := matches[first]
		if match.End != end {
			// No single rule explains the entire connected interval.
			match.Label = "SECRET"
			match.End = end
		}
		merged = append(merged, match)
		first = next
	}
	return merged
}

// Existing placeholders stay intact, while sensitive text on either side is
// still masked. Both inputs are ordered and non-overlapping.
func excludeProtectedMatches(matches, protected []Match) []Match {
	if len(protected) == 0 {
		return matches
	}
	out := make([]Match, 0, len(matches))
	first := 0
	for _, match := range matches {
		for first < len(protected) && protected[first].End <= match.Start {
			first++
		}
		for index := first; index < len(protected) && protected[index].Start < match.End; index++ {
			if match.Start < protected[index].Start {
				part := match
				part.End = protected[index].Start
				out = append(out, part)
			}
			match.Start = max(match.Start, protected[index].End)
		}
		if match.Start < match.End {
			out = append(out, match)
		}
	}
	return out
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

func validInternationalPhone(value string) bool {
	digits := 0
	for index := 1; index < len(value); index++ {
		if isASCIIDigit(value[index]) {
			digits++
		}
	}
	return len(value) > 1 && value[1] != '0' && digits >= 8 && digits <= 15
}

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
	// Check broad province-level prefixes without maintaining a brittle list of
	// county codes, which change over time and include historical assignments.
	switch value[:2] {
	case "11", "12", "13", "14", "15", "21", "22", "23", "31", "32", "33", "34", "35", "36", "37",
		"41", "42", "43", "44", "45", "46", "50", "51", "52", "53", "54", "61", "62", "63", "64", "65", "71", "81", "82", "91":
	default:
		return false
	}
	if value[2:6] == "0000" || value[14:17] == "000" {
		return false
	}
	birthDate, err := time.Parse("20060102", value[6:14])
	if err != nil || birthDate.Year() < 1800 || value[6:14] > time.Now().Format("20060102") {
		return false
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
	if strings.Count(digits, digits[:1]) == len(digits) {
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
