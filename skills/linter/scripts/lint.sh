#!/bin/bash

# Check for Go project
if [ -f "go.mod" ]; then
    echo "Detected Go project."
    if command -v go &> /dev/null; then
        echo "Running go vet..."
        go vet ./...
        vet_exit_code=$?

        echo "Running gofmt..."
        unformatted=$(gofmt -l .)
        if [ -n "$unformatted" ]; then
            echo "Formatting issues found in:"
            echo "$unformatted"
            fmt_exit_code=1
        else
            fmt_exit_code=0
        fi

        if [ $vet_exit_code -eq 0 ] && [ $fmt_exit_code -eq 0 ]; then
            echo "Linting passed."
            exit 0
        else
            echo "Linting failed."
            exit 1
        fi
    else
        echo "Error: 'go' command not found."
        exit 1
    fi
fi

# Fallback for other project types (to be implemented)
echo "No supported project type detected or linter not implemented for this type."
exit 0