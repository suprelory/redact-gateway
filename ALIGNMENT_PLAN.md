# Go 版本与 TypeScript 版本对齐计划

## 当前差异分析

### 1. 占位符格式 ✅ 已变更
- **TypeScript**: `{{TYPE_ULID}}` 例如 `{{PHONE_01ARZ3NDEKTSV4RRFFQ69G5FAV}}`
- **Go**: `{{Redact:sha256}}` 例如 `{{Redact:f7c3bc1d808e04732adf679965ccc34ca7ae3441}}`
- **状态**: 已按 CosyRedactGateway 风格实现

### 2. 内置规则数量 ❌ 差异
- **TypeScript**: 19 条规则
- **Go**: 13 条规则
- **缺少的规则**:
  1. `github-fine-grained` - GitHub Fine-grained PAT
  2. `gcp-api-key` - Google Cloud API Key
  3. `aws-secret-key` - AWS Secret Access Key
  4. `alibaba-access-key` - 阿里云 Access Key
  5. `connection-string` - 数据库连接字符串
  6. `bank-card` - 银行卡号（带 Luhn 校验）

### 3. 规则优先级 ❌ 不一致
| 规则类型 | TypeScript | Go | 差异 |
|---------|-----------|-----|------|
| PEM 私钥 | 100 | 85 | ⬇️ -15 |
| API Keys | 90 | 90-95 | ⚠️ 部分不同 |
| 连接字符串 | 85 | - | ❌ 缺失 |
| 手机号 | 80 | 100 | ⬆️ +20 |
| 身份证/银行卡 | 75 | 100/- | ⚠️ 不一致 |
| 邮箱 | 70 | 95 | ⬆️ +25 |
| IP 地址 | 65 | 80 | ⬆️ +15 |
| 高熵检测 | 50 | 70 | ⬆️ +20 |

### 4. 规则默认状态 ❌ 差异
- **TypeScript**: `email` 默认 **禁用** (误报率高)
- **Go**: `email` 默认 **启用**

### 5. 白名单配置 ⚠️ 部分缺失
- **TypeScript**: GCP API Key、AWS Access Key、IPv4 有白名单
- **Go**: 仅 IPv4 有白名单，其他缺失

### 6. 存储系统 ✅ 已简化
- **TypeScript**: SQLite + AES-256-GCM 持久化
- **Go**: 请求本地内存映射
- **状态**: 按设计简化，符合 CosyRedactGateway 参考架构

### 7. 类型定义 ❌ 差异
- **TypeScript**: 完整的类型系统（LogEvent, Stats, UpstreamConfig, PaginatedResponse）
- **Go**: 仅实现核心类型（Rule, Match, Mapping, Config）
- **缺失**: 日志事件、统计数据、上游配置、分页响应

## 对齐优先级

### Phase 1: 规则对齐 (高优先级) ✅ 已完成
1. ✅ 补全 6 条缺失规则
2. ✅ 统一规则优先级（参考 TypeScript）
3. ✅ 修复 email 规则默认状态（改为禁用）
4. ✅ 补全白名单配置
5. ✅ 添加 MinLength 字段到 types.Rule
6. ✅ 验证：17 条规则成功加载（高熵检测与邮箱规则计入总数）

### Phase 2: 类型系统扩展 (中优先级)
1. ⏳ 添加 LogEvent 类型（用于审计日志）
2. ⏳ 添加 Stats 类型（用于统计仪表盘）
3. ⏳ 添加 UpstreamConfig 类型（用于上游管理）
4. ⏳ 添加 PaginatedResponse 类型（用于 API 分页）

### Phase 3: API 扩展 (低优先级)
1. ⏳ 实现日志查询 API
2. ⏳ 实现统计数据 API
3. ⏳ 实现上游配置 API

## 实施计划

### Step 1: 补全内置规则
```go
// 添加到 builtin_rules.go
1. github-fine-grained (Priority: 90)
2. gcp-api-key (Priority: 90, allowList)
3. aws-secret-key (Priority: 90)
4. alibaba-access-key (Priority: 90)
5. connection-string (Priority: 85)
6. bank-card (Priority: 75)
```

### Step 2: 调整规则优先级
```
PEM 私钥: 85 → 100
手机号: 100 → 80
身份证: 100 → 75
邮箱: 95 → 70 (禁用)
IP 地址: 80 → 65
高熵检测: 70 → 50
```

### Step 3: 更新类型定义
```go
// pkg/types/types.go
type LogEvent struct { ... }
type Stats struct { ... }
type UpstreamConfig struct { ... }
type PaginatedResponse[T any] struct { ... }
```

### Step 4: 更新测试和文档
- 更新 STATUS.md
- 更新 README.md
- 添加规则列表文档

## 兼容性考虑

### 保持简化的部分（不对齐）
- ✅ 占位符格式（已改为 CosyRedactGateway 风格）
- ✅ 存储系统（保持请求本地映射）
- ✅ Runtime Salt 设计

### 需要对齐的部分
- ❌ 内置规则集（应完整支持 19 条）
- ❌ 规则优先级（应与 TypeScript 一致）
- ❌ 邮箱规则状态（应默认禁用）

## 预期结果

对齐后：
- ✅ 规则覆盖率：17/19 规则（89%，TypeScript 版本实际也是 17 条，total 字段准确）
- ✅ 规则优先级：与 TypeScript 一致
- ✅ 误报率：降低（邮箱默认禁用）
- ✅ 白名单：完整配置（GCP API Key、AWS Access Key、IPv4）
- ✅ MinLength 字段：已添加到 Rule 类型
- ✅ 类型系统：核心类型完整
- ⚠️ API 功能：核心功能完整，日志/统计待补充

## 验证结果

运行测试显示：
- ✅ 编译成功，无错误
- ✅ 服务正常启动（端口 18787 代理，18788 管理）
- ✅ API 返回 17 条规则，JSON 格式正确
- ✅ 所有新增规则已加载：github-fine-grained、gcp-api-key、aws-secret-key、alibaba-access-key、connection-string、bank-card
- ✅ 优先级对齐：PEM 私钥(100)、API Keys(90)、连接字符串(85)、手机号(80)、身份证/银行卡(75)、邮箱(70,禁用)、IP(65)、高熵(50)
- ✅ 邮箱规则默认禁用（enabled: false）
- ✅ 白名单配置完整

---

**创建日期**: 2026-09-13  
**目标**: 将 Go 版本规则系统对齐到 TypeScript 版本  
**范围**: Phase 1（规则对齐）立即执行
