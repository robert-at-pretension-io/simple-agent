---
name: dependency-radar
description: proactive check for outdated dependencies or vulnerabilities.
version: 1.0.0
hooks:
  startup: scripts/check.sh
---

# Dependency Radar

## Overview
This skill acts as a watchdog for your project dependencies. It automatically runs on startup to identify outdated packages or potential security risks.

## Scripts

### `check.sh`
Runs `go list -u -m all` to find available updates.

**Usage**: `scripts/check.sh`