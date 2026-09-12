package engine

import (
	"sort"
	"strings"

	"github.com/suprelory/redact-gateway/pkg/types"
)

// RedactOptions 脱敏选项
type RedactOptions struct {
	Rules       []*types.Rule
	RuntimeSalt string
	Creator     string
}

// Redact 执行脱敏
func Redact(text string, opts RedactOptions) *types.RedactResult {
	// 1. 匹配所有规则
	matches := MatchAll(text, opts.Rules)

	if len(matches) == 0 {
		return &types.RedactResult{
			RedactedText:  text,
			Mappings:      []types.Mapping{},
			MatchCount:    0,
			RedactedCount: 0,
			RuleHits:      make(map[string]int),
		}
	}

	// 2. 合并重叠匹配（优先级高的优先）
	merged := mergeOverlaps(matches)

	// 3. 生成占位符并替换
	var mappings []types.Mapping
	ruleHits := make(map[string]int)

	// 按位置排序（从后往前替换，避免索引偏移）
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Start > merged[j].Start
	})

	redactedText := text
	for _, match := range merged {
		placeholder := GeneratePlaceholder(match.Text, opts.RuntimeSalt)

		// 替换文本
		before := redactedText[:match.Start]
		after := redactedText[match.End:]
		redactedText = before + placeholder + after

		// 记录映射
		mappings = append(mappings, types.Mapping{
			Placeholder: placeholder,
			Plaintext:   match.Text,
			RuleType:    match.RuleType,
			Creator:     opts.Creator,
		})

		// 统计规则命中
		ruleHits[match.RuleID]++
	}

	// 反转 mappings 顺序（恢复原始顺序）
	for i, j := 0, len(mappings)-1; i < j; i, j = i+1, j-1 {
		mappings[i], mappings[j] = mappings[j], mappings[i]
	}

	return &types.RedactResult{
		RedactedText:  redactedText,
		Mappings:      mappings,
		MatchCount:    len(matches),
		RedactedCount: len(merged),
		RuleHits:      ruleHits,
	}
}

// mergeOverlaps 合并重叠匹配（优先级高的优先）
func mergeOverlaps(matches []types.Match) []types.Match {
	if len(matches) == 0 {
		return matches
	}

	// 按优先级排序（高优先级优先）
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Priority != matches[j].Priority {
			return matches[i].Priority > matches[j].Priority
		}
		return matches[i].Start < matches[j].Start
	})

	var result []types.Match
	used := make([]bool, len(matches))

	for i, m1 := range matches {
		if used[i] {
			continue
		}

		// 标记与当前匹配重叠的低优先级匹配
		for j := i + 1; j < len(matches); j++ {
			if used[j] {
				continue
			}

			m2 := matches[j]
			if isOverlap(m1, m2) {
				used[j] = true
			}
		}

		result = append(result, m1)
	}

	// 按位置排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Start < result[j].Start
	})

	return result
}

// isOverlap 检查两个匹配是否重叠
func isOverlap(m1, m2 types.Match) bool {
	return !(m1.End <= m2.Start || m2.End <= m1.Start)
}

// InjectRedactNotice 注入 Redact Notice 到消息
func InjectRedactNotice(content string, redactCount int, ruleTypes []string) string {
	if redactCount == 0 {
		return content
	}

	notice := buildRedactNotice(redactCount, ruleTypes)
	return content + "\n\n" + notice
}

// buildRedactNotice 构建 Redact Notice 文本
func buildRedactNotice(count int, ruleTypes []string) string {
	var sb strings.Builder

	sb.WriteString("🔒 **Redact Notice**: ")
	sb.WriteString("此消息已自动脱敏，")
	sb.WriteString("共替换 ")
	sb.WriteString(strings.Join([]string{string(rune(count + '0'))}, ""))
	sb.WriteString(" 处敏感信息")

	if len(ruleTypes) > 0 {
		sb.WriteString("（")
		sb.WriteString(strings.Join(uniqueStrings(ruleTypes), "、"))
		sb.WriteString("）")
	}

	sb.WriteString("。响应将自动还原。")

	return sb.String()
}

// uniqueStrings 去重字符串数组
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
