#!/bin/bash

# Orchestrator script for Git Gatekeeper
# Returns 1 if any check fails

FAILED=0

echo "🔒 [Git Gatekeeper] Running pre-commit checks..."

# 1. Check for Conflict Markers
if ! bash skills/git-gatekeeper/scripts/check_conflict_markers.sh; then
    FAILED=1
fi

# 2. Check for Large Files
if ! bash skills/git-gatekeeper/scripts/check_large_files.sh; then
    FAILED=1
fi

# 3. Check for Secrets
if ! python3 skills/git-gatekeeper/scripts/check_secrets.py; then
    FAILED=1
fi

if [ $FAILED -ne 0 ]; then
    echo "❌ [Git Gatekeeper] Checks failed. Commit blocked."
    exit 1
fi

echo "✅ [Git Gatekeeper] All checks passed."
exit 0