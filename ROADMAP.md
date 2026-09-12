# 开发路线图

## 当前状态：项目骨架完成 ✅

**版本**: v0.1.0 (Alpha)  
**完成度**: 30%  
**状态**: 可开始核心开发

---

## Phase 1: 核心引擎实现 (Week 1-2)

### 优先级 P0 - 脱敏引擎

**目标**: 实现文本扫描和敏感信息替换

- [ ] `backend/src/engine/redactor.ts`
  - [ ] `scanText()` - 主扫描函数
  - [ ] `applyRules()` - 应用规则集
  - [ ] `replaceMatches()` - 批量替换
  - [ ] 处理重叠匹配（优先级排序）
  - [ ] 映射表生成和复用

- [ ] `backend/src/engine/matcher.ts`
  - [ ] 正则匹配器 `matchRegex()`
  - [ ] 字典匹配器 `matchDictionary()`
  - [ ] 熵检测匹配器 `matchEntropy()`
  - [ ] Luhn 校验（银行卡）
  - [ ] 身份证校验位验证

- [ ] `backend/src/engine/entropy.ts`
  - [ ] 交叉熵计算 `calculateCrossEntropy()`
  - [ ] 英文字母二元组频率表
  - [ ] length-aware 阈值函数
  - [ ] Shannon 熵辅助检查

**验收标准**:
- 单元测试覆盖率 > 80%
- 能正确识别 19 类内置规则
- 相同明文生成相同占位符
- 性能：1MB 文本 < 100ms

### 优先级 P0 - 还原引擎

**目标**: 实现占位符还原和流式处理

- [ ] `backend/src/engine/restorer.ts`
  - [ ] `restoreText()` - 文本还原
  - [ ] `SlidingWindowRestorer` 类
    - [ ] `process()` - 处理数据块
    - [ ] `flush()` - 清空缓冲区
  - [ ] 跨块占位符拼接
  - [ ] Creator 授权验证

**验收标准**:
- 完整占位符 100% 还原
- 跨块切分占位符正确拼接
- 未授权占位符保持原样
- 性能：1MB 流式数据 < 50ms

---

## Phase 2: 存储层实现 (Week 3)

### 优先级 P1 - 加密存储

- [ ] `backend/src/storage/crypto.ts`
  - [ ] HKDF 密钥派生
  - [ ] AES-256-GCM 加密/解密
  - [ ] HMAC-SHA256 索引生成
  - [ ] 安全随机数生成

- [ ] `backend/src/storage/mappings.ts`
  - [ ] SQLite 数据库初始化
  - [ ] 表结构定义
  - [ ] CRUD 操作
    - [ ] `createMapping()` - 创建映射
    - [ ] `getMapping()` - 查询映射
    - [ ] `findByPlaintext()` - 复用查询
    - [ ] `deleteExpired()` - 清理过期
  - [ ] 索引优化

**验收标准**:
- 映射表加密存储
- 相同明文+creator 复用占位符
- 过期映射自动清理
- 并发安全（SQLite WAL 模式）

---

## Phase 3: 代理服务 (Week 4)

### 优先级 P1 - HTTP 代理

- [ ] `backend/src/proxy/server.ts`
  - [ ] Fastify 服务器启动
  - [ ] 代理端口监听
  - [ ] 请求拦截和转发
  - [ ] 错误处理和恢复

- [ ] `backend/src/proxy/protocol.ts`
  - [ ] OpenAI Chat Completions 识别
  - [ ] OpenAI Responses 识别
  - [ ] Anthropic Messages 识别
  - [ ] 通用 JSON 端点处理
  - [ ] Redact Notice 注入

- [ ] `backend/src/proxy/sse.ts`
  - [ ] SSE 流解析
  - [ ] 流式还原集成
  - [ ] 背压处理
  - [ ] 连接超时管理

**验收标准**:
- 支持 OpenAI/Anthropic 协议
- SSE 流式响应正确还原
- 请求失败不泄漏明文
- 性能：代理延迟 < 10ms

---

## Phase 4: 管理 API (Week 5)

### 优先级 P2 - REST API

- [ ] `backend/src/api/management.ts`
  - [ ] Fastify 路由注册
  - [ ] Admin Token 认证中间件
  - [ ] CORS 配置

- [ ] `backend/src/api/rules.ts`
  - [ ] GET /api/rules - 获取规则列表
  - [ ] POST /api/rules - 创建自定义规则
  - [ ] PUT /api/rules/:id - 更新规则
  - [ ] DELETE /api/rules/:id - 删除规则
  - [ ] POST /api/rules/test - 规则试跑

