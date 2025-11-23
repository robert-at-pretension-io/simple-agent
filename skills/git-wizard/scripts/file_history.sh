#!/bin/bash

FILE=$1

if [ -z "$FILE" ]; then
    echo "Usage: $0 <filepath>"
    exit 1
fi

if [ ! -f "$FILE" ]; then
    echo "Error: File '$FILE' not found."
    exit 1
fi

echo "=== 🕰️ Evolution of $FILE (Last 5 changes) ==="
git log -n 5 -p -- "$FILE"