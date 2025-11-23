#!/bin/bash

LIMIT=${1:-10}

echo "=== 📜 Recent History (Last $LIMIT commits) ==="
git log -n "$LIMIT" --stat --graph --format="%h - %an (%ar): %s"