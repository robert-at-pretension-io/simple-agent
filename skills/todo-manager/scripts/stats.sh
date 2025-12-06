#!/bin/bash

if git rev-parse --is-inside-work-tree > /dev/null 2>&1; then
    # FAST PATH: Use git grep (respects .gitignore)
    TODO_COUNT=$(git grep "TODO" 2>/dev/null | wc -l)
    FIXME_COUNT=$(git grep "FIXME" 2>/dev/null | wc -l)
else
    # SLOW PATH: Not a git repo.
    # Safety check: Don't scan huge directories (like $HOME) unless they look like projects.
    if [ ! -f "go.mod" ] && [ ! -f "package.json" ] && [ ! -f "Makefile" ] && [ ! -f "requirements.txt" ] && [ ! -f "pom.xml" ]; then
        # Not a recognizable project root. Skip scan to prevent freezing.
        exit 0
    fi

    # Fallback with extra excludes for common dependency folders
    # Note: Avoid brace expansion for POSIX compatibility
    EXCLUDES="--exclude-dir=.git --exclude-dir=node_modules --exclude-dir=vendor --exclude-dir=dist --exclude-dir=build --exclude-dir=target --exclude-dir=venv --exclude-dir=.venv --exclude-dir=bin --exclude-dir=obj"
    
    TODO_COUNT=$(grep -r "TODO" . $EXCLUDES 2>/dev/null | wc -l)
    FIXME_COUNT=$(grep -r "FIXME" . $EXCLUDES 2>/dev/null | wc -l)
fi

if [ "$TODO_COUNT" -eq "0" ] && [ "$FIXME_COUNT" -eq "0" ]; then
    exit 0
fi

echo "📝 Project Status:"
echo "   - Pending TODOs:  $TODO_COUNT"
echo "   - Pending FIXMEs: $FIXME_COUNT"