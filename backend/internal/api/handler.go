package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/suprelory/redact-gateway/internal/engine"
	"github.com/suprelory/redact-gateway/internal/storage"
	"github.com/suprelory/redact-gateway/pkg/types"
)

// Handler API 处理器
type Handler struct {
	store *storage.MemoryStore
	cfg   *types.Config
}

// NewHandler 创建 API 处理器
func NewHandler(store *storage.MemoryStore, cfg *types.Config) *Handler {
	return &Handler{
		store: store,
		cfg:   cfg,
	}
}

// GetStatus 获取网关状态
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "running",
		"version": "1.0.0",
		"runtime": fiber.Map{
			"salt": h.store.GetRuntimeSalt()[:16] + "...",
		},
		"rules": fiber.Map{
			"total":   len(engine.GetBuiltinRules()),
			"enabled": len(engine.GetEnabledRules()),
		},
	})
}

// GetRules 获取规则列表
func (h *Handler) GetRules(c *fiber.Ctx) error {
	rules := engine.GetBuiltinRules()
	return c.JSON(fiber.Map{
		"rules": rules,
		"total": len(rules),
	})
}

// TestRulesRequest 测试规则请求
type TestRulesRequest struct {
	Text  string   `json:"text"`
	Rules []string `json:"rules,omitempty"` // 规则 ID 列表，空则测试所有
}

// TestRules 测试规则
func (h *Handler) TestRules(c *fiber.Ctx) error {
	var req TestRulesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Text == "" {
		return c.Status(400).JSON(fiber.Map{
			"error": "Text is required",
		})
	}

	// 选择规则
	var rules []*types.Rule
	if len(req.Rules) > 0 {
		// 按 ID 筛选
		allRules := engine.GetBuiltinRules()
		ruleMap := make(map[string]*types.Rule)
		for _, r := range allRules {
			ruleMap[r.ID] = r
		}
		for _, id := range req.Rules {
			if rule, ok := ruleMap[id]; ok {
				rules = append(rules, rule)
			}
		}
	} else {
		// 使用所有启用的规则
		rules = engine.GetEnabledRules()
	}

	// 执行脱敏
	result := engine.Redact(req.Text, engine.RedactOptions{
		Rules:       rules,
		RuntimeSalt: h.store.GetRuntimeSalt(),
		Creator:     "test",
	})

	return c.JSON(fiber.Map{
		"redactedText":  result.RedactedText,
		"matchCount":    result.MatchCount,
		"redactedCount": result.RedactedCount,
		"ruleHits":      result.RuleHits,
		"mappings": func() []fiber.Map {
			mappings := make([]fiber.Map, len(result.Mappings))
			for i, m := range result.Mappings {
				mappings[i] = fiber.Map{
					"placeholder": m.Placeholder,
					"plaintext":   m.Plaintext,
					"ruleType":    m.RuleType,
				}
			}
			return mappings
		}(),
	})
}
