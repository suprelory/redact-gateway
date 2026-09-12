# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project structure and architecture
- 19 built-in redaction rules covering API keys, secrets, PII, and credentials
- TypeScript type system for gateway, rules, and mappings
- Dark theme UI design with cyan accent colors
- Configuration management with environment variables
- Structured logging system with Pino
- Docker multi-stage build configuration
- Docker Compose orchestration setup
- Complete frontend layout with 5 main pages
- API client abstraction layer
- Creator identity isolation system
- ULID-based placeholder generation

### Project Setup
- Backend: Node.js + TypeScript + Fastify
- Frontend: React 18 + TypeScript + Vite + Tailwind CSS
- Development tooling: ESLint + Prettier
- Deployment: Docker + Docker Compose

## [0.1.0] - 2026-09-12

### Project Initialization
- Created project skeleton
- Set up build and development environment
- Established code structure and conventions
- Added comprehensive documentation

---

## Version Numbering

- **MAJOR**: Incompatible API changes
- **MINOR**: Backward-compatible functionality additions
- **PATCH**: Backward-compatible bug fixes

## Release Process

1. Update version in `package.json` files
2. Update this CHANGELOG.md
3. Create git tag: `git tag -a v0.1.0 -m "Release v0.1.0"`
4. Push tag: `git push origin v0.1.0`
5. GitHub Actions will build and publish releases
