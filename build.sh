#!/bin/bash
set -e

echo "Building Redact Gateway..."

# Build binary
cd backend
go build -o ../gateway ./cmd/gateway

echo "Build complete: ./gateway"
echo ""
echo "Usage:"
echo "  export ADMIN_TOKEN='your-secure-token'"
echo "  ./gateway"
