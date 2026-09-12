# Quick Start Guide

## Prerequisites

- Go 1.22+ (for building from source)
- OR Docker (for containerized deployment)

## Option 1: Build from Source

### Windows

```cmd
cd backend
set ADMIN_TOKEN=your-secure-admin-token-here
go build -o gateway.exe .\cmd\gateway
gateway.exe
```

### Linux/Mac

```bash
cd backend
export ADMIN_TOKEN="your-secure-admin-token-here"
go build -o gateway ./cmd/gateway
./gateway
```

## Option 2: Use Build Scripts

### Windows
```cmd
build.bat
set ADMIN_TOKEN=your-secure-token
gateway.exe
```

### Linux/Mac
```bash
./build.sh
export ADMIN_TOKEN="your-secure-token"
./gateway
```

## Option 3: Docker

```bash
# Create .env file
cp .env.example .env
# Edit .env and set ADMIN_TOKEN

# Build and run
docker-compose up -d

# Check status
curl http://localhost:18788/api/status
```

## Testing the Gateway

### 1. Check Status

```bash
curl http://localhost:18788/api/status
```

Expected response:
```json
{
  "status": "running",
  "version": "1.0.0",
  "runtime": {
    "salt": "3f8a9c2d1e4b..."
  },
  "rules": {
    "total": 13,
    "enabled": 13
  }
}
```

### 2. Test Rules

```bash
curl -X POST http://localhost:18788/api/rules/test \
  -H "Content-Type: application/json" \
  -d '{
    "text": "My phone is 13812345678 and email is user@example.com"
  }'
```

Expected response:
```json
{
  "redactedText": "My phone is {{Redact:abc123...}} and email is {{Redact:def456...}}",
  "matchCount": 2,
  "redactedCount": 2,
  "ruleHits": {
    "china-phone": 1,
    "email": 1
  },
  "mappings": [...]
}
```

### 3. Proxy AI API Request

```bash
# OpenAI example
curl http://localhost:18787/HPSIBE$https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer sk-your-openai-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "My API key is sk-ant-abc123 and phone is 13812345678"
      }
    ]
  }'
```

The gateway will:
1. Redact sensitive data in the request (API key, phone)
2. Inject a redact notice into the user message
3. Forward the redacted request to OpenAI
4. Restore placeholders in the response
5. Return the restored response to you

### 4. Try Different Detection Flags

```bash
# Only high-entropy detection
curl http://localhost:18787/H$https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -d '{...}'

# High-entropy + secrets
curl http://localhost:18787/HS$https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -d '{...}'

# All detection types (default)
curl http://localhost:18787/HPSIBE$https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -d '{...}'
```

## Flag Reference

| Flag | Type | Examples |
|------|------|----------|
| `H` | High Entropy | API keys, tokens (9-128 chars) |
| `P` | Phone | China mobile, international |
| `S` | Secret | AWS keys, GitHub tokens, JWT |
| `I` | Identity | China ID cards, SSN |
| `B` | Bank | Credit card numbers |
| `E` | Email | Standard email addresses |

**Default**: `HPSIBE` (all enabled)

## Environment Variables

Create a `.env` file or set environment variables:

```bash
# Required
ADMIN_TOKEN=your-secure-admin-token-min-16-chars

# Optional (defaults shown)
PROXY_PORT=18787
MANAGEMENT_PORT=18788
DATA_DIR=./data
LOG_LEVEL=info
MASTER_KEY=                    # Auto-generated if not provided
```

## Troubleshooting

### "ADMIN_TOKEN is required"
Set the `ADMIN_TOKEN` environment variable with at least 16 characters.

### Port already in use
Change `PROXY_PORT` or `MANAGEMENT_PORT` in your environment variables.

### Connection refused
Make sure the gateway is running and listening on the expected ports.

### Build fails
```bash
cd backend
go mod download
go mod tidy
go build -o gateway ./cmd/gateway
```

## Next Steps

- Read the full [README.md](README.md) for architecture details
- See [MIGRATION.md](MIGRATION.md) for TypeScript → Go changes
- Check [backend/TODO.md](backend/TODO.md) for implementation status
- Review built-in rules in `backend/internal/engine/builtin_rules.go`

## Support

For issues or questions, refer to the project documentation or create an issue in the repository.
