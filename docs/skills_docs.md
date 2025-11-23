# Implementing an Agent Skills System

> A guide for architects and developers building a file-based skill discovery and execution system for AI agents.

In the age of autonomous AI agents, "Skills" are not just hard-coded functions but dynamic, user-defined capabilities. This document describes how to implement a system where an agent uses its fundamental tools (`read_file`, `execute_script`, `edit_file`) to discover and execute complex workflows defined in data.

## The Purpose of Skills

Skills serve as a bridge between general-purpose reasoning and domain-specific execution.

1.  **Extensibility**: Users can teach the agent new tricks (e.g., "deploy to AWS", "audit code style") without modifying the agent's core binary.
2.  **Encapsulation**: Complex logic is hidden in scripts and instructions, keeping the agent's main context window clean until the skill is needed.
3.  **Agent Autonomy**: Instead of the system "running" the tool, the agent *reads* the manual (the skill definition) and *drives* the execution itself.

## Core Architecture

The system relies on providing the agent with a **Map** of what is available, and the **Means** to access it.

1.  **Discovery Engine**: Scans filesystem paths to build an index of available skills.
2.  **Skill Definition**: A standardized format (Markdown/YAML) that acts as the "instruction manual" for the agent.
3.  **Agent Prompting**: Configuring the agent to know that skills exist and how to "invoke" them (by reading them).

## 1. Skill Definition Schema

A skill is a directory containing a definition file and supporting resources.

### Directory Structure
```
skills/
  ├── data-analysis/
  │   ├── SKILL.md        # The instruction manual
  │   └── scripts/        # Python/Bash scripts
  │       └── analyze.py
  └── git-workflow/
      ├── SKILL.md
      └── ...
```

### The `SKILL.md` Format
This file serves two purposes: metadata for the system (YAML frontmatter) and instructions for the agent (Markdown body).

```markdown
---
name: data-analysis
description: Analyze CSV/Excel files and generate summary statistics and charts.
---

# Data Analysis Instructions

To analyze a dataset:

1.  **Inspect**: First, read the first few lines of the file to understand the schema.
2.  **Execute**: Run the analysis script provided in this directory:
    `python {skill_path}/scripts/analyze.py --file <target_file>`
3.  **Report**: Summarize the output printed by the script for the user.
```

## 2. Discovery Implementation

The system must scan for skills and present them to the agent.

### Search Paths
Support hierarchical loading to allow project-specific overrides:
1.  `./.app/skills/` (Project specific)
2.  `~/.app/skills/` (User global)
3.  `/usr/local/share/app/skills/` (System defaults)

### The Index
The Discovery Engine parses the YAML frontmatter of every `SKILL.md` found and builds a lightweight index:

```json
[
  {
    "name": "data-analysis",
    "description": "Analyze CSV/Excel files...",
    "path": "/abs/path/to/skills/data-analysis"
  },
  {
    "name": "git-commit",
    "description": "Generate conventional commit messages...",
    "path": "/abs/path/to/skills/git-commit"
  }
]
```

## 3. The Agent Runtime Loop

Unlike traditional plugin systems where the host executes the code, here the **Agent** executes the skill.

### Step 1: System Prompting
Inject the Skill Index into the agent's system prompt. This gives the agent "awareness" of its capabilities.

> **System Prompt Addition:**
>
> You have access to the following Skills. To use a skill, you must first read its instruction file (`SKILL.md`) to learn how to operate it.
>
> - **data-analysis**: Analyze CSV/Excel files... (Path: `/skills/data-analysis`)
> - **git-commit**: Generate conventional commit messages... (Path: `/skills/git-commit`)
>
> If a user request matches a skill, use your `read_file` tool to read the `SKILL.md` file at the specified path. Then, follow the instructions inside.

### Step 2: Invocation (The "Handshake")
1.  **User**: "Analyze this sales.csv file."
2.  **Agent (Thought)**: "I see a `data-analysis` skill in my list. I need to know how to use it."
3.  **Agent (Action)**: Calls `read_file("/skills/data-analysis/SKILL.md")`.
4.  **System**: Returns the content of the markdown file.

### Step 3: Execution
The agent now has the instructions in its context.
1.  **Agent (Thought)**: "The instructions say to run `scripts/analyze.py`."
2.  **Agent (Action)**: Calls `execute_script("python /skills/data-analysis/scripts/analyze.py --file sales.csv")`.
3.  **System**: Runs the script and returns stdout/stderr.
4.  **Agent**: Interprets the results and answers the user.

## 4. Examples

### Example A: Code Style Auditor
**Goal**: Check code against strict corporate guidelines.

