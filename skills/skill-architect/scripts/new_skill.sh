#!/bin/bash

NAME="$1"

if [ -z "$NAME" ]; then
    echo "Usage: new_skill.sh <skill-name>"
    exit 1
fi

# Basic validation
if [[ ! "$NAME" =~ ^[a-z0-9-]+$ ]]; then
    echo "Error: Skill name must be lowercase, numbers, and hyphens only."
    exit 1
fi

TARGET="skills/$NAME"

if [ -d "$TARGET" ]; then
    echo "Error: Skill '$NAME' already exists."
    exit 1
fi

echo "Creating skill '$NAME'..."
mkdir -p "$TARGET/scripts"

cat > "$TARGET/SKILL.md" <<EOF
---
name: $NAME
description: Describe what this skill does and when to use it (max 1024 chars).
version: 1.0.0
---

# $NAME

## Overview
Provide a brief overview of the skill.

## Instructions
1. Step one
2. Step two

## Scripts
List your scripts here.
EOF

echo "Skill created at $TARGET"
echo "Edit $TARGET/SKILL.md to finish setup."
