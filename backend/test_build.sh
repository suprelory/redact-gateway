#!/bin/bash
export ADMIN_TOKEN="test-admin-token-for-verification"
export PROXY_PORT=18787
export MANAGEMENT_PORT=18788

echo "Testing gateway startup..."
timeout 3 ./gateway.exe &
PID=$!
sleep 2

# Check if process started
if ps -p $PID > /dev/null 2>&1; then
    echo "✓ Gateway started successfully (PID: $PID)"
    kill $PID
    exit 0
else
    echo "✗ Gateway failed to start"
    exit 1
fi
