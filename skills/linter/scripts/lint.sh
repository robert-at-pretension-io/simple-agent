#!/bin/bash

# Description:
#   This script performs linting and formatting checks on the codebase.
#   It attempts to detect the project language and run appropriate tools.

# Function to handle Go projects
lint_go() {
    echo "Detected Go project (go.mod found)."
    
    if command -v go &> /dev/null; then
        # 1. Static Analysis with go vet
        echo "Step 1: Running static analysis (go vet)..."
        if go vet ./...; then
            echo "  > go vet passed."
        else
            echo "  > go vet failed."
            exit 1
        fi

        # 2. Style Check with gofmt
        echo "Step 2: Checking code formatting (gofmt)..."
        unformatted=$(gofmt -l .)
        if [ -n "$unformatted" ]; then
            echo "  > Formatting issues found in the following files:"
            echo "$unformatted"
            echo "  > Please run 'gofmt -w .' to fix them."
            exit 1
        else
            echo "  > gofmt passed."
        fi
        
        echo "Success: All linting checks passed for Go project."
    else
        echo "Error: 'go' command not found. Please ensure Go is installed."
        exit 1
    fi
}

# Main execution
if [ -f "go.mod" ]; then
    lint_go
else
    echo "Notice: No supported project type detected (e.g., no go.mod found)."
    echo "Currently supported languages: Go."
    exit 0
fi