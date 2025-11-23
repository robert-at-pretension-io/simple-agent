#!/bin/bash

# Count TODOs
TODO_COUNT=$(grep -r "TODO" . --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=skills | wc -l)

# Count FIXMEs
FIXME_COUNT=$(grep -r "FIXME" . --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=skills | wc -l)

if [ "$TODO_COUNT" -eq "0" ] && [ "$FIXME_COUNT" -eq "0" ]; then
    exit 0
fi

echo "📝 Project Status:"
echo "   - Pending TODOs:  $TODO_COUNT"
echo "   - Pending FIXMEs: $FIXME_COUNT"