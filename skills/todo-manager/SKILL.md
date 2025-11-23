---
name: todo-manager
description: Scans the codebase for TODO and FIXME comments to help manage technical debt and pending tasks.
version: 1.0.0
hooks:
  startup: scripts/stats.sh
---

# Todo Manager

This skill helps you stay on top of technical debt by finding and tracking `TODO` and `FIXME` comments in your code.

## Usage

### Scan for TODOs

To see a list of all TODOs in the project with their locations:

```bash
run_script skills/todo-manager/scripts/scan.py
```

### Get Statistics

To get a quick count of pending items:

```bash
run_script skills/todo-manager/scripts/stats.sh
```

## Hooks

- **Startup**: Runs `stats.sh` to show you the current count of TODOs when the session starts.