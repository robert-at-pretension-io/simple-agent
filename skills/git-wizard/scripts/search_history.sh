#!/bin/bash
# Usage: search_history.sh <search_term> [limit]

QUERY="$1"
LIMIT="${2:-20}"

if [ -z "$QUERY" ]; then
  echo "Usage: $0 <search_term> [limit]"
  exit 1
fi

echo "=== 🔍 Searching Commit Messages for '$QUERY' ==="
git log --oneline --grep="$QUERY" -n "$LIMIT" --color=always

echo ""
echo "(Showing top $LIMIT results)"