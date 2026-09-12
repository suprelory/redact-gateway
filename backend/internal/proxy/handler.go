package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/suprelory/redact-gateway/internal/engine"
	"github.com/suprelory/redact-gateway/internal/storage"
	"github.com/suprelory/redact-gateway/pkg/types"
)

// Handler 代理处理器
type Handler struct {
	store *storage.MemoryStore
	cfg   *types.Config
}

// NewHandler 创建代理处理器
func NewHandler(store *storage.MemoryStore, cfg *types.Config) *Handler {
	return &Handler{
		store: store,
		cfg:   cfg,
	}
}

// HandleProxy 处理代理请求
func (h *Handler) HandleProxy(c *fiber.Ctx) error {
	// 1. 解析路由 (/<flags>$<upstream-url>)
	path := c.Path()
	if !strings.Contains(path, "$") {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid route format, expected: /<flags>$<upstream-url>",
		})
	}

	parts := strings.SplitN(path[1:], "$", 2)
	if len(parts) != 2 {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid route format",
		})
	}

	_ = parts[0] // flags - reserved for future use
	upstreamURL := parts[1]

	// 2. 获取 creator (从 Authorization header)
	apiKey := c.Get("Authorization")
	if apiKey == "" {
		return c.Status(401).JSON(fiber.Map{
			"error": "Missing Authorization header",
		})
	}
	creator := storage.GetCreatorHash(apiKey)

	// 3. 创建请求上下文
	reqCtx := h.store.NewRequestContext(creator)
	defer reqCtx.Close()

	// 4. 读取请求体
	bodyBytes := c.Body()

	// 5. 检测协议类型
	protocol := detectProtocol(upstreamURL, bodyBytes)

	// 6. 脱敏请求
	rules := engine.GetEnabledRules()
	redactOpts := engine.RedactOptions{
		Rules:       rules,
		RuntimeSalt: h.store.GetRuntimeSalt(),
		Creator:     creator,
	}

	var redactedBody []byte
	if protocol == "openai-chat" || protocol == "anthropic-messages" {
		redactedBody = h.redactChatRequest(bodyBytes, redactOpts, reqCtx)
	} else {
		// 通用文本脱敏
		redactResult := engine.Redact(string(bodyBytes), redactOpts)
		for _, mapping := range redactResult.Mappings {
			_ = reqCtx.SaveMapping(&mapping)
		}
		redactedBody = []byte(redactResult.RedactedText)
	}

	// 7. 转发请求
	upstreamReq, err := http.NewRequest(c.Method(), upstreamURL, bytes.NewReader(redactedBody))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to create upstream request",
		})
	}

	// 复制 headers（排除 Host 和 Authorization）
	c.Request().Header.VisitAll(func(key, value []byte) {
		keyStr := string(key)
		if keyStr != "Host" && keyStr != "Authorization" {
			upstreamReq.Header.Set(keyStr, string(value))
		}
	})
	upstreamReq.Header.Set("Authorization", apiKey)
	upstreamReq.Header.Set("Content-Length", strconv.Itoa(len(redactedBody)))

	// 8. 发送请求
	client := &http.Client{Timeout: 60 * time.Second}
	upstreamResp, err := client.Do(upstreamReq)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{
			"error": "Upstream request failed",
		})
	}
	defer upstreamResp.Body.Close()

	// 9. 读取响应
	respBody, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{
			"error": "Failed to read upstream response",
		})
	}

	// 10. 还原响应
	mappingTable := make(map[string]string)
	for _, m := range reqCtx.GetAllMappings() {
		mappingTable[m.Placeholder] = m.Plaintext
	}

	restoreOpts := engine.RestoreOptions{
		MappingTable: mappingTable,
	}
	restoreResult := engine.Restore(string(respBody), restoreOpts)

	// 11. 返回响应
	c.Status(upstreamResp.StatusCode)
	for key, values := range upstreamResp.Header {
		for _, value := range values {
			c.Set(key, value)
		}
	}

	return c.Send([]byte(restoreResult.RestoredText))
}

// redactChatRequest 脱敏聊天请求
func (h *Handler) redactChatRequest(bodyBytes []byte, opts engine.RedactOptions, reqCtx *storage.RequestContext) []byte {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		// 解析失败，返回原始内容
		return bodyBytes
	}

	// 脱敏 messages 数组
	if messages, ok := payload["messages"].([]interface{}); ok {
		for i, msg := range messages {
				if msgMap, ok := msg.(map[string]interface{}); ok {
					if content, ok := msgMap["content"].(string); ok {
						result := engine.Redact(content, opts)
						for _, mapping := range result.Mappings {
							_ = reqCtx.SaveMapping(&mapping)
						}

					// 注入 Redact Notice 到最后一个用户消息
					if i == len(messages)-1 && msgMap["role"] == "user" && result.RedactedCount > 0 {
						ruleTypes := make([]string, 0, len(result.RuleHits))
						for ruleID := range result.RuleHits {
							ruleTypes = append(ruleTypes, ruleID)
						}
						msgMap["content"] = engine.InjectRedactNotice(result.RedactedText, result.RedactedCount, ruleTypes)
					} else {
						msgMap["content"] = result.RedactedText
					}
				}
			}
		}
	}

	// 重新编码
	redactedBytes, _ := json.Marshal(payload)
	return redactedBytes
}

// detectProtocol 检测协议类型
func detectProtocol(url string, body []byte) string {
	if strings.Contains(url, "openai.com") || strings.Contains(url, "/v1/chat/completions") {
		return "openai-chat"
	}
	if strings.Contains(url, "anthropic.com") || strings.Contains(url, "/v1/messages") {
		return "anthropic-messages"
	}
	return "generic"
}
