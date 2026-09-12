# 贡献指南

感谢你对 Redact Gateway 项目的兴趣！

## 开发环境设置

### 前置要求

- Node.js 20+
- npm 或 pnpm
- Git
- VSCode（推荐）

### 克隆项目

```bash
git clone https://github.com/your-org/redact-gateway.git
cd redact-gateway
```

### 安装依赖

```bash
# 后端
cd backend
npm install

# 前端
cd frontend
npm install
```

### 配置环境

```bash
cd backend
cp .env.example .env
# 编辑 .env 文件
```

### VSCode 推荐扩展

项目已配置推荐扩展，VSCode 会自动提示安装：
- Prettier - Code formatter
- ESLint
- Tailwind CSS IntelliSense
- TypeScript and JavaScript Language Features

## 开发流程

### 分支策略

- `main` - 生产分支
- `develop` - 开发分支
- `feature/*` - 功能分支
- `fix/*` - 修复分支

### 提交规范

遵循 Conventional Commits 规范：

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Type:**
- `feat` - 新功能
- `fix` - 修复缺陷
- `docs` - 文档改动
- `style` - 代码格式化
- `refactor` - 重构
- `perf` - 性能优化
- `test` - 测试
- `chore` - 杂项

**示例:**
```
feat(engine): add entropy detection rule
fix(api): handle null creator in mapping
docs(readme): update installation steps
```

### 开发步骤

1. 创建功能分支
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. 进行开发

3. 运行测试和检查
   ```bash
   npm run typecheck
   npm run lint
   npm run test
   ```

4. 提交代码
   ```bash
   git add .
   git commit -m "feat(scope): description"
   ```

5. 推送并创建 PR
   ```bash
   git push origin feature/your-feature-name
   ```

## 代码规范

### TypeScript

- 使用严格模式
- 显式类型注解
- 避免 `any` 类型
- 使用接口定义对象结构

### 命名约定

- 文件名：`kebab-case.ts`
- 类名：`PascalCase`
- 函数/变量：`camelCase`
- 常量：`UPPER_SNAKE_CASE`
- 接口：`PascalCase`（不加 `I` 前缀）

### 注释

- 使用 JSDoc 为公共 API 添加注释
- 复杂逻辑添加内联注释
- 中文注释优先（面向中文用户）

### 目录结构

```
backend/src/
├── engine/        # 脱敏引擎
├── storage/       # 存储层
├── proxy/         # 代理服务
├── api/           # API 层
└── utils/         # 工具函数

frontend/src/
├── pages/         # 页面组件
├── components/    # 可复用组件
├── lib/           # 工具库
└── styles/        # 样式文件
```

## 测试

### 单元测试

```bash
cd backend
npm run test

cd frontend
npm run test
```

### 集成测试

```bash
npm run test:integration
```

### E2E 测试

```bash
npm run test:e2e
```

## 文档

- 新功能必须更新 README.md
- API 变更必须更新 API 文档
- 重大变更必须更新 CHANGELOG.md

## Pull Request

### PR 标题

遵循 Conventional Commits 格式：
```
feat(engine): add new redaction rule
```

### PR 描述模板

```markdown
## 变更说明

简要描述这个 PR 做了什么。

## 变更类型

- [ ] 新功能
- [ ] Bug 修复
- [ ] 文档更新
- [ ] 性能优化
- [ ] 重构
- [ ] 测试

## 测试

- [ ] 添加了单元测试
- [ ] 添加了集成测试
- [ ] 手动测试通过

## 检查清单

- [ ] 代码遵循项目规范
- [ ] 通过了所有测试
- [ ] 更新了相关文档
- [ ] 没有引入新的警告
- [ ] Commit 消息符合规范
```

## 发布流程

1. 更新版本号（`package.json`）
2. 更新 `CHANGELOG.md`
3. 创建 git tag
4. 推送 tag 触发 CI/CD

## 问题反馈

- 使用 GitHub Issues
- 提供完整的复现步骤
- 附上日志和截图
- 说明环境信息

## 安全问题

如果发现安全漏洞，请**不要**公开提 Issue，而是：
- 发送邮件到 security@example.com
- 或使用 GitHub Security Advisory

## 行为准则

- 尊重所有贡献者
- 友好和包容
- 接受建设性批评
- 专注于对项目最好的事情

## 许可证

提交代码即表示同意将代码以 MIT 许可证发布。

---

感谢你的贡献！🎉