- [ ] `backend/src/api/stats.ts`
  - [ ] GET /api/stats - 获取统计数据
  - [ ] GET /api/logs - 获取日志
  - [ ] GET /api/logs/:id - 日志详情

- [ ] `backend/src/api/config.ts`
  - [ ] GET /api/config - 获取配置
  - [ ] PUT /api/config - 更新配置
  - [ ] GET /api/status - 健康检查

**验收标准**:
- RESTful API 规范
- 统一的错误响应格式
- Admin Token 保护
- API 文档（OpenAPI）

---

## Phase 5: UI 完善 (Week 6)

### 优先级 P2 - 前端集成

- [ ] 集成真实 API
  - [ ] 替换所有模拟数据
  - [ ] 实时数据刷新
  - [ ] WebSocket 连接（可选）

- [ ] UI 组件补全
  - [ ] shadcn/ui 组件集成
  - [ ] 规则编辑对话框
  - [ ] 日志详情对话框
  - [ ] Toast 提示系统

- [ ] 数据可视化
  - [ ] Recharts 图表集成
  - [ ] 请求趋势折线图
  - [ ] 规则命中饼图
  - [ ] 实时监控仪表盘

- [ ] 交互优化
  - [ ] 加载状态
  - [ ] 错误处理
  - [ ] 表单验证
  - [ ] 快捷键支持

**验收标准**:
- 所有功能可通过 UI 操作
- 响应式设计（移动端友好）
- 无障碍支持（ARIA）
- 用户体验流畅

---

## Phase 6: 测试与文档 (Week 7)

### 优先级 P1 - 测试

- [ ] 单元测试
  - [ ] 引擎模块 > 80% 覆盖率
  - [ ] 存储模块 > 80% 覆盖率
  - [ ] API 模块 > 70% 覆盖率

- [ ] 集成测试
  - [ ] 端到端脱敏/还原流程
  - [ ] 多用户隔离
  - [ ] 并发请求

- [ ] 性能测试
  - [ ] 压力测试（1000 req/s）
  - [ ] 内存占用监控
  - [ ] 流式响应基准

### 优先级 P2 - 文档

- [ ] API 文档
  - [ ] OpenAPI 规范
  - [ ] 接口示例
  - [ ] 错误码说明

- [ ] 用户指南
  - [ ] 安装部署
  - [ ] 客户端配置
  - [ ] 规则自定义
  - [ ] 故障排查

- [ ] 开发文档
  - [ ] 架构设计
  - [ ] 模块接口
  - [ ] 扩展指南

---

## 未来规划 (Post v1.0)

### v1.1 - 高级功能
- [ ] 规则热重载（无需重启）
- [ ] 审计日志导出
- [ ] Prometheus 指标导出
- [ ] 多上游负载均衡

### v1.2 - 企业特性
- [ ] RBAC 权限管理
- [ ] SSO 集成（SAML/OAuth）
- [ ] 审计报告生成
- [ ] 合规模板（GDPR/HIPAA）

### v1.3 - 性能优化
- [ ] Redis 缓存层
- [ ] 规则编译优化
- [ ] 并发处理提升
- [ ] 内存占用优化

### v2.0 - 架构演进
- [ ] 分布式部署支持
- [ ] 插件系统
- [ ] 自定义脱敏算法
- [ ] 机器学习增强检测

---

## 里程碑

| 版本 | 目标 | 预计时间 | 状态 |
|-----|------|---------|------|
| v0.1.0 | 项目骨架 | 2026-09-12 | ✅ 完成 |
| v0.2.0 | 核心引擎 | 2026-09-12 | ✅ 完成 |
| v0.3.0 | 代理服务 | 2026-09-19 | 🚧 进行中 |
| v0.4.0 | 管理 API | 2026-09-26 | ⏳ 计划中 |
| v0.5.0 | UI 完善 | 2026-10-03 | ⏳ 计划中 |
| v0.6.0 | 测试与文档 | 2026-10-10 | ⏳ 计划中 |
| v1.0.0 | 正式发布 | 2026-10-17 | ⏳ 计划中 |

---

**当前焦点**: Phase 2 代理服务实现  
**下一个任务**: 实现 `backend/src/proxy/server.ts`

**最新进展** (2026-09-12):
- ✅ Phase 1 核心引擎完成（~1360 行代码）
- ✅ 4 个引擎模块 + 3 个存储模块
- ✅ 4 个单元测试套件
- ✅ 内存存储实现（开发环境可用）
- ⚠️ SQLite 存储需要原生编译，已完成代码但暂不安装依赖
