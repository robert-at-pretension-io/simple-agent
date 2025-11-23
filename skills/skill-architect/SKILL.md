---
name: skill-architect
description: Validates skill structure and creates new skills following best practices.
version: 1.0.0
---

# Skill Architect

## Overview
This skill acts as the guardian of the "Skills" system. It provides tools to validate existing skills against the official specification and to scaffold new ones. Use this skill whenever creating, modifying, or auditing other skills to ensure they are high-quality, discoverable, and robust.

## Skill Authoring Standards
When creating or reviewing skills, STRICTLY adhere to the following standards derived from the official documentation.

### 1. Metadata (YAML Frontmatter)
- **`name`**:
  - **Constraint**: Max 64 characters.
  - **Format**: Lowercase letters, numbers, and hyphens ONLY (`^[a-z0-9-]+$`).
  - **Style**: Use gerund form (e.g., `processing-pdfs`, `analyzing-data`) or action-oriented noun phrases (`git-wizard`).
  - **Forbidden**: XML tags, reserved words (`anthropic`, `claude`), spaces, underscores.
- **`description`**:
  - **Constraint**: Max 1024 characters. Non-empty. No XML tags.
  - **Voice**: **Third-person imperative** (e.g., "Extracts text from PDFs", NOT "I can extract...").
  - **Content**: Must include WHAT the skill does and WHEN to use it (triggers).
  - **Discovery**: Use specific keywords users will mention (e.g., "Excel", "pivot tables", "bug", "refactor").
- **`version`**: Semantic versioning (e.g., `1.0.0`) is highly recommended.
- **`allowed-tools`**: (Optional) specific list of tools if you want to restrict the skill's capabilities (e.g., `Read, Grep`).

### 2. File Structure & Paths
- **Paths**: MUST use forward slashes (`scripts/myscript.py`), never backslashes.
- **Location**:
  - Personal: `~/.claude/skills/<skill-name>/`
  - Project: `.claude/skills/<skill-name>/` (or project root `skills/` in this repo).
- **Components**:
  - `SKILL.md` (Required): The entry point.
  - `scripts/`: Directory for executable code.
  - Reference files (`docs/`, `reference.md`): Use for large documentation to enable progressive disclosure.

### 3. Content & Philosophy
- **Conciseness**: `SKILL.md` body should be < 500 lines. Token efficiency is paramount.
- **Progressive Disclosure**:
  - Do not dump all info in `SKILL.md`. Link to reference files.
  - Links should be **one level deep** (e.g., `SKILL.md` -> `reference.md`). Avoid nested links (`SKILL.md` -> `ref.md` -> `detail.md`).
- **Degrees of Freedom**:
  - **Low Freedom**: For fragile tasks (DB migrations). "Run exactly this script."
  - **High Freedom**: For creative tasks (Code review). "Analyze structure, then suggest improvements."
- **No Time-Sensitive Info**: Use "Old Patterns" sections for deprecated APIs instead of relative dates ("Use X after 2024").

### 4. Script Best Practices
- **Solve, Don't Punt**: Scripts should handle errors (try/catch) and fix them or report specific issues, not crash and ask the agent to fix it.
- **No Voodoo Constants**: Document why a timeout is 30s.
- **Dependencies**: List required packages in `SKILL.md`. Do not assume tools are installed unless verified.
- **Execution**: Prefer executing scripts (`scripts/analyze.py`) over reading them.

### 5. Workflows
- **Plan-Validate-Execute**: For complex tasks, create a plan, validate it with a script, then execute.
- **Feedback Loops**: Implement loops where output is validated and corrected before finalization (e.g., "Edit XML -> Validate XML -> If Fail, Fix -> Repeat").

## Checklist for Effective Skills
Before finalizing a skill, verify:
- [ ] Name matches regex `^[a-z0-9-]+$` and uses gerunds where possible.
- [ ] Description is specific, third-person, and includes triggers.
- [ ] Paths are Unix-style.
- [ ] `SKILL.md` is concise and uses referenced files for bulk content.
- [ ] Scripts handle errors explicitly.
- [ ] No hardcoded secrets or Windows paths.

## Scripts

### `validate_skill.py`
Checks a skill directory for compliance:
- Valid `SKILL.md` frontmatter.
- Correct naming conventions.
- Existence of referenced scripts.
- Checks description length and format.

**Usage**: `scripts/validate_skill.py <path_to_skill_dir>`

### `new_skill.sh`
Creates a new skill with the standard directory structure and a template `SKILL.md` that adheres to these standards.

**Usage**: `scripts/new_skill.sh <skill_name>`
**Example**: `scripts/new_skill.sh my-new-feature`