# Installation Guide

## Prerequisites (Source Install Only)
If you plan to build from source (Option 2), you need:
- **Go**: Version 1.21 or higher
  - **macOS**: `brew install go`
  - **Windows**: `winget install GoLang.Go`
  - **Linux**: `sudo snap install go --classic`
  - **Manual**: [Download from go.dev](https://go.dev/dl/)
- **Git**: [Download Git](https://git-scm.com/downloads)

## Installation

### Option 1: Quick Install Script (Linux/macOS)
Does not require Go. Downloads the latest binary for your system.

```bash
curl -sL https://raw.githubusercontent.com/robert-at-pretension-io/simple-agent/main/install.sh | sh
```

### Option 2: Install from Source
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
