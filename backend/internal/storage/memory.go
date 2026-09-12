package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"

	"github.com/suprelory/redact-gateway/pkg/types"
)

// MemoryStore 请求本地内存映射存储
type MemoryStore struct {
	masterKey   string
	runtimeSalt string
	mu          sync.RWMutex
}

// NewMemoryStore 创建内存存储实例
func NewMemoryStore(masterKey string) *MemoryStore {
	return &MemoryStore{
		masterKey:   masterKey,
		runtimeSalt: generateRuntimeSalt(),
	}
}

// generateRuntimeSalt 生成随机 runtime salt
func generateRuntimeSalt() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate runtime salt: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

// GetRuntimeSalt 获取当前 runtime salt
func (s *MemoryStore) GetRuntimeSalt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.runtimeSalt
}

// RequestContext 请求上下文（生命周期仅限当前请求）
type RequestContext struct {
	store    *MemoryStore
	mappings map[string]*types.Mapping
	creator  string
	mu       sync.RWMutex
}

// NewRequestContext 创建请求上下文
func (s *MemoryStore) NewRequestContext(creator string) *RequestContext {
	return &RequestContext{
		store:    s,
		mappings: make(map[string]*types.Mapping),
		creator:  creator,
	}
}

// SaveMapping 保存映射（仅在当前请求上下文）
func (rc *RequestContext) SaveMapping(mapping *types.Mapping) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// 设置创建者和时间戳
	mapping.Creator = rc.creator
	mapping.CreatedAt = time.Now()
	mapping.ExpiresAt = time.Now().Add(24 * time.Hour) // 形式上的过期时间

	rc.mappings[mapping.Placeholder] = mapping
	return nil
}

// GetMapping 获取映射（仅从当前请求上下文）
func (rc *RequestContext) GetMapping(placeholder string) (*types.Mapping, error) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if mapping, ok := rc.mappings[placeholder]; ok {
		return mapping, nil
	}
	return nil, nil
}

// GetAllMappings 获取所有映射（仅从当前请求上下文）
func (rc *RequestContext) GetAllMappings() []*types.Mapping {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	result := make([]*types.Mapping, 0, len(rc.mappings))
	for _, m := range rc.mappings {
		result = append(result, m)
	}
	return result
}

// GetCreatorHash 计算 creator hash (SHA256)
func GetCreatorHash(credential string) string {
	h := sha256.New()
	h.Write([]byte(credential))
	return hex.EncodeToString(h.Sum(nil))
}

// Close 关闭请求上下文（清理映射）
func (rc *RequestContext) Close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.mappings = nil
}
