---
name: remember
description: Manage a project-specific knowledge base (remember.txt) to persist context, decisions, and lessons learned across sessions.
hooks:
  startup: inject_skill_md
---

# Remember Skill

This skill allows you to maintain a persistent memory file for the current project. This is crucial for keeping track of architectural decisions, important constraints, and "lessons learned" that should not be forgotten when the context window is reset.

## The Memory File
The knowledge base is stored in a file named **`remember.txt`** in the root of the project.

## How to Use

### 1. Reading Memory
To recall information, read the content of the memory file:
- Run: `run_script skills/remember/scripts/read.sh`
- Or simply: `cat remember.txt`

### 2. Curating Memory (IMPORTANT)
To add, update, or remove information, **you MUST use the `apply_udiff` tool**.
- **Do not** simply append text blindly.
- **Curate** the content: Organize facts into sections, update outdated info, and keep it concise.
- **The Edit Tool**: You have the `apply_udiff` tool available. Use it to surgically edit `remember.txt`.

### Best Practices
- **Check Memory First**: When starting a complex task, read `remember.txt` to ground yourself in the project context.
- **Record Decisions**: When you make a significant architectural choice or fix a tricky bug, add a note to `remember.txt`.
- **Project Specific**: This file is specific to the current working directory. Use it to store information relevant *only* to this project.
