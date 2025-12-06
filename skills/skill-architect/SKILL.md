---
name: skill-architect
description: Guide for creating effective skills. Use when creating new skills or updating existing ones to extend capabilities with specialized knowledge, workflows, or tool integrations.
---

# Skill Architect

This skill provides guidance and tools for creating effective skills.

## About Skills

Skills are modular, self-contained packages that extend the agent's capabilities by providing specialized knowledge, workflows, and tools.

### Core Principles

1. **Concise is Key**: The context window is shared. Only add context the agent doesn't already have.
2. **Appropriate Degrees of Freedom**:
   - **High freedom**: For tasks where multiple approaches are valid.
   - **Medium freedom**: Pseudocode or scripts with parameters.
   - **Low freedom**: Specific scripts for fragile operations.
3. **Progressive Disclosure**:
   - **Metadata**: Always in context.
   - **SKILL.md body**: Loaded when triggered.
   - **Bundled resources**: Loaded/Executed only as needed.

### Anatomy of a Skill

```
skill-name/
├── SKILL.md (required)
│   ├── YAML frontmatter (name, description)
│   └── Markdown instructions
└── Bundled Resources (optional)
    ├── scripts/          - Executable code (Python/Bash/etc.)
    ├── references/       - Documentation loaded as needed
    └── assets/           - Files used in output
```

### Hooks (Optional)

Hooks allow you to automate workflows by triggering scripts on specific system events. Define them in the `SKILL.md` frontmatter.

**Supported Hooks:**
- **`startup`**: Runs at session start (e.g., check dependencies).
- **`pre_edit` / `post_edit`**: Runs before/after `apply_udiff`. Useful for linting or testing.
- **`pre_view` / `post_view`**: Runs before/after `read_file`.
- **`pre_run` / `post_run`**: Runs before/after `run_script`.
- **`pre_commit`**: Runs before the agent proposes a git commit.

**Example Frontmatter:**
```yaml
---
name: my-skill
description: ...
hooks:
  startup: scripts/check_deps.sh
  post_edit: scripts/lint.sh
  pre_commit: scripts/test.sh
---
```

## Skill Creation Process

### Step 1: Initialize the Skill

Use the provided script to create a robust skill scaffold.

```bash
scripts/new_skill.sh <skill-name> [parent-path]
```

This generates:
- Skill directory
- `SKILL.md` template with best practices
- `scripts/`, `references/`, and `assets/` directories with examples

### Step 2: Edit the Skill

#### SKILL.md
- **Frontmatter**: `name` (hyphen-case), `description` (triggers), `hooks` (optional).
- **Body**: Imperative instructions. Keep it concise (< 500 lines).

#### Bundled Resources
- **scripts/**: Automate repetitive or fragile tasks.
- **references/**: Move large documentation here. Link from `SKILL.md` (e.g., "See [API.md](references/API.md)").
- **assets/**: Templates or files to be copied/used in output.

### Step 3: Validate

Run the validation script to ensure standards are met.

```bash
scripts/validate_skill.py <path-to-skill>
```

## Scripts

- **`new_skill.sh`**: Wraps `init_skill.py` to create a new skill structure.
- **`validate_skill.py`**: rigorous checks for frontmatter, naming, and structure.
- **`init_skill.py`**: The core python logic for initializing skills.

## References

See `references/` for design patterns:
- **[output-patterns.md](references/output-patterns.md)**: Templates for consistent output.
- **[workflows.md](references/workflows.md)**: Sequential and conditional workflow patterns.