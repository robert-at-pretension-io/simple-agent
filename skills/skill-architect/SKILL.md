---
name: skill-architect
description: Validates skill structure and creates new skills following best practices.
version: 1.0.0
---

# Skill Architect

## Overview
This skill helps maintain the integrity of the "Skills" system. It provides tools to validate existing skills against the official specification and to scaffold new ones.

## Scripts

### `validate_skill.py`
Checks a skill directory for compliance:
- Valid `SKILL.md` frontmatter.
- Correct naming conventions.
- Existence of referenced scripts.

**Usage**: `scripts/validate_skill.py <path_to_skill_dir>`

### `new_skill.sh`
Creates a new skill with the standard directory structure and a template `SKILL.md`.

**Usage**: `scripts/new_skill.sh <skill_name>`
**Example**: `scripts/new_skill.sh my-new-feature`