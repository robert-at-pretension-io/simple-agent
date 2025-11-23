### `check_security.sh`
Checks for known vulnerabilities using `govulncheck` or `gosec` if installed.

### `check_coverage.sh`
Runs Go tests with coverage profiling and displays function-level coverage statistics.

### `check_fmt_diff.sh`
Runs `gofmt -d` to show formatting differences without modifying the files. Useful for previewing changes.
---
name: linter
description: Run static analysis and style checks to ensure code quality and formatting.
version: 1.0.0
hooks:
  post_edit: scripts/lint.sh
dependencies:
  - go
---

## Overview
This Skill allows you to run linting checks on the project to ensure code quality and adherence to standards. It is designed to be language-aware, automatically detecting the project type and applying the appropriate checks.

## When to Apply
Apply this skill when:
- Verifying code correctness before a commit.
- Checking for formatting issues (e.g., indentation, style).
- Debugging potential static analysis errors.

## Scripts
### lint.sh
Detects the project language and runs the appropriate linter.

**Supported Languages:**
- **Go**: Checks for `go.mod`. Runs `go vet` for logic and `gofmt` for formatting.

## Usage
Execute the script from the skill directory:
`scripts/lint.sh`

## Examples
> User: "Run the linter."
> Claude: Executes `scripts/lint.sh`.