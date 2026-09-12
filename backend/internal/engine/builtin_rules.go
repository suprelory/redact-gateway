package engine

import (
	"github.com/suprelory/redact-gateway/pkg/types"
)

// GetBuiltinRules 返回所有内置规则（对齐 TypeScript 版本）
func GetBuiltinRules() []*types.Rule {
	rules := []*types.Rule{
		// ============= 私钥类 (Priority 100) =============
		{
			ID:       "pem-private-key",
			Type:     "PRIVATE_KEY",
			Name:     "PEM 私钥",
			Enabled:  true,
			Builtin:  true,
			Priority: 100,
			Method:   types.MethodRegex,
			Pattern:  `-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----[\s\S]+?-----END[^-]*PRIVATE KEY-----`,
		},

		// ============= API Keys (Priority 90) =============
		{
			ID:       "openai-key",
			Type:     "API_KEY",
			Name:     "OpenAI API Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\bsk-[A-Za-z0-9]{20,}`,
		},
		{
			ID:       "anthropic-key",
			Type:     "API_KEY",
			Name:     "Anthropic API Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\bsk-ant-api03-[A-Za-z0-9_-]{93}AA`,
		},
		{
			ID:       "github-token",
			Type:     "API_KEY",
			Name:     "GitHub Token",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36,255}\b`,
		},
		{
			ID:       "github-fine-grained",
			Type:     "API_KEY",
			Name:     "GitHub Fine-grained PAT",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\bgithub_pat_[A-Za-z0-9_]{50,}`,
		},
		{
			ID:       "gcp-api-key",
			Type:     "API_KEY",
			Name:     "Google Cloud API Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\bAIza[0-9A-Za-z_-]{35,38}`,
			AllowList: []string{"AIzaSyabcdefghijklmnopqrstuvwxyz1234567"}, // 示例密钥
		},
		{
			ID:       "aws-access-key",
			Type:     "ACCESS_KEY",
			Name:     "AWS Access Key ID",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\b(?:AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16}\b`,
			AllowList: []string{"AKIAIOSFODNN7EXAMPLE"}, // AWS 示例密钥
		},
		{
			ID:           "aws-secret-key",
			Type:         "ACCESS_KEY",
			Name:         "AWS Secret Access Key",
			Enabled:      true,
			Builtin:      true,
			Priority:     90,
			Method:       types.MethodRegex,
			Pattern:      `(?i)aws[_-]?secret[_-]?access[_-]?key["']?\s*[:=]\s*["']?([A-Za-z0-9/+=]{40})`,
			CaptureGroup: 1,
		},
		{
			ID:       "alibaba-access-key",
			Type:     "ACCESS_KEY",
			Name:     "阿里云 Access Key",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\bLTAI[A-Za-z0-9]{12,20}`,
		},
		{
			ID:       "jwt-token",
			Type:     "JWT",
			Name:     "JWT Token",
			Enabled:  true,
			Builtin:  true,
			Priority: 90,
			Method:   types.MethodRegex,
			Pattern:  `\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`,
		},

		// ============= 连接字符串 (Priority 85) =============
		{
			ID:       "connection-string",
			Type:     "CONNSTR",
			Name:     "数据库连接字符串",
			Enabled:  true,
			Builtin:  true,
			Priority: 85,
			Method:   types.MethodRegex,
			Pattern:  `\b(?:mysql|postgresql|postgres|mongodb|redis|mssql)://[^\s<>"']+`,
		},

		// ============= 中国 PII (Priority 75-80) =============
		{
			ID:       "china-phone",
			Type:     "PHONE",
			Name:     "中国手机号",
			Enabled:  true,
			Builtin:  true,
			Priority: 80,
			Method:   types.MethodRegex,
			Pattern:  `\b1[3-9]\d{9}\b`,
		},
		{
			ID:       "china-id-card",
			Type:     "IDENTITY",
			Name:     "中国身份证号",
			Enabled:  true,
			Builtin:  true,
			Priority: 75,
			Method:   types.MethodRegex,
			Pattern:  `\b\d{17}[\dXx]\b`,
		},
		{
			ID:       "bank-card",
			Type:     "BANK_CARD",
			Name:     "银行卡号",
			Enabled:  true,
			Builtin:  true,
			Priority: 75,
			Method:   types.MethodRegex,
			Pattern:  `\b\d{13,19}\b`,
		},

		// ============= 邮箱 (Priority 70, 默认禁用) =============
		{
			ID:       "email",
			Type:     "EMAIL",
			Name:     "邮箱地址",
			Enabled:  false, // 默认禁用，误报率高
			Builtin:  true,
			Priority: 70,
			Method:   types.MethodRegex,
			Pattern:  `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
		},

		// ============= IP 地址 (Priority 65) =============
		{
			ID:        "ipv4-private",
			Type:      "IPPRIVATE",
			Name:      "内网 IPv4 地址",
			Enabled:   true,
			Builtin:   true,
			Priority:  65,
			Method:    types.MethodRegex,
			Pattern:   `\b(?:10|172\.(?:1[6-9]|2\d|3[01])|192\.168)\.\d{1,3}\.\d{1,3}\b`,
			AllowList: []string{"127.0.0.1", "0.0.0.0", "255.255.255.255"},
		},

		// ============= 高熵检测 (Priority 50) =============
		{
			ID:               "high-entropy",
			Type:             "SECRET",
			Name:             "高熵字符串",
			Enabled:          true,
			Builtin:          true,
			Priority:         50,
			Method:           types.MethodEntropy,
			MinLength:        9,
			EntropyThreshold: 5.2,
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

// FindRuleByID 根据 ID 查找规则
func FindRuleByID(id string) *types.Rule {
	allRules := GetBuiltinRules()
	for _, rule := range allRules {
		if rule.ID == id {
			return rule
		}
	}
	return nil
}
