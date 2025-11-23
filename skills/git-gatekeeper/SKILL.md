---
name: git-gatekeeper
description: A security and hygiene skill that validates commits before they happen. It checks for hardcoded secrets, git conflict markers, and large files.
version: 1.0.0
hooks:
  pre_commit: scripts/run_checks.sh
---

# Git Gatekeeper

## Overview
This skill acts as a safety net for your git workflow. It automatically runs whenever you attempt to commit changes (via the system's `pre_commit` hook).

## What it checks
1.  **Secrets**: Scans staged files for potential API keys, private keys, and passwords.
2.  **Conflict Markers**: Ensures you aren't committing files with `<<<<<<< HEAD` or `>>>>>>>`.
3.  **Large Files**: Warns if you try to commit files larger than 1MB (configurable).

## Usage
This skill runs automatically. You do not need to invoke it manually.

If a check fails, the commit will be blocked, and you will see an error message explaining what needs to be fixed.

## Scripts
- `run_checks.sh`: The orchestrator script triggered by the hook.
- `check_secrets.py`: Python script using regex to find potential secrets.
- `check_conflict_markers.sh`: Shell script to grep for conflict markers.
- `check_large_files.sh`: Shell script to check file sizes.

## Bypass
If you absolutely must bypass these checks (not recommended), you can use `git commit --no-verify` manually in a terminal, but the agent will always try to respect these checks.