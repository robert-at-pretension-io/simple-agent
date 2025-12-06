
## License# Gemini REPL

A simple REPL for interacting with Gemini 3.0 Pro Preview using the OpenAI-compatible REST API.

## Prerequisites

- Go 1.21 or later
- A Gemini API Key from [Google AI Studio](https://aistudio.google.com/)

## How to Run

1.  **Set your API Key:**

    On Linux/macOS:
    ```bash
    export GEMINI_API_KEY="your_api_key_here"
    ```

    On Windows (Command Prompt):
    ```cmd
    set GEMINI_API_KEY=your_api_key_here
    ```

    On Windows (PowerShell):
    ```powershell
    $env:GEMINI_API_KEY="your_api_key_here"
    ```

2.  **Run the application:**

    ```bash
    go run main.go
    ```

## Usage

- Type your message at the `> ` prompt and press Enter.
- The model can reply with text or call the `apply_udiff` tool to modify files in the current directory.
- Press `Ctrl+C` to exit.

## Versioning

This project follows semantic versioning. The current version is `v1.1.3`.
To check the installed version, run:
```bash
simple-agent --version
```

## Project Goals

- **Maintain a Changelog**: All version changes will be documented to ensure transparency and easy upgrades.
- **Robust Installation**: Ensure seamless installation via `go install` across different environments.

## Configuration

- **Auto-Accept Diffs**: By default, the agent automatically accepts proposed file changes. To require manual confirmation, use the `--no-auto-accept` flag.
- **Auto-Update**: The agent checks for updates on startup. Use `--no-update` to disable.
