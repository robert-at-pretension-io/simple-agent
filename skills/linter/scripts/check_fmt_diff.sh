#!/bin/bash
# Usage: check_fmt_diff.sh
# Shows formatting differences without modifying files.

echo "=== 🎨 Formatting Diff (gofmt -d) ==="
gofmt -d .