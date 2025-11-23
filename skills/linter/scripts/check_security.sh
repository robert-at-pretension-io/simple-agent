#!/bin/bash
# Usage: check_security.sh
# Checks for common Go security scanners (govulncheck, gosec) and runs them.

if command -v govulncheck &> /dev/null; then
    echo "=== 🛡️  Running govulncheck ==="
    govulncheck ./...
    exit 0
fi

if command -v gosec &> /dev/null; then
    echo "=== 🛡️  Running gosec ==="
    gosec ./...
    exit 0
fi

echo "=== ⚠️  Security Check Skipped ==="
echo "Neither 'govulncheck' nor 'gosec' found in PATH."
echo "To enable security checks, install one of them:"
echo "  go install golang.org/x/vuln/cmd/govulncheck@latest"
echo "  # or"
echo "  go install github.com/securego/gosec/v2/cmd/gosec@latest"
exit 0