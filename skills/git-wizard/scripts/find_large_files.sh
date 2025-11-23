#!/bin/bash
# Usage: find_large_files.sh [top_n]
# Finds largest files in the current HEAD.

TOP_N="${1:-10}"

echo "=== 🐘 Top $TOP_N Largest Files ==="
# list files, get objects, sort by size, take top N
git rev-list --objects --all \
| git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' \
| sed -n 's/^blob //p' \
| sort --numeric-sort --key=2 --reverse | head -n "$TOP_N" | awk '{print $2 " " $3}' | numfmt --to=iec-i --suffix=B --field=1