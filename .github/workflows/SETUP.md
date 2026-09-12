# GitHub Actions 配置指南

## 快速开始

工作流已经创建完成，只需推送到 GitHub 即可自动激活。

## 推送代码

```bash
# 推送所有提交到 GitHub
git push -u origin master

# CI 工作流将自动触发
# 查看构建状态: https://github.com/suprelory/redact-gateway/actions
```

## Docker Hub 配置（可选）

如果需要同时推送到 Docker Hub，需要配置以下 Secret：

### 1. 创建 Docker Hub 访问令牌

1. 登录 Docker Hub: https://hub.docker.com
2. 进入 Account Settings → Security
3. 点击 "New Access Token"
4. 设置名称: `github-actions-redact-gateway`
5. 权限选择: Read & Write
6. 复制生成的 token（只显示一次）

### 2. 在 GitHub 仓库配置 Secret

访问: https://github.com/suprelory/redact-gateway/settings/secrets/actions

添加以下 Secret：

| 名称 | 值 | 说明 |
|------|-----|------|
| `DOCKERHUB_USERNAME` | 你的 Docker Hub 用户名 | 如: `suprelory` |
| `DOCKERHUB_TOKEN` | 刚才生成的访问令牌 | 完整的 token 字符串 |

### 3. 验证配置

配置完成后，下次推送 tag 时会自动推送到两个仓库：
- ✅ `ghcr.io/suprelory/redact-gateway` (GitHub Container Registry)
- ✅ `suprelory/redact-gateway` (Docker Hub)

## 创建首个版本发布

```bash
# 打标签
git tag v1.0.0

# 推送标签（触发 Release 工作流）
git push origin v1.0.0
```

Release 工作流会自动：
1. 构建 5 个平台的二进制文件
2. 构建并推送 Docker 镜像（双架构）
3. 生成 Changelog
4. 创建 GitHub Release
5. 上传所有构建产物和校验和

## 工作流说明

### CI (`ci.yml`)

**触发时机**:
- 推送到 `master`/`main`/`develop` 分支
- 创建 Pull Request

**执行内容**:
- 运行单元测试（带覆盖率报告）
- 构建 6 个平台的二进制文件
- Docker 镜像构建测试
- 运行 golangci-lint 代码检查

**构建平台**:
- Linux: amd64, arm64
- macOS: amd64, arm64  
- Windows: amd64

### Release (`release.yml`)

**触发时机**:
- 推送匹配 `v*` 的标签（如 `v1.0.0`, `v2.1.3-beta`）

**执行内容**:
- 构建所有平台发布版本（带版本信息）
- 构建 Docker 多架构镜像
- 推送到 GHCR 和 Docker Hub
- 生成 Changelog
- 创建 GitHub Release
- 上传二进制文件和 SHA256 校验和

**Docker 标签**:
- `v1.2.3` (完整版本)
- `v1.2` (主次版本)
- `v1` (主版本)
- `latest` (最新稳定版)
- `master-sha-abc1234` (Git SHA)

### Docker Latest (`docker-latest.yml`)

**触发时机**:
- 推送到 `master`/`main` 分支（仅 backend 代码变更）
- 手动触发 (workflow_dispatch)

**执行内容**:
- 构建并推送 `latest` 标签到 GHCR
- 推送 SHA 标签用于精确版本追踪

## 状态徽章

在 README.md 中添加构建状态徽章：

```markdown
[![CI](https://github.com/suprelory/redact-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/suprelory/redact-gateway/actions/workflows/ci.yml)
[![Release](https://github.com/suprelory/redact-gateway/actions/workflows/release.yml/badge.svg)](https://github.com/suprelory/redact-gateway/actions/workflows/release.yml)
[![Docker](https://github.com/suprelory/redact-gateway/actions/workflows/docker-latest.yml/badge.svg)](https://github.com/suprelory/redact-gateway/actions/workflows/docker-latest.yml)
```

## Dependabot 自动更新

已配置 Dependabot 自动更新：
- Go 模块依赖（每周）
- GitHub Actions 版本（每周）
- Dockerfile 基础镜像（每周）

Dependabot 会自动创建 PR，你只需要审查并合并。

## 常见问题

### 1. Docker Hub 推送失败

**原因**: Secret 未配置或配置错误

**解决**:
- 检查 Secret 是否正确配置
- 验证 Docker Hub token 是否有写权限
- 确认用户名拼写正确

### 2. Release 创建失败

**原因**: 权限不足或标签格式错误

**解决**:
- 确保标签遵循 semver: `v1.2.3`
- 检查仓库权限: Settings → Actions → General → Workflow permissions
- 确保选择了 "Read and write permissions"

### 3. 构建产物缺失

**原因**: 构建步骤失败或产物保留期过期

**解决**:
- 查看工作流日志确认构建是否成功
- CI 产物保留 7 天，Release 产物保留 30 天
- 检查 artifact upload/download 步骤

### 4. 只想推送到 GHCR，不想推送到 Docker Hub

**解决**: 不配置 `DOCKERHUB_USERNAME` 和 `DOCKERHUB_TOKEN` 即可。

工作流会检测 Secret 是否存在，如果不存在会跳过 Docker Hub 推送（但会显示警告）。

如需彻底禁用，可以编辑 `release.yml`，移除 Docker Hub 相关的登录和推送步骤。

## 手动触发 Docker 构建

如果需要手动触发 Docker 镜像构建：

1. 访问: https://github.com/suprelory/redact-gateway/actions/workflows/docker-latest.yml
2. 点击 "Run workflow"
3. 选择分支（通常是 master）
4. 点击绿色的 "Run workflow" 按钮

## 查看构建历史

访问仓库的 Actions 页面：
https://github.com/suprelory/redact-gateway/actions

可以看到：
- 所有工作流的运行历史
- 每次运行的详细日志
- 构建产物下载链接
- 成功/失败状态

## 下一步

1. **立即执行**: `git push -u origin master` 激活 CI
2. **可选配置**: 添加 Docker Hub Secret（如需推送到 Docker Hub）
3. **创建发布**: `git tag v1.0.0 && git push origin v1.0.0`
