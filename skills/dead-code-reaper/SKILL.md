---
name: dead-code-reaper
description: Scans for exported functions or variables that are never imported or used elsewhere.
version: 1.0.0
---

# Dead Code Reaper

## Overview
Keeps the codebase lean by identifying obsolete code that should be deleted.

## Scripts

### `scan.py`
Scans the project for exported functions (capitalized names in Go) and checks if they are used in other files.

**Usage**: `scripts/scan.py`