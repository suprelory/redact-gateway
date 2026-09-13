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

Authorization 和厂商 API Key 头只会转发给 URL 中指定的上游，不写入事件库。日志仅记录上游域名、路径、状态、耗时、规则命中数量和脱敏字段路径，不记录请求正文、响应正文或查询参数。

控制台的请求日志支持打开详情，查看实际命中的脱敏规则和 JSON 字段路径（例如 `$.messages[0].content`）；详情不会显示敏感原文。

请求日志采用服务端分页，默认每页 50 条，可选择 20、50 或 100 条，并显示总条数及首页、上一页、下一页和末页控件。搜索覆盖全部保留的日志；更改搜索条件或每页条数会回到第一页，手动与自动刷新保留当前页。

管理接口 `GET /api/v1/events` 支持 `page`（从 1 开始）、`limit`（每页条数，默认 100，最多 500）、`q`（搜索上游、接口、协议、规则组合或状态）和 `upstream`（精确匹配上游域名）。响应包含 `events`、`total`、`page` 和 `limit`；超出末页时返回最后一页，无匹配记录时返回第 1 页和空数组。日志按时间倒序排列，同一毫秒内按记录 ID 倒序排列。

字段路径按请求去重，字符串形式的工具参数记录其所在字段（例如 `$.tool.arguments`），不会解析成内部字段路径。字段名本身命中检测规则时，在日志路径中隐藏为 `<redacted-key>`。升级时自动迁移已有 SQLite 数据库；历史日志保留原有命中规则，但无法补录升级前的字段路径。

工具参数中的 JSON 字符串在脱敏时先解码为原值，还原时按目标 JSON 层级转义。普通响应和 SSE 均支持包含换行、引号或反斜杠的敏感值，保持参数可解析，避免重复转义。

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

通过 `REDACT_ADMIN_TOKEN` 指定的管理令牌长度至少为 12 个字符。

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
