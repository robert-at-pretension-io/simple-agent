#!/bin/bash
# Wrapper to force python3 usage since the main app defaults to 'python'
SCRIPT_DIR=$(dirname "$0")
python3 "$SCRIPT_DIR/check.py" "$@"