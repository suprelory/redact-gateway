package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/suprelory/redact-gateway/pkg/types"
)

// Load 从环境变量加载配置
func Load() (*types.Config, error) {
	cfg := &types.Config{
		ProxyPort:      getEnvInt("PROXY_PORT", 18787),
		ManagementPort: getEnvInt("MANAGEMENT_PORT", 18788),
		DataDir:        getEnv("DATA_DIR", "./data"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		MappingTTL:     getEnvInt("MAPPING_TTL", 7*24*60*60), // 7 days
	}

	// 加载主密钥
	masterKey, err := loadMasterKey(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load master key: %w", err)
	}
	cfg.MasterKey = masterKey

	// 加载管理员 Token
	adminToken := getEnv("ADMIN_TOKEN", "")
	if adminToken == "" {
		return nil, fmt.Errorf("ADMIN_TOKEN is required (min 16 characters)")
	}
	if len(adminToken) < 16 {
		return nil, fmt.Errorf("ADMIN_TOKEN must be at least 16 characters")
	}
	cfg.AdminToken = adminToken

	// 确保数据目录存在
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return cfg, nil
}

// loadMasterKey 从文件或环境变量加载主密钥
func loadMasterKey(dataDir string) (string, error) {
	// 1. 尝试从环境变量读取
	if key := os.Getenv("MASTER_KEY"); key != "" {
		if len(key) < 32 {
			return "", fmt.Errorf("MASTER_KEY must be at least 32 characters")
		}
		return key, nil
	}

	// 2. 尝试从文件读取
	keyPath := filepath.Join(dataDir, ".master.key")
	if data, err := os.ReadFile(keyPath); err == nil {
		key := string(data)
		if len(key) < 32 {
			return "", fmt.Errorf("master key in file must be at least 32 characters")
		}
		return key, nil
	}

	// 3. 生成新密钥并保存
	key, err := generateMasterKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate master key: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create data directory: %w", err)
	}

	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return "", fmt.Errorf("failed to save master key: %w", err)
	}

	fmt.Printf("Generated new master key: %s\n", keyPath)
	return key, nil
}

// generateMasterKey 生成随机主密钥
func generateMasterKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	hash := sha256.Sum256(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// getEnv 获取环境变量，带默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt 获取整数环境变量
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}
