#!/bin/bash
# Usage: search_content.sh <search_term>
# Uses git pickaxe (-S) to find commits that added or removed a string.

QUERY="$1"

if [ -z "$QUERY" ]; then
  echo "Usage: $0 <search_term>"
  exit 1
fi

echo "=== ⛏️  Searching Code History for '$QUERY' ==="
echo "Listing commits that added or removed this string..."

# -S looks for differences that introduce or remove an instance of <string>
git log -S"$QUERY" --oneline --color=always --stat

echo "=== End of Search ==="