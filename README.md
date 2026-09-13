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

日志单独显示还原结果，区分已还原、部分未还原、存在未还原、响应未回显占位符、无脱敏内容及响应异常。详情提供还原次数、唯一还原数、未还原次数和容错还原次数；同一占位符在 SSE 增量与完整快照中重复出现会累计替换次数，唯一还原数按占位符去重。旧记录显示“未统计”，不会把缺失的历史诊断信息当作零。Responses 的 `response.failed`、`response.incomplete` 和流内 `error` 事件即使 HTTP 为 200，也会记录错误分类。

请求日志采用服务端分页，默认每页 50 条，可选择 20、50 或 100 条，并显示总条数及首页、上一页、下一页和末页控件。搜索覆盖全部保留的日志；更改搜索条件或每页条数会回到第一页，手动与自动刷新保留当前页。

管理接口 `GET /api/v1/events` 支持 `page`（从 1 开始）、`limit`（每页条数，默认 100，最多 500）、`q`（搜索上游、接口、协议、规则组合或状态）和 `upstream`（精确匹配上游域名）。响应包含 `events`、`total`、`page` 和 `limit`；超出末页时返回最后一页，无匹配记录时返回第 1 页和空数组。日志按时间倒序排列，同一毫秒内按记录 ID 倒序排列。

字段路径按请求去重，字符串形式的工具参数记录其所在字段（例如 `$.tool.arguments`），不会解析成内部字段路径。字段名本身命中检测规则时，在日志路径中隐藏为 `<redacted-key>`。升级时自动迁移已有 SQLite 数据库；历史日志保留原有命中规则，但无法补录升级前的字段路径。

工具参数中的 JSON 字符串在脱敏时先解码为原值，还原时按目标 JSON 层级转义。普通响应和 SSE 均支持包含换行、引号或反斜杠的敏感值，保持参数可解析，避免重复转义。

已知占位符支持有限的格式容错：括号缺失、括号被反斜杠转义、大小写变化或标签中增加下划线。容错仍须匹配原有完整随机标识和归一化标签；不会根据相似后缀猜原文，也不会替换普通标识符中的片段。未知占位符原样保留并计入未还原，`{{RG_TYPE_TOKEN}}` 这类格式示例不计入。

SSE 按文本、工具调用及内容索引分别恢复，只暂存可能属于占位符的尾部；确定的前缀、其他通道和心跳立即转发。尾部会在对应的结束事件或 EOF 前输出；Responses 的完整快照与已输出原文前缀一致时，可用于补全未发送完的占位符。工具参数分片支持 JSON 转义和 Unicode 转义。单个 SSE 事件上限为 `REDACT_MAX_BODY_BYTES` 的两倍（默认 32 MiB），每条流最多维护 1024 个通道，暂存尾部及事件模板合计不超过 4 MiB。超限时终止流，保留此前已完成的输出，日志记录 `stream_limit_exceeded` 和“响应异常”；已经发出的 HTTP 200 不会改变。

只有存在可还原映射时才向用户消息添加脱敏提示。提示要求静默完成原任务，按需原样引用占位符，不再包含可被模型复述的占位符格式示例。系统及开发者指令保持原有优先级；本次请求及启用的会话中均没有映射时，不会添加提示。

## 可选的会话映射缓存

默认每个请求独立保存映射。如果客户端仅通过 Responses 的 `previous_response_id` 续接，上游可能再次输出先前请求的占位符，而当前请求并没有对应原文。需要跨请求恢复时，在 `.env` 中启用 `REDACT_SESSION_CACHE_ENABLED=1`，重新创建容器，并让客户端为每段对话生成独立的随机 `X-Redact-Session`，在续接时保持一致。

```python
from uuid import uuid4

# client 使用上文配置的网关地址和上游 API Key。
session_headers = {"X-Redact-Session": str(uuid4())}
first = client.responses.create(
    model="your-model",
    input="请记住后续任务使用的邮箱 alice@example.com",
    extra_headers=session_headers,
)
second = client.responses.create(
    model="your-model",
    previous_response_id=first.id,
    input="重复刚才的邮箱",
    extra_headers=session_headers,
)
```

会话头只由网关消费，不转发给上游，也不写入日志。标识须为 16–256 个 ASCII 字母、数字、下划线或连字符；UUID 符合要求。缓存按会话标识、转发的授权/API Key、组织/项目以及上游完整接口地址（含端口、路径和查询参数）隔离。缺少会话标识或上游凭据时继续按请求隔离；同一 API Key 下不同用户或对话应使用不同标识。改变凭据、项目或接口地址后不会共享之前的映射。

| 环境变量 | 默认值 | 含义 |
|---|---|---|
| `REDACT_SESSION_CACHE_ENABLED` | `0` | 显式启用内存缓存 |
| `REDACT_SESSION_CACHE_TTL_SECONDS` | `1800` | 会话空闲过期时间，每次同范围请求刷新 |
| `REDACT_SESSION_CACHE_MAX_SESSIONS` | `128` | 全局会话上限 |
| `REDACT_SESSION_CACHE_MAX_ENTRIES` | `16384` | 全局映射条数上限 |
| `REDACT_SESSION_CACHE_MAX_BYTES` | `16777216` | 原文、占位符及索引开销的缓存预算，至少 1024 字节 |

缓存容量不足时优先淘汰较久未使用的会话；单个会话用满预算或单值过大时，新值仍可在当前请求恢复，但不会跨请求缓存。映射仅存在进程内存中，不写 SQLite 或磁盘；正在处理的请求持有独立映射，缓存过期、淘汰或关闭不会破坏这些响应的还原。还原次数与规则命中仍按请求统计，不继承旧请求的计数。

过期、淘汰、进程重启或请求落到其他实例后，历史映射可能不可用。未知占位符会原样保留并计入“未还原”；此时应重新发送必要的原始上下文并开始新的上游会话。多实例部署需要会话请求落到同一实例，缓存不提供持久化续接。Responses 顶层的 `previous_response_id` 与 `input` 项中的 `encrypted_content` 按协议原样传递，不经过高熵脱敏。

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
