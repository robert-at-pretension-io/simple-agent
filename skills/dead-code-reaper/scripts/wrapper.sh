#!/bin/bash
# Wrapper to force python3 usage
SCRIPT_DIR=$(dirname "$0")
python3 "$SCRIPT_DIR/scan.py" "$@"