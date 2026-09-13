# Redact Gateway

一个面向 OpenAI、Anthropic 和兼容 LLM API 的本地脱敏网关。客户端在请求 URL 中携带真实上游地址，网关在请求出站前替换敏感数据，并在普通响应或 SSE 流中恢复已知占位符。

## 路由格式

```text
http://127.0.0.1:8787/<flags>$<upstream-url>
```

空 flags 表示启用全部规则：

```text
http://127.0.0.1:8787/$https://api.openai.com/v1
```

常用规则：

| Flag | 检测内容 |
|---|---|
| `H` | 高熵字符串 |
| `P` | 电话号码 |
| `S` | `sk-` 密钥 |
| `I` | 中国身份证 |
| `B` | 银行卡 |
| `E` | 邮箱 |
| `G` | 私钥、JWT、云密钥、Git Token 和连接串 |

OpenAI Python SDK：

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/HPSE$https://api.openai.com/v1",
    api_key="sk-your-real-key",
)
```

Anthropic/Claude Code：

```powershell
$env:ANTHROPIC_BASE_URL = 'http://127.0.0.1:8787/PSE$https://api.anthropic.com'
$env:ANTHROPIC_API_KEY = 'your-real-key'
```

在 PowerShell 或 Bash 中直接书写代理 URL 时应使用单引号，避免 `$` 被 shell 当作变量前缀展开。

Authorization 和厂商 API Key 头只会转发给 URL 中指定的上游，不写入事件库。日志仅记录上游域名、路径、状态、耗时和规则命中数量，不记录请求正文、响应正文或查询参数。

## 本地运行

前置要求：Go 1.24+、Node.js 24+。

```powershell
cd web
npm install
npm run build
cd ..

go run ./cmd/redact-gateway serve
```

默认监听：

```text
数据面：http://127.0.0.1:8787
控制面：http://127.0.0.1:8788
```

首次启动会在日志中输出管理令牌，并保存到本地数据目录的 `admin-token` 文件。

## Docker Compose

```bash
cp .env.example .env
# 修改 .env 中的镜像名称和 REDACT_ADMIN_TOKEN
docker compose up -d --build
```

更新镜像：

```bash
docker compose pull
docker compose up -d
```

SQLite 数据保存在 `redact_gateway_data` 命名卷中。

## 安全设置

默认只允许访问公网 HTTP/HTTPS 地址，并阻止环回、私网、链路本地和其他特殊用途 IP。需要访问公司内网上游时显式设置：

```text
REDACT_ALLOW_PRIVATE_UPSTREAMS=1
```

公网部署必须设置允许的上游域名：

```text
REDACT_ALLOWED_HOSTS=api.openai.com,api.anthropic.com,relay.example.com
```

管理控制台的“运行设置”页面也可以编辑允许的上游域名。保存后新请求立即使用新白名单，无需重启网关；设置保存在数据目录的 SQLite 中，重启或 Docker 容器更新后仍然保留。首次启动且尚未在页面保存过设置时，使用 `REDACT_ALLOWED_HOSTS` 作为默认值。

网关禁止自动跟随上游重定向。非空请求体必须是 JSON；解析或脱敏失败时请求会被阻断，不会明文旁路。

## 自动构建

- `.github/workflows/ci.yml`：每次 push 和 pull request 执行前端、Go、Docker 构建与容器冒烟。
- `.github/workflows/release.yml`：推送 `v*` SemVer 标签时构建 `linux/amd64`、`linux/arm64` 镜像并推送 GHCR。
- 配置 `DOCKERHUB_USERNAME` 和 `DOCKERHUB_TOKEN` 后，同一 Release 会同时推送 Docker Hub。
