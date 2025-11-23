#!/bin/bash

# Find files with the most commits
LIMIT=${1:-15}

echo "=== 🔥 Codebase Hotspots (Top $LIMIT churned files) ==="
echo "Commits | File Path"
echo "--------|----------"

git log --all --find-renames --name-only --format='' | \
    sort | \
    uniq -c | \
    sort -rn | \
    head -n "$LIMIT"