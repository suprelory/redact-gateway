# Redact Gateway

> **本地 AI 隐私脱敏网关 · 请求自动打码 · 响应流式还原 · 零遥测**

**Redact Gateway** 是一个透明的本地代理，在你的 AI 客户端与上游 LLM API 之间自动脱敏敏感信息，保护隐私数据不被外部模型服务商接触。

## 🔄 工作原理

```
你的输入:     排查数据库 mysql://root:Pass123@192.168.1.50:3306/db，联系 13800138000
模型看到:     排查数据库 {{CONNSTR_01ARZ3NDEK}}，联系 {{PHONE_01ARZ3NDEK}}
模型回复:     建议检查 {{CONNSTR_01ARZ3NDEK}} 的连接权限
你最终看到:   建议检查 mysql://root:Pass123@192.168.1.50:3306/db 的连接权限
```

## ✨ 核心特性

- **🛡️ 深度脱敏** - 内置 19 类规则：API Key、PEM 私钥、数据库连接串、手机号、身份证、IP 地址等
- **⚡ 流式还原** - SSE 流式响应毫秒级还原，完全保留原生打字机体验
- **🔌 即插即用** - 只需修改客户端 Base URL，支持 OpenAI/Anthropic 协议
- **🔒 本地加密** - 映射表 AES-256-GCM 加密持久化，主密钥本地管理
- **📊 可视化管理** - Web 界面实时监控、规则配置、流量分析
- **🚀 零依赖部署** - 单文件可执行 / Docker / Tauri 桌面应用

## 🚀 快速开始

### 方式 A：Docker（推荐）

```bash
docker run -d \
  --name redact-gateway \
  -p 127.0.0.1:18787:18787 \
  -p 127.0.0.1:18788:18788 \
  -v redact-data:/data \
  -e MASTER_KEY="your-secret-key-min-32-chars" \
  redact-gateway:latest
```

- 代理端口: `http://127.0.0.1:18787`
- 管理界面: `http://127.0.0.1:18788`

### 方式 B：从源码运行

```bash
# 后端
cd backend
npm install
npm run dev

# 前端（新终端）
cd frontend
npm install
npm run dev
```

## 🔌 客户端接入

### Cursor

Settings → Models → OpenAI Base URL:
```
http://127.0.0.1:18787/v1
```

### Claude Code

```bash
export ANTHROPIC_BASE_URL="http://127.0.0.1:18787"
claude
```

### Python 代码

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:18787/v1",
    api_key="your-api-key"
)
```

## 📁 项目结构

```
redact-gateway/
├── backend/           # Node.js + TypeScript 后端
│   ├── src/
│   │   ├── engine/    # 脱敏/还原引擎
│   │   ├── storage/   # 加密存储
│   │   └── api/       # 管理 API
│   └── package.json
├── frontend/          # React + TypeScript 前端
│   ├── src/
│   │   ├── pages/     # 页面组件
│   │   └── components/
│   └── package.json
└── docker-compose.yml
```

## 🔒 安全特性

- **加密持久化** - AES-256-GCM + HMAC-SHA256 索引
- **Creator 隔离** - 不同 API Key 的映射完全隔离
- **失败关闭** - 无法脱敏时拒绝转发，不泄漏明文
- **零日志泄漏** - 日志中不记录明文、凭据、完整占位符

## 📄 许可证

MIT License

---

**Made with ❤️ for privacy-first AI development**
