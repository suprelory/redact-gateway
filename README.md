# Redact Gateway

A high-performance sensitive data redaction gateway written in Go, designed for AI API proxying with automatic secret detection and restoration.

## Features

- 🔒 **Automatic Redaction**: Detects and redacts sensitive data before forwarding requests
- 🔄 **Transparent Restoration**: Automatically restores placeholders in responses
- 🚀 **High Performance**: Go-based implementation with <5ms latency overhead
- 🎯 **Smart Detection**: 13+ built-in rules + cross-entropy based secret detection
- 🔌 **Protocol Support**: OpenAI Chat API, Anthropic Messages API
- 🌊 **Streaming**: Full SSE streaming support with sliding window restoration
- 🔑 **Request Isolation**: Per-request mapping tables, zero persistence overhead

## Architecture

### Token Format

Placeholders use SHA256-based format: `{{Redact:sha256_hash}}`

```
Original: sk-ant-api03-abc123...
Redacted: {{Redact:f7c3bc1d808e04732adf679965ccc34ca7ae3441abc...}}
```

### Detection Methods

1. **Regex Matching**: Pattern-based detection for phones, emails, IDs
2. **Dictionary Matching**: Keyword-based with word boundary checking
3. **Cross-Entropy Detection**: Language-aware high-entropy string detection

## Quick Start

### Build

```bash
cd backend
go build -o gateway ./cmd/gateway
```

### Run

```bash
export ADMIN_TOKEN="your-secure-admin-token-here"
export PROXY_PORT=18787
export MANAGEMENT_PORT=18788

./gateway
```

### Usage

Route format: `/<flags>$<upstream-url>`

```bash
# Redact all types
curl http://localhost:18787/HPSIBE$https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{...}'

# Redact only high-entropy + secrets
curl http://localhost:18787/HS$https://api.anthropic.com/v1/messages \
  -H "Authorization: Bearer sk-ant-..." \
  -d '{...}'
```

### Detection Flags

- `H` - High entropy strings (API keys, tokens)
- `P` - Phone numbers
- `S` - Secrets (AWS keys, GitHub tokens, JWT)
- `I` - Identity (ID cards, SSN)
- `B` - Bank cards
- `E` - Email addresses

Default: `HPSIBE` (all enabled)

## Built-in Rules

| Priority | Type | Examples |
|----------|------|----------|
| 100 | IDENTITY | China ID cards, SSN |
| 100 | PHONE | China mobile, international |
| 95 | EMAIL | Standard email addresses |
| 95 | BANK | Credit card numbers (Luhn validated) |
| 95 | API_KEY | OpenAI, Anthropic keys |
| 90 | SECRET | AWS keys, GitHub tokens |
| 85 | SECRET | JWT, private key headers |
| 80 | NETWORK | Private IPv4 addresses |
| 70 | HIGH_ENTROPY | Base64, hex strings (9-128 chars) |

## API Endpoints

### Management API (`:18788`)

**GET /api/status**
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

**GET /api/rules**

Returns all available rules with configuration.

**POST /api/rules/test**

Test rules against sample text:
```json
{
  "text": "My phone is 13812345678 and email is user@example.com",
  "rules": ["china-phone", "email"]
}
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PROXY_PORT` | 18787 | Proxy server port |
| `MANAGEMENT_PORT` | 18788 | Management API port |
| `MASTER_KEY` | auto-generated | Encryption master key (32+ chars) |
| `ADMIN_TOKEN` | required | Admin API token (16+ chars) |
| `DATA_DIR` | ./data | Data directory for keys |
| `LOG_LEVEL` | info | Log level |

## Cross-Entropy Detection

Uses English bigram frequency tables to detect high-entropy strings that don't match natural language patterns:

- **Length-aware thresholds**: 9-128 characters, adaptive scoring
- **Shannon entropy filter**: Excludes low-entropy repetitive strings
- **False positive rate**: ~0.99% on natural English text
- **Recall rate**: >99% on random API keys/tokens (16+ chars)

## Request-Local Mapping

Following CosyRedactGateway design:

- Each request creates an isolated mapping table
- Mappings live only during request/response cycle
- No cross-request persistence
- Runtime salt generated once per process startup
- Same plaintext + same salt = same placeholder (within process lifetime)

## Performance

Expected metrics (Go vs Node.js):

| Metric | Go | Node.js |
|--------|-----|---------|
| Startup | <50ms | ~500ms |
| Memory | ~20MB | ~80MB |
| Latency | <5ms | ~10ms |
| Throughput | 10k+ req/s | ~3k req/s |

## Security

- API keys never logged or persisted
- Creator identity: SHA256(API key)
- Placeholder determinism: SHA256(plaintext + runtime_salt)
- No credential storage: mappings discarded after each request
- Master key auto-generated with 0600 permissions

## Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o gateway ./cmd/gateway

# Run
./gateway
```

## Docker

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY backend/ .
RUN go build -o gateway ./cmd/gateway

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/gateway /gateway
EXPOSE 18787 18788
CMD ["/gateway"]
```

## Migration from TypeScript

See [MIGRATION.md](MIGRATION.md) for details on the Node.js → Go transition.

Key changes:
- Token format: `{{TYPE_ULID}}` → `{{Redact:sha256}}`
- Storage: SQLite persistence → request-local memory
- Runtime: Node.js → Go native binary
- Framework: Fastify → Fiber

## License

MIT

## Reference

Inspired by [CosyRedactGateway](https://github.com/your-org/CosyRedactGateway) - the original Cloudflare Worker implementation.
