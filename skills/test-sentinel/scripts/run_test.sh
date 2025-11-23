#!/bin/bash
FILE="$1"

if [[ ! "$FILE" =~ \.go$ ]]; then
    exit 0
fi

TEST_FILE=""

if [[ "$FILE" =~ _test\.go$ ]]; then
    TEST_FILE="$FILE"
else
    TEST_FILE="${FILE%.go}_test.go"
fi

if [ -f "$TEST_FILE" ]; then
    echo "[Test Sentinel] Running tests for $FILE..."
    go test -v "$TEST_FILE"
    # We assume the test might need the package context, often `go test ./pkg/...` is better but file-specific is faster.
fi