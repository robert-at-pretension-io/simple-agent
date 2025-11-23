---
name: test-sentinel
description: Automatically runs tests when files are modified.
version: 1.0.0
hooks:
  post_edit: scripts/run_test.sh
---

# Test Sentinel

## Overview
Provides immediate feedback by running unit tests associated with the modified file.

## Scripts

### `run_test.sh`
Identifies the corresponding `_test.go` file and runs it.

**Usage**: `scripts/run_test.sh <modified_file_path>`