**`SKILL.md`**:
```markdown
---
name: style-audit
description: Check code against corporate style guidelines.
---
# Style Audit

1. Run `flake8` on the target file.
2. If errors are found, cross-reference them with `rules.txt` in this directory.
3. Suggest fixes using the `edit_file` tool.
```

### Example B: Database Migration Helper
**Goal**: Safely generate and apply SQL migrations.

**`SKILL.md`**:
```markdown
---
name: db-migrate
description: Generate and apply database migrations.
---
# Migration Helper

1. Inspect the `models.py` file.
2. Run `python {skill_path}/scripts/generate_migration.py`.
3. **Wait** for user confirmation before applying.
4. Run `alembic upgrade head`.
```

## 5. Implementation Guide (Python)

Here is a simple implementation of the Discovery Engine.

```python
import frontmatter
import glob
import os

def discover_skills(search_paths):
    skills = {}
    for path in search_paths:
        # Find all SKILL.md files
        for skill_file in glob.glob(os.path.join(path, "**/SKILL.md"), recursive=True):
            try:
                with open(skill_file, 'r') as f:
                    post = frontmatter.load(f)
                
                # Validate
                if 'name' not in post.metadata:
                    continue
                    
                # Store (overwriting lower priority if needed)
                skills[post.metadata['name']] = {
                    "name": post.metadata['name'],
                    "description": post.metadata.get('description', ''),
                    "path": os.path.abspath(os.path.dirname(skill_file)),
                    "definition_file": os.path.abspath(skill_file)
                }
            except Exception as e:
                print(f"Failed to load skill {skill_file}: {e}")
    return skills

def generate_system_prompt_addition(skills):
    prompt = "\n# Available Skills\n"
    prompt += "You can perform complex tasks by using the following skills. "
    prompt += "To use one, read the definition file first.\n\n"
    
    for name, data in skills.items():
        prompt += f"- **{name}**: {data['description']}\n"
        prompt += f"  Definition: {data['definition_file']}\n"
        
    return prompt
```

## 6. Skill Authoring Best Practices

When designing skills for agents, follow these principles to ensure reliability and efficiency.

### Concise is Key
The context window is a shared resource. Only add context the agent doesn't already have.
*   **Good**: "Use `pdfplumber` to extract text." (Assumes the agent knows Python/libraries)
*   **Bad**: "PDF is a file format... You need to install a library... Here is how to import it..."

### Progressive Disclosure
Don't load everything at once. Use the filesystem to your advantage.
*   **Pattern**: Keep `SKILL.md` as a high-level index.
*   **Implementation**: If a skill has complex documentation (e.g., an API reference), put it in `reference.md`. The `SKILL.md` should say: "For API details, read `reference.md`."
*   **Benefit**: The agent only pays the token cost for `reference.md` if it actually needs to read it.

### Utility Scripts vs. Generated Code
Prefer pre-written scripts over asking the agent to write code on the fly.
*   **Reliability**: A script like `scripts/validate_form.py` is deterministic. Asking the agent to "write a validation script" is probabilistic and prone to errors.
*   **Efficiency**: Executing a script consumes fewer tokens than generating, reading, and debugging new code.

### Feedback Loops
For complex tasks, design a "Plan-Validate-Execute" workflow.
*   **Example**:
    1.  **Analyze**: Agent runs `analyze.py` to understand the state.
    2.  **Plan**: Agent writes a `plan.json` file.
    3.  **Validate**: Agent runs `validate_plan.py` to check for errors.
    4.  **Execute**: Only if validation passes, agent runs `execute.py`.

### Naming Conventions
Use **gerund form** (verb + -ing) for skill names to clearly describe the capability.
*   **Good**: `processing-pdfs`, `analyzing-spreadsheets`
*   **Bad**: `helper`, `utils`, `data`

### Effective Descriptions
The `description` field in the frontmatter is critical. It is the *only* thing the agent sees before deciding to load the skill.
*   **Be Specific**: Include triggers and context.
*   **Third Person**: "Processes Excel files..." (not "I can help you...")
*   **Example**: "Generate descriptive commit messages by analyzing git diffs. Use when the user asks for help writing commit messages."

## Best Practices for System Design

1.  **Path Templating**: In your `SKILL.md`, use placeholders like `{skill_path}` so the agent knows where to find scripts relative to the skill definition.
2.  **Tool Availability**: Ensure the agent has the fundamental tools (`execute_script`, `read_file`) enabled. A skill is useless if the agent can't execute the instructions it reads.
3.  **Sandboxing**: Since the agent will be executing scripts found in these directories, ensure your `execute_script` tool runs in a safe environment (e.g., Docker container) if you are loading skills from untrusted sources.
