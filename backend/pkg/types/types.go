package types

import "time"

// RuleMethod 匹配方法类型
type RuleMethod string

const (
	MethodRegex   RuleMethod = "regex"
	MethodDict    RuleMethod = "dict"
	MethodEntropy RuleMethod = "entropy"
)

// Rule 脱敏规则定义
type Rule struct {
	ID               string     `json:"id"`
	Type             string     `json:"type"`              // PHONE, EMAIL, API_KEY, etc.
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	Enabled          bool       `json:"enabled"`
	Builtin          bool       `json:"builtin"`
	Priority         int        `json:"priority"`          // 100=highest, 50=lowest
	Method           RuleMethod `json:"method"`            // regex, dict, entropy
	Pattern          string     `json:"pattern,omitempty"` // for regex
	CaptureGroup     int        `json:"captureGroup,omitempty"`
	Dictionary       []string   `json:"dictionary,omitempty"`       // for dict
	CaseSensitive    bool       `json:"caseSensitive,omitempty"`    // for dict
	WordBoundary     bool       `json:"wordBoundary,omitempty"`     // for dict
	MinLength        int        `json:"minLength,omitempty"`        // for entropy
	EntropyThreshold float64    `json:"entropyThreshold,omitempty"` // for entropy
	AllowList        []string   `json:"allowList,omitempty"`
}

// Mapping 明文-占位符映射
type Mapping struct {
	Placeholder string    `json:"placeholder"` // {{TYPE_ULID}}
	Plaintext   string    `json:"plaintext"`
	RuleType    string    `json:"ruleType"`
	Creator     string    `json:"creator"` // SHA256(credential)
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

// Match 匹配结果
type Match struct {
	Text     string `json:"text"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	RuleID   string `json:"ruleId"`
	RuleType string `json:"ruleType"`
	Priority int    `json:"priority"`
}

// RedactResult 脱敏结果
type RedactResult struct {
	RedactedText  string         `json:"redactedText"`
	Mappings      []Mapping      `json:"mappings"`
	MatchCount    int            `json:"matchCount"`
	RedactedCount int            `json:"redactedCount"`
	RuleHits      map[string]int `json:"ruleHits"`
}

// RestoreResult 还原结果
type RestoreResult struct {
	RestoredText string `json:"restoredText"`
	RestoreCount int    `json:"restoreCount"`
	SkippedCount int    `json:"skippedCount"`
}

// Config 网关配置
type Config struct {
	ProxyPort      int    `json:"proxyPort"`
	ManagementPort int    `json:"managementPort"`
	MasterKey      string `json:"-"` // 不序列化
	AdminToken     string `json:"-"`
	DataDir        string `json:"dataDir"`
	LogLevel       string `json:"logLevel"`
	MappingTTL     int    `json:"mappingTTL"` // seconds
}

// ProxyRequest 代理请求上下文
type ProxyRequest struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	Creator     string            `json:"creator"`
	UpstreamURL string            `json:"upstreamUrl"`
}

// ProxyResponse 代理响应上下文
type ProxyResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       []byte            `json:"body"`
	IsStream   bool              `json:"isStream"`
}
