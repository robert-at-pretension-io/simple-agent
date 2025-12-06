---
name: background-manager
description: Manage long-running background processes (start, list, log, stop). Use when running servers, watchers, or heavy computations that should persist while you perform other tasks.
---

# Background Manager

## Overview

This skill enables you to run processes in the background, keeping them alive even if the agent's main session is interrupted or busy. You can list running processes, check their logs incrementally, and stop them when needed.

**Key Features:**
- **Detached Execution:** Processes run in a new session/process group.
- **State Persistence:** Keeps track of running processes across agent sessions (stored in `skills/background-manager/data/processes.json.enc`).
- **Log Management:** Automatically redirects stdout/stderr to log files.
- **Encryption:** **ALL** logs and state data are encrypted at rest using AES-256-CBC.
- **Interactivity:** Send input (commands/keypresses) to running processes via standard input.

## Security & Requirements

**IMPORTANT:** This skill requires a password to encrypt/decrypt data.
You must set the `BG_PASSWORD` environment variable before running any script.

```bash
export BG_PASSWORD="your-secure-password"
```

## Capabilities

### 1. Start a Process
Runs a command in the background.

```bash
python3 skills/background-manager/scripts/start_process.py <command> [args...]
```

**Example:**
```bash
python3 skills/background-manager/scripts/start_process.py python3 -m http.server 8080
```

### 2. List Processes
Shows all managed processes and their status (running, stopped, zombie).

```bash
python3 skills/background-manager/scripts/list_processes.py
```

**Output columns:**
- **ID**: Unique 8-char identifier.
- **PID**: System Process ID.
- **STATUS**: `running`, `stopped`, etc.
- **STARTED**: Start timestamp.
- **COMMAND**: The command being run.

### 3. Get Logs
Retrieves output from a background process.

```bash
python3 skills/background-manager/scripts/get_logs.py <id> [-n LINES]
```

- `<id>`: The process ID (or unique partial ID from `list_processes.py`).
- `-n LINES`: Number of lines to show from the end (default: 20). Set to 0 for all.

**Example:**
```bash
python3 skills/background-manager/scripts/get_logs.py a1b2 -n 50
```

### 4. Stop a Process
Terminates a background process and its process group (children).

```bash
python3 skills/background-manager/scripts/stop_process.py <id>
```

**Example:**
```bash
python3 skills/background-manager/scripts/stop_process.py a1b2
```

### 5. Send Input (Interactive)
Sends text input to the standard input (stdin) of a running process. Useful for interactive commands like REPLs, prompts, or confirmations.

```bash
python3 skills/background-manager/scripts/send_input.py <id> <text> [-n]
```

- `<id>`: The process ID.
- `<text>`: The string to send. By default, a newline is appended (simulating "Enter").
- `-n`: Do not append a newline.

**Example:**
```bash
python3 skills/background-manager/scripts/send_input.py a1b2 "yes"
```

## Storage

- **State File:** `skills/background-manager/data/processes.json.enc` (Encrypted)
- **Logs Directory:** `skills/background-manager/data/logs/` (*.log.enc)
