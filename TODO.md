# 待办事项清单

## 🔥 当前优先级 (本周)

### Phase 2: 代理服务
- [ ] 实现 `backend/src/proxy/server.ts`
  - [ ] Fastify 服务器初始化
  - [ ] 代理端口监听
  - [ ] 请求拦截和转发
  - [ ] 错误处理

- [ ] 实现 `backend/src/proxy/protocol.ts`
  - [ ] OpenAI Chat Completions 识别
  - [ ] Anthropic Messages 识别
  - [ ] 通用 JSON 端点处理
  - [ ] Redact Notice 注入

- [ ] 实现 `backend/src/proxy/sse.ts`
  - [ ] SSE 流解析
  - [ ] 流式还原集成
  - [ ] 背压处理

---

## 📋 积压任务

### 核心引擎 - 已完成 ✅
- [x] 实现 `backend/src/engine/redactor.ts`
- [x] 实现 `backend/src/engine/matcher.ts`
- [x] 实现 `backend/src/engine/entropy.ts`
- [x] 实现 `backend/src/engine/restorer.ts`
- [x] 单元测试（redactor, matcher, entropy, restorer）

### 存储层 - 已完成 ✅
- [x] 实现 `backend/src/storage/crypto.ts`
- [x] 实现 `backend/src/storage/memory.ts` (内存版本)
- [x] 实现 `backend/src/storage/mappings.ts` (SQLite 版本)

### 代理服务
- [ ] 实现 `backend/src/proxy/server.ts`
- [ ] 实现 `backend/src/proxy/protocol.ts`
- [ ] 实现 `backend/src/proxy/sse.ts`
- [ ] 集成测试

### 管理 API
- [ ] 实现 `backend/src/api/management.ts`
- [ ] 实现 `backend/src/api/rules.ts`
- [ ] 实现 `backend/src/api/stats.ts`
- [ ] 实现 `backend/src/api/config.ts`
- [ ] API 文档

### 前端
- [ ] 集成 shadcn/ui 组件
- [ ] 替换模拟数据为真实 API
- [ ] 实现图表可视化
- [ ] 实现规则编辑对话框
- [ ] 实现日志详情对话框

---

## 🐛 已知问题

- 无

---

## 💡 功能想法

### 短期
- [ ] 规则试跑功能（Dry Run）
- [ ] 导出/导入规则配置
- [ ] 请求统计图表

### 中期
- [ ] WebSocket 实时日志推送
- [ ] 规则热重载
- [ ] 多语言支持（i18n）

### 长期
- [ ] 插件系统
- [ ] 机器学习增强检测
- [ ] 分布式部署支持

---

## 📝 文档待办

- [ ] API 文档（OpenAPI）
- [ ] 用户手册
- [ ] 开发者指南
- [ ] 部署最佳实践
- [ ] 安全指南

---

## ✅ 已完成

### Phase 0: 项目骨架 (2026-09-12)
- [x] 项目骨架搭建
- [x] TypeScript 配置
- [x] 前后端项目结构
- [x] 内置规则集定义
- [x] 占位符生成器
- [x] Creator 身份识别
- [x] 配置管理模块
- [x] 日志系统
- [x] 前端页面布局
- [x] 深色主题样式
- [x] Docker 配置
- [x] 文档（README, QUICK_START, etc.）

### Phase 1: 核心引擎 (2026-09-12)
- [x] 脱敏引擎 (redactor.ts)
- [x] 匹配器 (matcher.ts) - 正则/字典/熵检测
- [x] 熵检测算法 (entropy.ts) - 交叉熵 + Shannon 熵
- [x] 还原引擎 (restorer.ts) - 流式滑动窗口
- [x] 加密工具 (crypto.ts) - AES-256-GCM + PBKDF2
- [x] 存储层接口 (memory.ts, mappings.ts)
- [x] 单元测试（4 个测试套件，~400 行）

**最后更新**: 2026-09-12
