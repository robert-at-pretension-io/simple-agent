
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
