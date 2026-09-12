package engine

import (
	"github.com/suprelory/redact-gateway/pkg/types"
)

// GetBuiltinRules 返回所有内置规则
func GetBuiltinRules() []*types.Rule {
	rules := []*types.Rule{
		// 高优先级规则 (100)
		{
			ID:       "china-id-card",
			Type:     "IDENTITY",
			Name:     "中国身份证号",
			Enabled:  true,
			Builtin:  true,
			Priority: 100,
			Method:   types.MethodRegex,
			Pattern:  `\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
		},
		{
			ID:       "china-phone",
			Type:     "PHONE",
			Name:     "中国手机号",
			Enabled:  true,
			Builtin:  true,
			Priority: 100,
			Method:   types.MethodRegex,
			Pattern:  `\b1[3-9]\d{9}\b`,
		},
		{
			ID:       "email",
			Type:     "EMAIL",
			Name:     "电子邮箱",
			Enabled:  true,
			Builtin:  true,
			Priority: 95,
			Method:   types.MethodRegex,
			Pattern:  `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
		},
		{
			ID:       "credit-card",
			Type:     "BANK",
			Name:     "信用卡号",
			Enabled:  true,
			Builtin:  true,
			Priority: 95,
			Method:   types.MethodRegex,
			Pattern:  `\b(?:4[0-9]{12}(?:[0-9]{3})?|5[1-5][0-9]{14}|3[47][0-9]{13}|6(?:011|5[0-9]{2})[0-9]{12})\b`,
		},

		// API Keys (90-95)
		{
			ID:       "openai-api-key",
			Type:     "API_KEY",
			Name:     "OpenAI API Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 95,
			Method:   types.MethodRegex,
			Pattern:  `\bsk-[a-zA-Z0-9]{20}T3BlbkFJ[a-zA-Z0-9]{20}\b`,
		},
		{
			ID:       "anthropic-api-key",
			Type:     "API_KEY",
			Name:     "Anthropic API Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 95,
			Method:   types.MethodRegex,
			Pattern:  `\bsk-ant-[a-zA-Z0-9\-]{95,}\b`,
		},
		{
			ID:       "aws-access-key",
			Type:     "SECRET",
			Name:     "AWS Access Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\b(AKIA|A3T|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}\b`,
		},
		{
			ID:       "github-token",
			Type:     "SECRET",
			Name:     "GitHub Token",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\bgh[pousr]_[A-Za-z0-9_]{36,}\b`,
		},

		// 密码和密钥 (85)
		{
			ID:       "jwt-token",
			Type:     "SECRET",
			Name:     "JWT Token",
			Enabled:  true,
			Builtin:  true,
			Priority: 85,
			Method:   types.MethodRegex,
			Pattern:  `\beyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`,
		},
		{
			ID:       "private-key-header",
			Type:     "SECRET",
			Name:     "Private Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 85,
			Method:   types.MethodRegex,
			Pattern:  `-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`,
		},

		// IPv4 地址 (80)
		{
			ID:       "ipv4-private",
			Type:     "NETWORK",
			Name:     "私有 IPv4 地址",
			Enabled:  true,
			Builtin:  true,
			Priority: 80,
			Method:   types.MethodRegex,
			Pattern:  `\b(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3})\b`,
		},

		// 高熵检测 (70)
		{
			ID:               "high-entropy",
			Type:             "HIGH_ENTROPY",
			Name:             "高熵字符串",
			Enabled:          true,
			Builtin:          true,
			Priority:         70,
			Method:           types.MethodEntropy,
			EntropyThreshold: 0, // 使用默认阈值
		},
	}

	return rules
}

// GetRulesByType 按类型过滤规则
func GetRulesByType(ruleType string) []*types.Rule {
	allRules := GetBuiltinRules()
	var filtered []*types.Rule

	for _, rule := range allRules {
		if rule.Type == ruleType {
			filtered = append(filtered, rule)
		}
	}

	return filtered
}

// GetEnabledRules 获取启用的规则
func GetEnabledRules() []*types.Rule {
	allRules := GetBuiltinRules()
	var enabled []*types.Rule

	for _, rule := range allRules {
		if rule.Enabled {
			enabled = append(enabled, rule)
		}
	}

	return enabled
}
