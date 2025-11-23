#!/bin/bash
# Usage: branch_report.sh
# Lists branches with tracking info and last commit msg.

echo "=== 🌿 Branch Overview ==="
git branch -vv --color=always
echo ""
echo "Use 'git log graph' for visual history."