# Installation Guide

## Prerequisites

- **Go**: Version 1.21 or higher. [Download Go](https://go.dev/dl/)
- **Git**: [Download Git](https://git-scm.com/downloads)

## Installation

You can install `simple-agent` directly from the source using the `go install` command:

```bash
go install github.com/robert-at-pretension-io/simple-agent@latest
```

Ensure that your Go bin directory (usually `$HOME/go/bin`) is in your system's `PATH`.

## Configuration

The agent requires a Google Gemini API key to function.

1.  **Get an API Key**: Visit [Google AI Studio](https://aistudio.google.com/) to create a key.
2.  **Set the Environment Variable**:

    **Bash/Zsh (Linux/macOS):**
    ```bash
    export GEMINI_API_KEY="your_api_key_here"
    ```

    **PowerShell (Windows):**
    ```powershell
    $env:GEMINI_API_KEY="your_api_key_here"
    ```

## Verification

To verify the installation, run:

```bash
simple-agent --help
```
