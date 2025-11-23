#!/bin/bash

# Provides a comprehensive snapshot of the current git state

echo "=== 🌳 Branch & Status ==="
git status -sb

echo ""
echo "=== 📦 Staged Changes (Summary) ==="
if ! git diff --cached --stat --exit-code > /dev/null; then
    git diff --cached --stat
else
    echo "No staged changes."
fi

echo ""
echo "=== 📝 Unstaged Changes (Summary) ==="
if ! git diff --stat --exit-code > /dev/null; then
    git diff --stat
else
    echo "No unstaged changes."
fi

echo ""
echo "=== 📍 Last Commit ==="
git log -1 --stat --oneline --decorate