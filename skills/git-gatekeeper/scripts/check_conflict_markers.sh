#!/bin/bash

# Check staged files for git conflict markers

FILES=$(git diff --name-only --cached)

if [ -z "$FILES" ]; then
    exit 0
fi

FAILED=0

for file in $FILES; do
    if [ -f "$file" ]; then
        if grep -lE "^<<<<<<< |^=======$|^>>>>>>> " "$file" > /dev/null; then
            echo "❌ Conflict markers found in: $file"
            FAILED=1
        fi
    fi
done

exit $FAILED