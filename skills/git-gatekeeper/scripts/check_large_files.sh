#!/bin/bash

# Check staged files for size limit (1MB)
LIMIT=1048576

FILES=$(git diff --name-only --cached)

if [ -z "$FILES" ]; then
    exit 0
fi

FAILED=0

for file in $FILES; do
    if [ -f "$file" ]; then
        # Portable way to get file size
        size=$(wc -c < "$file" | tr -d ' ')
        if [ "$size" -gt "$LIMIT" ]; then
            echo "❌ File too large ($size bytes > $LIMIT bytes): $file"
            FAILED=1
        fi
    fi
done

exit $FAILED