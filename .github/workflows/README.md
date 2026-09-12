# GitHub Actions Workflows

This directory contains automated CI/CD workflows for the Redact Gateway project.

## Workflows

### 1. CI (`ci.yml`)

**Trigger**: Push to `master`/`main`/`develop` branches, or pull requests

**Jobs**:
- **Test**: Run unit tests with coverage reporting
- **Build**: Build binaries for multiple platforms (Linux, macOS, Windows) and architectures (amd64, arm64)
- **Docker**: Test Docker image build
- **Lint**: Run golangci-lint for code quality checks

**Artifacts**: Build artifacts are retained for 7 days

### 2. Release (`release.yml`)

**Trigger**: Push tags matching `v*` (e.g., `v1.0.0`, `v2.1.3-beta`)

**Jobs**:
- **Build**: Create release binaries for all platforms with version info embedded
- **Docker**: Build and push multi-arch Docker images to:
  - GitHub Container Registry (`ghcr.io/suprelory/redact-gateway`)
  - Docker Hub (`suprelory/redact-gateway`)
- **Release**: Create GitHub release with:
  - Changelog from git commits
  - Binary attachments for all platforms
  - SHA256 checksums
  - Docker pull instructions

**Docker Tags**:
- Semver tags: `v1.2.3`, `v1.2`, `v1`
- SHA tag: `master-sha-abc1234`
- Latest tag (if default branch)

### 3. Docker Latest (`docker-latest.yml`)

**Trigger**: Push to `master`/`main` branches (backend changes only), or manual dispatch

**Jobs**:
- Build and push `latest` tag to GitHub Container Registry
- Push SHA-based tag for exact version tracking

## Required Secrets

Configure these secrets in your GitHub repository settings:

### For Docker Hub (optional)
- `DOCKERHUB_USERNAME`: Your Docker Hub username
- `DOCKERHUB_TOKEN`: Docker Hub access token (generate at https://hub.docker.com/settings/security)

### Automatic (no configuration needed)
- `GITHUB_TOKEN`: Provided automatically by GitHub Actions
- Used for GHCR and GitHub Releases

## Usage Examples

### Continuous Integration
```bash
# Push triggers CI
git push origin master

# Or create a PR
gh pr create
```

### Creating a Release
```bash
# Tag and push
git tag v1.0.0
git push origin v1.0.0

# Automated:
# - Builds binaries for all platforms
# - Pushes Docker images to GHCR and Docker Hub
# - Creates GitHub release with artifacts
```

### Manual Docker Build
```bash
# Trigger from GitHub UI:
# Actions → Docker Latest → Run workflow
```

## Docker Image Usage

### Pull from GitHub Container Registry
```bash
docker pull ghcr.io/suprelory/redact-gateway:latest
docker pull ghcr.io/suprelory/redact-gateway:v1.0.0
```

### Pull from Docker Hub
```bash
docker pull suprelory/redact-gateway:latest
docker pull suprelory/redact-gateway:v1.0.0
```

### Multi-architecture Support
Images are built for:
- `linux/amd64`
- `linux/arm64`

Docker automatically pulls the correct architecture for your platform.

## Workflow Permissions

The workflows require these permissions:
- `contents: write` - For creating releases
- `packages: write` - For pushing to GHCR
- `contents: read` - For checking out code

These are configured in the workflow files and should work automatically for repository owners.

## Dependabot

Dependabot is configured to automatically:
- Update Go module dependencies weekly
- Update GitHub Actions versions weekly
- Update Dockerfile base images weekly

See `.github/dependabot.yml` for configuration.

## Status Badges

Add these badges to your README:

```markdown
[![CI](https://github.com/suprelory/redact-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/suprelory/redact-gateway/actions/workflows/ci.yml)
[![Release](https://github.com/suprelory/redact-gateway/actions/workflows/release.yml/badge.svg)](https://github.com/suprelory/redact-gateway/actions/workflows/release.yml)
[![Docker](https://github.com/suprelory/redact-gateway/actions/workflows/docker-latest.yml/badge.svg)](https://github.com/suprelory/redact-gateway/actions/workflows/docker-latest.yml)
```

## Troubleshooting

### Docker Hub push fails
- Ensure `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets are set
- Verify token has write permissions
- Check repository name matches: `suprelory/redact-gateway`

### Release creation fails
- Ensure the tag follows semver: `v1.2.3`
- Check repository permissions (Settings → Actions → General → Workflow permissions)
- Verify `GITHUB_TOKEN` has write access

### Build artifacts missing
- Check that the build step completed successfully
- Verify artifact upload/download steps in workflow logs
- Ensure artifact retention period hasn't expired (7 days for CI, 30 days for releases)
