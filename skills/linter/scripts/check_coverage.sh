#!/bin/bash
# Usage: check_coverage.sh
# Runs tests with coverage and displays function-level stats.

echo "=== 🧪 Running Tests with Coverage ==="

# Run tests and generate profile
go test -coverprofile=coverage.out ./...

echo ""
echo "=== 📊 Function Coverage ==="
go tool cover -func=coverage.out
rm coverage.out