---
name: yolo-runner
description: Executes arbitrary CLI commands directly. Use for ad-hoc system interaction, complex shell chains, or tools without dedicated skills.
version: 1.0.0
---

# Direct Shell Runner

## Overview
This skill provides direct access to the system shell. It bridges the gap between structured, predefined skills and the fluid, unpredictable nature of real-world development environments.

Use this skill when:
1.  **Exploration**: You need to check file permissions, installed package versions, or disk usage (e.g., `ls -la`, `grep`, `df -h`).
2.  **Complex Chains**: You need to pipe output between commands or use redirection (e.g., `grep pattern file.log | sort | uniq -c > stats.txt`).
3.  **Missing Capabilities**: You need to use a tool (e.g., `curl`, `wget`, `zip`) that does not have a dedicated skill wrapper.
4.  **One-off Tasks**: Creating a full skill for a single command execution would be inefficient.

## Philosophy
While other skills encapsulate logic to ensure safety and repeatability, this skill empowers you to act as a system administrator.

- **Precision**: You are responsible for the correctness of the command.
- **Context**: Always verify the current working directory or use absolute paths.
- **Safety**: Although restrictions are lifted, you must still apply common sense. Do not delete critical system files or expose secrets.

## Instructions

### 1. Constructing Commands
When using `run_command.sh`, the entire command line is passed as a single string argument.

- **Escaping**: If your command involves complex quoting (e.g., `awk` scripts or JSON payloads), ensure internal quotes are escaped properly so the shell receives the correct string.
- **Chaining**: You can use standard shell operators (`&&`, `||`, `;`, `|`).

### 2. Output Handling
The standard output (stdout) and standard error (stderr) will be returned.
- If the output is expected to be massive (e.g., `cat large_file.log`), consider piping to `head` or `tail` first to avoid overwhelming the context window.

### 3. State Changes
If you change directories (`cd`), it only affects that specific script execution sub-shell. It does **not** persist for subsequent steps. To work in a specific directory, chain the cd command:
`cd /path/to/dir && ./script.sh`

## Scripts

### `run_command.sh`
Executes the provided string as a shell command using `eval`.

**Usage**: `scripts/run_command.sh "<command_string>"`

**Example**:
`scripts/run_command.sh "find . -name '*.log' -mtime +7 -delete"`
