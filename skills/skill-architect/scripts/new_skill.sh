#!/bin/bash

SKILL_NAME="$1"
SKILL_PATH="${2:-skills}"

if [ -z "$SKILL_NAME" ]; then
    echo "Usage: new_skill.sh <skill-name> [parent-path]"
    echo "Example: new_skill.sh my-skill skills/public"
    exit 1
fi

# Get the directory where this script is located to find init_skill.py
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Execute the python initialization script
python3 "$SCRIPT_DIR/init_skill.py" "$SKILL_NAME" --path "$SKILL_PATH"
