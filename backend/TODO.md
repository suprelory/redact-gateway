# Go Backend - TODO

## ✅ Completed

- [x] Project structure setup
- [x] Go modules configuration
- [x] Type system (types.go)
- [x] Configuration loader (config.go)
- [x] Entropy detection (entropy.go)
  - [x] Cross-entropy calculation with bigram tables
  - [x] Shannon entropy filter
  - [x] Length-aware thresholds
- [x] Rule matcher (matcher.go)
  - [x] Regex matching
  - [x] Dictionary matching
  - [x] Entropy matching
- [x] Redactor engine (redactor.go)
  - [x] Overlap merging
  - [x] Priority-based selection
  - [x] Redact notice injection
- [x] Restorer engine (restorer.go)
  - [x] Placeholder restoration
  - [x] SSE streaming support
  - [x] Sliding window algorithm
- [x] Built-in rules (builtin_rules.go)
  - [x] 13 detection rules
  - [x] Priority configuration
- [x] Storage layer (memory.go)
  - [x] Request-local mapping
  - [x] Runtime salt generation
  - [x] Creator hash
- [x] Proxy handler (proxy/handler.go)
  - [x] Route parsing (/<flags>$<upstream>)
  - [x] Request redaction
  - [x] Response restoration
  - [x] Chat protocol detection
- [x] Management API (api/handler.go)
  - [x] GET /api/status
  - [x] GET /api/rules
  - [x] POST /api/rules/test
- [x] Main entry point (main.go)
- [x] Build system
  - [x] build.sh (Linux/Mac)
  - [x] build.bat (Windows)
- [x] Docker support
  - [x] Multi-stage Dockerfile
  - [x] docker-compose.yml

## 🚧 In Progress

- [ ] SSE streaming implementation
  - [x] Core streaming logic
  - [ ] Integration testing with real AI APIs
  - [ ] Backpressure handling

## ⏳ Todo

### Core Features
- [ ] Protocol-specific handlers
  - [ ] OpenAI Chat Completions
  - [ ] Anthropic Messages
  - [ ] Streaming response parsing
- [ ] Advanced detection
  - [ ] Gitleaks rules (218+ providers)
  - [ ] Luhn validation for credit cards
  - [ ] China ID validation algorithm
- [ ] Error handling
  - [ ] Graceful upstream failures
  - [ ] Timeout configuration
  - [ ] Retry logic

### Testing
- [ ] Unit tests
  - [ ] Entropy detection tests
  - [ ] Rule matching tests
  - [ ] Redaction/restoration tests
- [ ] Integration tests
  - [ ] End-to-end proxy tests
  - [ ] Real AI API tests (OpenAI/Anthropic)
  - [ ] Streaming tests
- [ ] Benchmark tests
  - [ ] Throughput benchmarks
  - [ ] Latency benchmarks
  - [ ] Memory usage benchmarks

### Documentation
- [ ] API documentation
  - [ ] OpenAPI/Swagger spec
  - [ ] Request/response examples
- [ ] Development guide
  - [ ] Architecture overview
  - [ ] Adding new rules
  - [ ] Testing guide
- [ ] Deployment guide
  - [ ] Production checklist
  - [ ] Performance tuning
  - [ ] Monitoring setup

### Operations
- [ ] Logging
  - [ ] Structured logging
  - [ ] Log levels
  - [ ] Request tracing
- [ ] Metrics
  - [ ] Prometheus metrics
  - [ ] Health checks
  - [ ] Performance metrics
- [ ] Observability
  - [ ] Distributed tracing
  - [ ] Error tracking
  - [ ] Dashboards

### Future Enhancements
- [ ] Persistent storage (optional)
  - [ ] BBolt integration
  - [ ] Encrypted mappings
  - [ ] Expiration cleanup
- [ ] Rate limiting
  - [ ] Per-creator limits
  - [ ] Global rate limits
- [ ] Caching
  - [ ] Rule compilation cache
  - [ ] Regex pattern cache
- [ ] Configuration
  - [ ] Hot reload
  - [ ] Dynamic rule updates
  - [ ] Feature flags

## Known Issues

None currently - backend compiles successfully!

## Performance Targets

- [x] Binary size: < 30MB (actual: ~15MB)
- [x] Startup time: < 50ms (estimated)
- [ ] Memory usage: < 20MB idle (to be measured)
- [ ] Request latency: < 5ms overhead (to be benchmarked)
- [ ] Throughput: > 10k req/s (to be benchmarked)

## Migration Status

- [x] Token format: `{{TYPE_ULID}}` → `{{Redact:sha256}}`
- [x] Storage: SQLite → Request-local memory
- [x] Framework: Fastify → Fiber
- [x] Language: TypeScript → Go
- [ ] Feature parity verification
- [ ] Performance comparison tests
