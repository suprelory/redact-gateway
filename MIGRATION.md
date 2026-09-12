# 从 Node.js 迁移到 Go

## 变更原因

根据用户需求，后端从 Node.js/TypeScript 切换到 Go 语言实现，主要考虑：
- 更好的性能（编译型语言，原生并发）
- 更低的内存占用
- 更简单的部署（单二进制文件）
- 用户对 Go 语言的偏好

前端保持不变（React + TypeScript）。

## 核心设计参考

后端实现参考 **CosyRedactGateway** (D:\Projects\CosyRedactGateway\worker.js)：

### 占位符格式

从 TypeScript 版本的 `{{TYPE_ULID}}` 改为 CosyRedactGateway 风格的 `{{Redact:sha256}}`：

```
原格式: {{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}
新格式: {{Redact:f7c3bc1d808e04732adf679965ccc34ca7ae3441}}
```

**优势**:
- 更短（75 字节 vs 原 ~50 字节，但固定长度）
- 基于内容哈希（SHA256）而非随机 ULID
- 相同明文 + 相同 runtime salt → 相同占位符
- 参考项目已验证的格式

### 熵检测算法

采用 CosyRedactGateway 的**交叉熵算法**：
- 基于英文字母二元组频率表（BIGRAM_COST）
- 长度感知阈值（9-128 字符，阈值从 5.424 到 4.566）
- Shannon 熵辅助检查（排除低熵重复字符串）
- 自然英文误报率 ~0.99%，随机字符串召回率 >99%（长度 16+）

### Runtime Salt

每次服务启动生成随机 salt：
- 进程内所有请求共享同一 salt
- 相同明文在进程生命周期内生成相同占位符
- 重启后 salt 改变，旧占位符失效（无法还原）

### 映射表生命周期

参考 CosyRedactGateway 的设计，**请求本地映射表**：
- 每个请求创建独立的映射表
- 请求完成后立即丢弃
- 不跨请求持久化
- 简化实现，避免存储开销

**与原 TypeScript 版本的差异**:
- TypeScript 版本支持加密持久化存储（SQLite + AES-256-GCM）
- Go 版本采用请求本地内存映射（与参考项目一致）
- 如需持久化，可后续添加（已有 storage 包接口）

## 技术栈对比

| 组件 | TypeScript 版本 | Go 版本 |
|------|----------------|---------|
| 运行时 | Node.js 20+ | Go 1.22+ |
| Web 框架 | Fastify | Fiber v2 |
| 存储 | better-sqlite3 (可选) | 内存映射 (简化) |
| 加密 | crypto (Node.js) | crypto/sha256 |
| 唯一 ID | ulid | SHA256 哈希 |
| 并发 | 异步 I/O | goroutine |
| 部署 | Node + npm | 单二进制文件 |

## 实现进度

### ✅ 已完成

- [x] 项目结构搭建
- [x] Go modules 配置
- [x] 类型系统 (types.go)
- [x] 配置管理 (config.go)
- [x] 熵检测算法 (entropy.go)
- [x] 规则匹配器 (matcher.go)
- [x] 主入口 (main.go)

### 🚧 进行中

- [ ] 脱敏引擎 (redactor.go)
- [ ] 还原引擎 (restorer.go)
- [ ] 内置规则集 (builtin_rules.go)
- [ ] 代理处理器 (proxy/handler.go)
- [ ] SSE 流式处理 (proxy/sse.go)
- [ ] 管理 API (api/handler.go)

### ⏳ 待实现

- [ ] 单元测试
- [ ] 性能基准测试
- [ ] Docker 镜像更新
- [ ] 文档更新

## 保留功能

以下功能保留 TypeScript 版本的设计：

1. **规则系统** - 19 种内置规则（API Key、手机、邮箱等）
2. **协议支持** - OpenAI/Anthropic 端点识别
3. **Redact Notice** - 注入提示信息到用户消息
4. **Creator 隔离** - SHA256(API Key) 作为身份标识
5. **前端 UI** - React 应用完全不变

## 简化设计

参考 CosyRedactGateway 的简洁设计：

1. **移除持久化存储** - 映射表仅存在于请求生命周期
2. **移除 ULID 依赖** - 使用 SHA256 哈希作为占位符标识
3. **移除复杂加密** - 不再需要 AES-256-GCM 加密存储
4. **简化配置** - 减少可配置项，专注核心功能

## 性能预期

Go 版本相比 Node.js 版本预期改进：
- **启动时间**: <50ms (vs Node.js ~500ms)
- **内存占用**: ~20MB (vs Node.js ~80MB)
- **请求延迟**: <5ms (vs Node.js ~10ms)
- **并发性能**: 10000+ req/s (vs Node.js ~3000 req/s)

## 迁移清单

### 开发者

- [x] 安装 Go 1.22+
- [x] 配置 go.mod
- [ ] 运行 `go mod download`
- [ ] 完成核心模块实现
- [ ] 编写单元测试
- [ ] 更新文档

### 部署

- [ ] 构建二进制: `go build -o gateway ./cmd/gateway`
- [ ] 更新 Dockerfile (使用 multi-stage build)
- [ ] 更新 docker-compose.yml
- [ ] 配置环境变量
- [ ] 测试启动流程

## 兼容性

### 前端

前端 API 接口保持兼容，无需修改代码。

### 配置

环境变量保持一致：
- `PROXY_PORT`
- `MANAGEMENT_PORT`
- `MASTER_KEY`
- `ADMIN_TOKEN`
- `DATA_DIR`

---

**迁移日期**: 2026-09-13  
**原因**: 用户需求，切换到 Go 语言  
**参考项目**: CosyRedactGateway
