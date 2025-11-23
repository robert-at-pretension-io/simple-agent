---
name: git-wizard
description: Extracts deep context from git history (status, logs, diffs). Use when onboarding, debugging regressions, or analyzing recent project activity.
version: 1.0.0
dependencies:
  - git
---

# Git Wizard

## Overview
This skill provides specialized tools to extract deep context from the git repository. Use this to orient yourself when starting a task, understanding what recently changed, or identifying volatile files.

## When to Apply
- **Start of Task**: IMMEDIATELY run `scripts/get_status.sh` and `scripts/recent_context.sh` to orient yourself. Do not ask for permission.
- **Debugging**: If the user mentions a bug or broken code, run `scripts/file_history.sh <filepath>` to see recent changes that might have caused it.
- **Project Analysis**: If the user asks for an overview or where to focus, run `scripts/churn_analysis.sh` to find active "hotspots".

## Scripts

### `get_status.sh`
Provides a "super-status". Shows current branch, dirty state, staged changes summary, and the last commit.
**Usage**: `scripts/get_status.sh`

### `recent_context.sh`
Summarizes the last 10 commits with stats. Helps you catch up on the project narrative.
**Usage**: `scripts/recent_context.sh [limit]` (default limit is 10)

### `file_history.sh`
Shows the recent evolution of a specific file, including diffs.
**Usage**: `scripts/file_history.sh <filepath>`

### `churn_analysis.sh`
Identifies the most frequently modified files in the repository history. High churn often correlates with technical debt or bugs.
**Usage**: `scripts/churn_analysis.sh`

## Examples
> User: "Why is main.go broken?"
> Claude: *Runs `scripts/file_history.sh main.go` to see who touched it last and what they changed.*

> User: "Where should I start refactoring?"
> Claude: *Runs `scripts/churn_analysis.sh` to find the files that change the most.*