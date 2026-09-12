# 快速开始指南

## 安装依赖

### 后端

```bash
cd backend
npm install
```

### 前端

```bash
cd frontend
npm install
```

## 配置环境

### 1. 创建环境变量文件

```bash
cd backend
cp .env.example .env
```

### 2. 编辑 `.env` 文件

必须设置的环境变量：

```bash
# 主密钥（至少 32 字符，用于加密映射表）
MASTER_KEY=your-secret-master-key-min-32-characters-here

# 管理令牌（至少 16 字符，用于访问管理界面）
ADMIN_TOKEN=your-admin-token-min-16-chars

# 端口配置（可选，使用默认值）
PROXY_PORT=18787
MANAGEMENT_PORT=18788

# 数据目录（可选）
DATA_DIR=./data
```

⚠️ **重要提示：**
- `MASTER_KEY` 必须至少 32 字符
- `ADMIN_TOKEN` 必须至少 16 字符
- 生产环境请使用强随机密钥

## 开发模式运行

### 启动后端（终端 1）

```bash
cd backend
npm run dev
```

后端将在以下端口启动：
- 代理端口: `http://127.0.0.1:18787`
- 管理端口: `http://127.0.0.1:18788`

### 启动前端（终端 2）

```bash
cd frontend
npm run dev
```

前端将在 `http://localhost:5173` 启动。

访问 http://localhost:5173 即可看到管理界面。

## 生产构建

### 构建后端

```bash
cd backend
npm run build
npm start
```

### 构建前端

```bash
cd frontend
npm run build
```

构建产物在 `frontend/dist` 目录。

## Docker 部署

### 使用 Docker Compose（推荐）

```bash
# 设置环境变量
export MASTER_KEY="your-secret-master-key-min-32-characters"
export ADMIN_TOKEN="your-admin-token-min-16-chars"

# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

### 手动 Docker 构建

```bash
# 构建镜像
docker build -t redact-gateway:latest .

# 运行容器
docker run -d \
  --name redact-gateway \
  -p 127.0.0.1:18787:18787 \
  -p 127.0.0.1:18788:18788 \
  -v redact-data:/data \
  -e MASTER_KEY="your-secret-key" \
  -e ADMIN_TOKEN="your-admin-token" \
  redact-gateway:latest
```

## 客户端配置

### Cursor

1. 打开 Cursor Settings → Models
2. 设置 OpenAI Base URL: `http://127.0.0.1:18787/v1`
3. 填入你的真实 OpenAI API Key

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
    api_key="your-real-api-key"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "测试脱敏：mysql://root:Pass123@192.168.1.50:3306/db"}]
)
print(response.choices[0].message.content)
```

## 验证安装

### 检查后端健康状态

```bash
curl http://127.0.0.1:18788/api/status
```

应该返回类似：

```json
{
  "success": true,
  "data": {
    "running": true,
    "version": "0.1.0",
    "uptime": 123
  }
}
```

### 访问管理界面

打开浏览器访问：http://127.0.0.1:18788

应该看到网关管理界面。

## 故障排查

### 后端启动失败

**错误：Master key not found**

解决：设置 `MASTER_KEY` 环境变量或创建 `data/master.key` 文件。

**错误：EADDRINUSE (端口被占用)**

解决：修改 `.env` 中的 `PROXY_PORT` 或 `MANAGEMENT_PORT`。

### 前端无法连接后端

检查后端是否运行：

```bash
curl http://127.0.0.1:18788/api/status
```

检查 Vite 代理配置（`frontend/vite.config.ts`）是否正确。

### Docker 容器无法启动

检查日志：

```bash
docker logs redact-gateway
```

常见原因：
- `MASTER_KEY` 未设置或长度不足
- 端口冲突
- 数据卷权限问题

## 下一步

- 阅读 [PROJECT_STRUCTURE.md](./PROJECT_STRUCTURE.md) 了解项目结构
- 阅读 [IMPLEMENTATION_SUMMARY.md](./IMPLEMENTATION_SUMMARY.md) 了解实现方案
- 开始开发核心引擎功能

## 需要帮助？

- 查看 [README.md](./README.md)
- 提交 GitHub Issue
- 查看项目文档
