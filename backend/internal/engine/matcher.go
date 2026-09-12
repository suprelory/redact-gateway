package engine

import (
	"regexp"
	"strings"

	"github.com/suprelory/redact-gateway/pkg/types"
)

// MatchRegex 正则表达式匹配
func MatchRegex(text string, rule *types.Rule) []types.Match {
	if rule.Pattern == "" {
		return nil
	}

	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return nil
	}

	var matches []types.Match
	locs := re.FindAllStringSubmatchIndex(text, -1)

	for _, loc := range locs {
		// 使用捕获组（如果指定）
		groupIdx := rule.CaptureGroup
		if groupIdx < 0 || groupIdx*2+1 >= len(loc) {
			groupIdx = 0
		}

		start := loc[groupIdx*2]
		end := loc[groupIdx*2+1]

		if start < 0 || end < 0 {
			continue
		}

		matchedText := text[start:end]

		matches = append(matches, types.Match{
			Text:     matchedText,
			Start:    start,
			End:      end,
			RuleID:   rule.ID,
			RuleType: rule.Type,
			Priority: rule.Priority,
		})
	}

	return matches
}

// MatchDictionary 字典精确匹配
func MatchDictionary(text string, rule *types.Rule) []types.Match {
	if len(rule.Dictionary) == 0 {
		return nil
	}

	var matches []types.Match
	lowerText := strings.ToLower(text)

	for _, keyword := range rule.Dictionary {
		lowerKeyword := strings.ToLower(keyword)
		startIndex := 0

		for {
			index := strings.Index(lowerText[startIndex:], lowerKeyword)
			if index == -1 {
				break
			}

			index += startIndex

			// 检查单词边界
			beforeOK := index == 0 || !isWordChar(rune(text[index-1]))
			afterIdx := index + len(keyword)
			afterOK := afterIdx >= len(text) || !isWordChar(rune(text[afterIdx]))

			if beforeOK && afterOK {
				matches = append(matches, types.Match{
					Text:     text[index : index+len(keyword)],
					Start:    index,
					End:      index + len(keyword),
					RuleID:   rule.ID,
					RuleType: rule.Type,
					Priority: rule.Priority,
				})
			}

			startIndex = index + 1
		}
	}

	return matches
}

// MatchEntropy 高熵检测匹配
func MatchEntropy(text string, rule *types.Rule) []types.Match {
	blocks := FindHighEntropyBlocks(text)

	// 应用自定义阈值
	if rule.EntropyThreshold > 0 {
		var filtered []types.Match
		for _, match := range blocks {
			entropy := CalculateCrossEntropy(match.Text)
			if entropy > rule.EntropyThreshold {
				match.RuleID = rule.ID
				match.RuleType = rule.Type
				match.Priority = rule.Priority
				filtered = append(filtered, match)
			}
		}
		return filtered
	}

	// 更新规则信息
	for i := range blocks {
		blocks[i].RuleID = rule.ID
		blocks[i].RuleType = rule.Type
		blocks[i].Priority = rule.Priority
	}

	return blocks
}

// Match 统一匹配入口
func Match(text string, rule *types.Rule) []types.Match {
	if !rule.Enabled {
		return nil
	}

	var matches []types.Match

	switch rule.Method {
	case types.MethodRegex:
		matches = MatchRegex(text, rule)
	case types.MethodDict:
		matches = MatchDictionary(text, rule)
	case types.MethodEntropy:
		matches = MatchEntropy(text, rule)
	default:
		return nil
	}

	// 应用白名单过滤
	if len(rule.AllowList) > 0 {
		allowSet := make(map[string]bool)
		for _, item := range rule.AllowList {
			allowSet[strings.ToLower(item)] = true
		}

		var filtered []types.Match
		for _, m := range matches {
			if !allowSet[strings.ToLower(m.Text)] {
				filtered = append(filtered, m)
			}
		}
		matches = filtered
	}

	return matches
}

// MatchAll 批量匹配多个规则
func MatchAll(text string, rules []*types.Rule) []types.Match {
	var allMatches []types.Match

	for _, rule := range rules {
		if rule.Enabled {
			matches := Match(text, rule)
			allMatches = append(allMatches, matches...)
		}
	}

	return allMatches
}

// isWordChar 检查是否为单词字符
func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
