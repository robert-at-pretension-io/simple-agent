---
name: complexity-cop
description: Checks code complexity to prevent unmaintainable code.
version: 1.0.0
hooks:
  post_edit: scripts/check.py
---

# Complexity Cop

## Overview
Monitors code complexity. If a file becomes too complex (too many nested blocks), it warns the user.

## Scripts

### `check.py`
Calculates indentation depth as a proxy for complexity.

**Usage**: `scripts/check.py <file_path>`