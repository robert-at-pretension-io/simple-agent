package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

//go:embed skills
var embeddedSkillsFS embed.FS

//go:embed install.sh
var installScript []byte

var CoreSkillsDir string

const (
	Version        = "v1.1.40"
	GeminiURL      = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	ModelName      = "gemini-3-pro-preview"
	FlashModelName = "gemini-2.5-flash"
)

// --- API Structures ---

type ChatCompletionRequest struct {
	Model     string          `json:"model"`
	Messages  []Message       `json:"messages"`
	Tools     []Tool          `json:"tools,omitempty"`
	ExtraBody json.RawMessage `json:"extra_body,omitempty"`
}

type Message struct {
	Role         string          `json:"role"`
	Content      string          `json:"content"`
	ToolCalls    []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID   string          `json:"tool_call_id,omitempty"`
	ExtraContent json.RawMessage `json:"extra_content,omitempty"`
}

type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type ToolCall struct {
	ID           string           `json:"id"`
	Type         string           `json:"type"`
	Function     ToolCallFunction `json:"function"`
	ExtraContent json.RawMessage  `json:"extra_content,omitempty"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionResponse struct {
	Choices []Choice  `json:"choices"`
	Error   *APIError `json:"error,omitempty"`
	Usage   *Usage    `json:"usage,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type APIError struct {
	Message string `json:"message"`
	Code    any    `json:"code"`
}

type Choice struct {
	Message Message `json:"message"`
}

// --- Tool Definitions ---

var udiffTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "apply_udiff",
		Description: "Apply a unified diff to a file. The diff should be in standard unified format (diff -U0), including headers. IMPORTANT: Context lines are mandatory for insertions. You must include at least 2 lines of context around your changes. A hunk with only '+' lines is invalid (unless creating a new file). Ensure enough context is provided to uniquely locate the code.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The file path to modify"
				},
				"diff": {
					"type": "string",
					"description": "The unified diff content. Must include @@ ... @@ headers for hunks. Must include context lines."
				}
			},
			"required": ["path", "diff"]
		}`),
	},
}

var runScriptTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "run_script",
		Description: "Execute a shell script (.sh) from a skill. This is the PRIMARY way to execute code and interact with the OS. You use this to run the 'yolo-runner' skill for arbitrary shell commands, or other specialized skill scripts.",
		Parameters: json.RawMessage(`{
	"type": "object",
	"properties": {
		"path": {
			"type": "string",
			"description": "The file path to the script. It MUST start with 'skills/' and contain '/scripts/' (e.g., 'skills/todo-manager/scripts/scan.sh')."
		},
		"args": {
			"type": "array",
			"items": {
				"type": "string"
			},
			"description": "Arguments to pass to the script"
		}
	},
	"required": ["path"]
		}`),
	},
}


var shortenContextTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "shorten_context",
		Description: "Summarize and shorten the conversation context based on the current task and vital information. This resets the conversation history with the summary.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"task_description": {
					"type": "string",
					"description": "Description of the current task being worked on."
				},
				"future_plans": {
					"type": "string",
					"description": "What needs to be done in the future of this session."
				},
				"vital_information": {
					"type": "string",
					"description": "Specific information, constraints, or code snippets that must be preserved verbatim."
				}
			},
			"required": ["task_description", "future_plans", "vital_information"]
		}`),
	},
}

// --- Skills System ---

type Skill struct {
	Name           string
	Description    string
	Version        string
	Dependencies   []string
	Path           string
	DefinitionFile string
	Hooks          map[string]string
	Scripts        []string
}

// var supportedHooks = []string{"startup", "pre_edit", "post_edit", "pre_view", "post_view", "pre_run", "post_run", "pre_commit"}

func getSkillsExplanation() string {
	return `
# Skills System Philosophy

You have the ability to discover and use "Skills". Skills are specialized capabilities defined in files within the 'skills' directory.

## Purpose
Skills bridge the gap between general reasoning and specific, repeatable tasks. They allow you to:
1.  **Extend Capabilities**: Learn new workflows (e.g., "deploy to AWS", "audit code") without core updates.
2.  **Encapsulate Logic**: Hide complex details in scripts and instructions.
3.  **Autonomy**: You read the "manual" (SKILL.md) and drive execution.

## Skill Structure
A skill is a directory (e.g., ` + "`skills/my-skill/`" + `) containing:
1.  **` + "`SKILL.md`" + `**: The instruction manual.
    - Must start with YAML frontmatter defining ` + "`name`" + ` and ` + "`description`" + `.
    - The body contains Markdown instructions for you to follow.
    - **Hooks (Recommended)**: Automate workflows by triggering scripts on system events.
      Define them in the frontmatter under a ` + "`hooks`" + ` section.
      **Supported Hooks**:
      - ` + "`startup`" + `: Runs at session start (e.g., dependency checks).
      - ` + "`pre_edit` / `post_edit`" + `: Runs before/after ` + "`apply_udiff`" + `. **Great for running linters/tests automatically.**
      - ` + "`pre_run` / `post_run`" + `: Runs before/after ` + "`run_script`" + `.
      - ` + "`pre_commit`" + `: Runs before the agent proposes a git commit.
      **Example**:
      hooks:
        post_edit: scripts/lint.sh
        startup: scripts/check_deps.sh
2.  **` + "`scripts/`" + `** (Optional): A subdirectory for utility scripts.
    - **Multiple Scripts**: You can include multiple scripts for different sub-tasks (e.g., ` + "`setup.sh`" + `, ` + "`validate.py`" + `).
    - **Descriptive Names**: Give scripts clear, action-oriented names (e.g., ` + "`install_dependencies.sh`" + ` is better than ` + "`run.sh`" + `).
    - **Invocation**: Scripts are invoked via ` + "`sh -c [path] [args]`" + `. Prefer scripts over complex manual steps in ` + "`SKILL.md`" + `.

## How to Invoke Skills
1.  **Discover**: The system provides a list of available skills.
2.  **Learn**: If a user request matches a skill, read its 'SKILL.md' (e.g. using 'yolo-runner').
3.  **Execute**: Follow the instructions in 'SKILL.md'.
    - If the instructions refer to scripts, execute them using 'run_script'.
    - Scripts are typically located relative to the skill directory (e.g., ` + "`skills/my-skill/scripts/script.sh`" + `).

## Creating and Managing Skills
You can also create new skills to solve problems!
1.  **Create Directory**: Create a new folder in ` + "`skills/`" + `.
2.  **Define Skill**: Create ` + "`SKILL.md`" + ` with frontmatter and instructions.
3.  **Add Scripts**: Create a ` + "`scripts/`" + ` folder.
    - **Organize**: Split complex logic into multiple, focused scripts.
    - **Naming**: Use descriptive names (e.g., ` + "`migrate_db.sh`" + `) to make the skill easier to understand and debug.

**Best Practices**:
- **Specific vs. General**: Create specific skills for complex, recurring problems. However, prefer general skills that can be reused.
- **Auditing**: If you find too many specific skills cluttering the system, suggest consolidating them or removing obsolete ones.
- **Concise**: Only add necessary context in ` + "`SKILL.md`" + `.
- **Self-Contained**: A skill should include everything needed to run it.
- **Automate with Hooks**: Whenever possible, use hooks to run validation (linting, testing) automatically rather than writing manual instructions.
- **Invocation**: Hooks must specify a script path (relative to the skill directory) and arguments.
- **Extended Resources**: If a skill needs long prompts, templates, or static data, store them in files within the skill directory and read them as needed.
- **Protective & Defensive**: **Do not assume** the user has specific system tools (like ` + "`jq`" + `, ` + "`aws`" + `, ` + "`npm`" + `) installed. Check for them or use standard, widely available tools.
- **Project Agnostic**: **Do not assume** the project uses a specific language (e.g., Go, TS). Dynamically detect the environment (e.g., check for ` + "`go.mod`" + ` vs ` + "`package.json`" + `) before executing language-specific logic.

When faced with a new, complex task that might be repeated, consider creating a new skill for it.
`
}

func discoverSkills(root string) []Skill {
	var skills []Skill
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return skills
	}

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == "SKILL.md" {
			skill, err := parseSkill(path)
			if err == nil {
				skills = append(skills, skill)
			}
		}
		return nil
	})
	return skills
}

func parseSkill(path string) (Skill, error) {
	f, err := os.Open(path)
	if err != nil {
		return Skill{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var name, description, version string
	var dependencies []string
	hooks := make(map[string]string)
	inFrontmatter := false
	inHooks := false
	inDependencies := false
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		trimmedLine := strings.TrimSpace(line)

		if line == "---" {
			if lineCount == 0 {
				inFrontmatter = true
				lineCount++
				continue
			}
			if inFrontmatter {
				break // End of frontmatter
			}
		}

		if inFrontmatter {
			if trimmedLine == "hooks:" {
				inHooks = true
				inDependencies = false
				continue
			}
			if trimmedLine == "dependencies:" {
				inDependencies = true
				inHooks = false
				continue
			}

			if inHooks {
				// Check if we are still in hooks (indented)
				if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
					parts := strings.SplitN(trimmedLine, ":", 2)
					if len(parts) == 2 {
						key := strings.TrimSpace(parts[0])
						val := strings.TrimSpace(parts[1])
						hooks[key] = val
					}
				} else if trimmedLine != "" {
					// Not empty and not indented, so we left hooks
					inHooks = false
				}
			}

			if inDependencies {
				if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
					val := strings.TrimSpace(trimmedLine)
					val = strings.TrimPrefix(val, "-")
					val = strings.TrimSpace(val)
					if val != "" {
						dependencies = append(dependencies, val)
					}
				} else if trimmedLine != "" {
					inDependencies = false
				}
			}

			if !inHooks && !inDependencies {
				if strings.HasPrefix(line, "name:") {
					name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				} else if strings.HasPrefix(line, "description:") {
					description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "version:") {
					version = strings.TrimSpace(strings.TrimPrefix(line, "version:"))
				}
			}
		}
		lineCount++
	}

	if name == "" {
		return Skill{}, fmt.Errorf("no name found in frontmatter")
	}

	absPath, _ := filepath.Abs(filepath.Dir(path))
	defFile, _ := filepath.Abs(path)

	var scripts []string
	scriptsDir := filepath.Join(filepath.Dir(path), "scripts")
	if _, err := os.Stat(scriptsDir); err == nil {
		filepath.WalkDir(scriptsDir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			scripts = append(scripts, p)
			return nil
		})
	}

	return Skill{
		Name:           name,
		Description:    description,
		Version:        version,
		Dependencies:   dependencies,
		Path:           absPath,
		DefinitionFile: defFile,
		Hooks:          hooks,
		Scripts:        scripts,
	}, nil
}

func generateSkillsPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n# Available Skills\n")
	sb.WriteString("You can perform complex tasks by using the following skills.\n")
	sb.WriteString("To use one, read the definition file first (e.g. using 'yolo-runner').\n\n")

	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**", s.Name))
		if s.Version != "" {
			sb.WriteString(fmt.Sprintf(" (v%s)", s.Version))
		}
		sb.WriteString(fmt.Sprintf(": %s\n", s.Description))
		if len(s.Dependencies) > 0 {
			sb.WriteString(fmt.Sprintf("  Dependencies: %s\n", strings.Join(s.Dependencies, ", ")))
		}
		if len(s.Scripts) > 0 {
			sb.WriteString("  Scripts:\n")
			for _, script := range s.Scripts {
				sb.WriteString(fmt.Sprintf("    - %s\n", script))
			}
		}
		sb.WriteString(fmt.Sprintf("  Definition: %s\n", s.DefinitionFile))

		if s.Name == "yolo-runner" {
			sb.WriteString("\n  **AUTONOMY MODE**: You have the 'yolo-runner' skill. Use it to run ANY shell command needed to complete your task. You are authorized to take initiative.\n")
		}
		if s.Name == "remember" {
			sb.WriteString("\n  **PROJECT MEMORY**: Use the 'remember' skill to persist important facts, decisions, and context. Read 'remember.txt' at the start of tasks and update it as you learn.\n")
		}
	}
	return sb.String()
}

func runSkillHooks(ctx context.Context, skills []Skill, event string, context map[string]string) string {
	var output strings.Builder
	for _, skill := range skills {
		if cmdTemplate, ok := skill.Hooks[event]; ok {
			// Special hook type: inject_skill_md
			if cmdTemplate == "inject_skill_md" {
				body, err := readSkillBody(skill.DefinitionFile)
				if err != nil {
					fmt.Printf("[Hook Error] Failed to read skill body for '%s': %v\n", skill.Name, err)
					continue
				}
				output.WriteString(fmt.Sprintf("\n[Skill: %s Instructions]\n%s\n", skill.Name, body))
				continue
			}

			// Prepare command
			cmdStr := cmdTemplate
			// Replace {skill_path}
			cmdStr = strings.ReplaceAll(cmdStr, "{skill_path}", skill.Path)
			// Replace context variables
			for k, v := range context {
				cmdStr = strings.ReplaceAll(cmdStr, "{"+k+"}", v)
			}

			// Parse command string into script path and args
			parts, err := parseArgs(cmdStr)
			if err != nil {
				fmt.Printf("[Hook Error] Failed to parse command '%s' for skill '%s': %v\n", cmdStr, skill.Name, err)
				continue
			}
			if len(parts) == 0 {
				continue
			}
			scriptPath := parts[0]
			args := parts[1:]

			// Resolve relative paths to skill directory
			if !filepath.IsAbs(scriptPath) {
				scriptPath = filepath.Join(skill.Path, scriptPath)
			}

			fmt.Printf("[Hook: %s] Running for skill '%s': %s %v\n", event, skill.Name, scriptPath, args)

			// Use runSafeScript to enforce security and execution logic
			out, err := runSafeScript(ctx, scriptPath, args, "")
			if err != nil {
				fmt.Printf("[Hook Error] %v\n", err)
				output.WriteString(fmt.Sprintf("Hook '%s' (skill: %s) failed: %v\n", event, skill.Name, err))
			} else if out != "" {
				output.WriteString(fmt.Sprintf("Hook '%s' (skill: %s) output:\n%s\n", event, skill.Name, out))
			}
		}
	}
	return output.String()
}

func readSkillBody(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := string(data)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if strings.HasPrefix(s, "---") {
		parts := strings.SplitN(s, "---", 3)
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[2]), nil
		}
	}
	return s, nil
}

// restoreTerminal restores the terminal to canonical mode and echo.
func restoreTerminal() {
	cmd := exec.Command("stty", "icanon", "echo", "isig")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// readInteractiveInput reads input in raw mode to support arrow keys and multi-line editing.
// It handles basic line wrapping and cursor movement.
func readInteractiveInput(reader *bufio.Reader, history []string) (string, error) {
	// Attempt to set raw mode
	cmd := exec.Command("stty", "-icanon", "-echo", "-isig")
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		// Fallback for non-POSIX or error: use the provided reader
		return reader.ReadString('\n')
	}
	defer restoreTerminal()

	var buf []rune
	cursor := 0
	currentVisualRow := 0 // Track cursor row relative to prompt start
	historyIndex := len(history)
	var currentInputDraft []rune
	var lastCtrlC time.Time

	isFirstLine := func() bool {
		for i := cursor - 1; i >= 0; i-- {
			if buf[i] == '\n' {
				return false
			}
		}
		return true
	}

	isLastLine := func() bool {
		for i := cursor; i < len(buf); i++ {
			if buf[i] == '\n' {
				return false
			}
		}
		return true
	}

	redraw := func() {
		width := getTermWidth()

		fmt.Print("\033[?25l") // Hide cursor

		// 1. Move cursor to start of the prompt (based on previous state)
		if currentVisualRow > 0 {
			fmt.Printf("\033[%dA", currentVisualRow)
		}
		fmt.Print("\r")

		// 2. Clear everything from cursor down
		fmt.Print("\033[J")

		// 3. Print prompt and buffer
		prompt := "\033[1;32mUser 👤\033[0m > "
		fmt.Print(prompt + string(buf))

		// 4. Calculate where the cursor IS now (end of print) vs where it SHOULD be
		// End position (where cursor is left after print)
		// Note: Prompt length is visually different from string length due to ANSI codes.
		// The prompt "> " is 2 chars. "User 👤 > " is 10 visual chars (User + space + emoji + space + > + space).
		// Let's approximate visual length as 10 (Emoji is usually wide).
		visualPromptLen := 10
		endRow, _ := getCursorVisualPos(buf, len(buf), width, visualPromptLen)

		// Target position (where cursor should be)
		targetRow, targetCol := getCursorVisualPos(buf, cursor, width, visualPromptLen)

		// 5. Move cursor to target
		// We are currently at endRow, endCol (implicit)
		// To get to target, we move up (endRow - targetRow)
		up := endRow - targetRow
		if up > 0 {
			fmt.Printf("\033[%dA", up)
		}
		fmt.Print("\r") // Move to start of the target row
		if targetCol > 0 {
			fmt.Printf("\033[%dC", targetCol)
		}

		// Update state
		currentVisualRow = targetRow

		fmt.Print("\033[?25h") // Show cursor
	}

	bufRead := make([]byte, 12)

	for {
		n, err := os.Stdin.Read(bufRead)
		if err != nil {
			return "", err
		}

		// Parse input
		s := string(bufRead[:n])

		if s == "\x03" { // Ctrl+C
			if len(buf) > 0 {
				fmt.Println("^C")
				buf = []rune{}
				cursor = 0
				currentVisualRow = 0
				redraw()
				continue
			}
			if time.Since(lastCtrlC) < 1*time.Second {
				return "", fmt.Errorf("interrupted")
			}
			lastCtrlC = time.Now()
			fmt.Println("^C\n(Press Ctrl+C again to exit)")
			currentVisualRow = 0
			redraw()
			continue
		} else if s == "\x04" { // Ctrl+D
			if len(buf) == 0 {
				return "", io.EOF
			}
			fmt.Println()
			return string(buf), nil
		} else if s == "\r" || s == "\n" {
			buf = append(buf[:cursor], append([]rune{'\n'}, buf[cursor:]...)...)
			cursor++
			if historyIndex == len(history) {
				currentInputDraft = buf
			}
		} else if s == "\x7f" { // Backspace
			if cursor > 0 {
				buf = append(buf[:cursor-1], buf[cursor:]...)
				cursor--
			}
			if historyIndex == len(history) {
				currentInputDraft = buf
			}
		} else if s == "\x17" || s == "\x1b\x7f" { // Ctrl+W or Alt+Backspace
			// Delete word backwards
			oldCursor := cursor
			for cursor > 0 && unicode.IsSpace(buf[cursor-1]) {
				cursor--
			}
			for cursor > 0 && !unicode.IsSpace(buf[cursor-1]) {
				cursor--
			}
			buf = append(buf[:cursor], buf[oldCursor:]...)
			if historyIndex == len(history) {
				currentInputDraft = buf
			}
		} else if s == "\x01" || s == "\x1b[H" || s == "\x1b[1~" || s == "\x1bOH" { // Ctrl+A or Home
			for cursor > 0 && buf[cursor-1] != '\n' {
				cursor--
			}
		} else if s == "\x05" || s == "\x1b[F" || s == "\x1b[4~" || s == "\x1bOF" { // Ctrl+E or End
			for cursor < len(buf) && buf[cursor] != '\n' {
				cursor++
			}
		} else if s == "\x1b[1;5H" { // Ctrl+Home
			cursor = 0
		} else if s == "\x1b[1;5F" { // Ctrl+End
			cursor = len(buf)
		} else if s == "\x15" { // Ctrl+U
			// Clear from cursor to start of line
			start := cursor
			for start > 0 && buf[start-1] != '\n' {
				start--
			}
			buf = append(buf[:start], buf[cursor:]...)
			cursor = start
			if historyIndex == len(history) {
				currentInputDraft = buf
			}
		} else if s == "\x0b" { // Ctrl+K
			// Clear from cursor to end of line
			end := cursor
			for end < len(buf) && buf[end] != '\n' {
				end++
			}
			buf = append(buf[:cursor], buf[end:]...)
			if historyIndex == len(history) {
				currentInputDraft = buf
			}
		} else if s == "\x1b[3~" { // Delete
			if cursor < len(buf) {
				buf = append(buf[:cursor], buf[cursor+1:]...)
				if historyIndex == len(history) {
					currentInputDraft = buf
				}
			}
		} else if s == "\x0c" { // Ctrl+L
			fmt.Print("\033[H\033[2J")
			currentVisualRow = 0
		} else if strings.HasPrefix(s, "\x1b") { // Escape sequence
			if s == "\x1b[D" { // Left
				if cursor > 0 {
					cursor--
				}
			} else if s == "\x1b[C" { // Right
				if cursor < len(buf) {
					cursor++
				}
			} else if s == "\x1b[A" { // Up (Previous line)
				if isFirstLine() {
					if historyIndex > 0 {
						if historyIndex == len(history) {
							currentInputDraft = make([]rune, len(buf))
							copy(currentInputDraft, buf)
						}
						historyIndex--
						buf = []rune(history[historyIndex])
						cursor = len(buf)
					}
				} else {
					// Find start of current line
					lineStart := cursor
					for lineStart > 0 && buf[lineStart-1] != '\n' {
						lineStart--
					}
					col := cursor - lineStart

					// Find start of previous line
					if lineStart > 0 {
						prevLineEnd := lineStart - 1
						prevLineStart := prevLineEnd
						for prevLineStart > 0 && buf[prevLineStart-1] != '\n' {
							prevLineStart--
						}

						newCursor := prevLineStart + col
						if newCursor > prevLineEnd {
							newCursor = prevLineEnd
						}
						cursor = newCursor
					}
				}
			} else if s == "\x1b[B" { // Down
				if isLastLine() {
					if historyIndex < len(history) {
						historyIndex++
						if historyIndex == len(history) {
							buf = make([]rune, len(currentInputDraft))
							copy(buf, currentInputDraft)
						} else {
							buf = []rune(history[historyIndex])
						}
						cursor = len(buf)
					}
				} else {
					// Find start of current line
					lineStart := cursor
					for lineStart > 0 && buf[lineStart-1] != '\n' {
						lineStart--
					}
					col := cursor - lineStart

					// Find end of current line
					lineEnd := cursor
					for lineEnd < len(buf) && buf[lineEnd] != '\n' {
						lineEnd++
					}

					if lineEnd < len(buf) { // Next line exists
						nextLineStart := lineEnd + 1
						nextLineEnd := nextLineStart
						for nextLineEnd < len(buf) && buf[nextLineEnd] != '\n' {
							nextLineEnd++
						}

						newCursor := nextLineStart + col
						if newCursor > nextLineEnd {
							newCursor = nextLineEnd
						}
						cursor = newCursor
					}
				}
			} else if s == "\x1b[1;5D" || s == "\x1b\x1b[D" || s == "\x1bb" { // Ctrl-Left or Alt-B
				// Move left until space
				for cursor > 0 && unicode.IsSpace(buf[cursor-1]) {
					cursor--
				}
				for cursor > 0 && !unicode.IsSpace(buf[cursor-1]) {
					cursor--
				}
			} else if s == "\x1b[1;5C" || s == "\x1b\x1b[C" || s == "\x1bf" { // Ctrl-Right or Alt-F
				// Move right until space
				for cursor < len(buf) && !unicode.IsSpace(buf[cursor]) {
					cursor++
				}
				for cursor < len(buf) && unicode.IsSpace(buf[cursor]) {
					cursor++
				}
			}
		} else {
			// Print chars
			runes := []rune(s)
			for _, r := range runes {
				if unicode.IsPrint(r) {
					buf = append(buf[:cursor], append([]rune{r}, buf[cursor:]...)...)
					cursor++
					if historyIndex == len(history) {
						currentInputDraft = buf
					}
				}
			}
		}
		redraw()
	}
}

func getTermWidth() int {
	cmd := exec.Command("tput", "cols")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	if err != nil {
		return 80 // Default fallback
	}
	w, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 80
	}
	return w
}

func getCursorVisualPos(buf []rune, pos int, width int, promptLen int) (int, int) {
	x := promptLen
	y := 0

	for i := 0; i < pos && i < len(buf); i++ {
		if buf[i] == '\n' {
			y++
			x = 0
		} else {
			x++
			if x >= width {
				x = 0
				y++
			}
		}
	}
	return y, x
}

// --- Main ---

func main() {
	versionFlag := flag.Bool("version", false, "Print version and exit")
	noUpdate := flag.Bool("no-update", false, "Skip auto-update check at startup")
	noAutoAccept := flag.Bool("no-auto-accept", false, "Disable automatic acceptance of diffs (require user confirmation)")
	continueSession := flag.Bool("continue", false, "Continue from previous session history")
	gitAutoCommit := flag.Bool("git-auto-commit", false, "Automatically propose commits for file changes after every turn")
	gitForceCommit := flag.Bool("git-force-commit", false, "Automatically commit changes without confirmation (implies -git-auto-commit)")
	flag.Parse()

	// Print version on startup
	fmt.Printf("Simple Agent %s\n", Version)

	// Default behavior is to auto-accept unless explicitly disabled
	shouldAutoApprove := !*noAutoAccept
	autoApprove := &shouldAutoApprove

	if *versionFlag {
		os.Exit(0)
	}

	if !*noUpdate && !*versionFlag {
		autoUpdate()
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	// Setup Core Skills (Extract embedded)
	if err := setupCoreSkills(); err != nil {
		fmt.Printf("Warning: Failed to extract core skills: %v\n", err)
	}

	// Discover skills
	// 1. Core Skills
	coreSkills := discoverSkills(CoreSkillsDir)
	// 2. Project Skills (Current Directory)
	projectSkills := discoverSkills("./skills")

	// Merge skills (Project overrides Core)
	skillMap := make(map[string]Skill)
	for _, s := range coreSkills {
		skillMap[s.Name] = s
	}
	for _, s := range projectSkills {
		skillMap[s.Name] = s
	}

	// Convert back to slice
	var skills []Skill
	for _, s := range skillMap {
		skills = append(skills, s)
	}

	skillsPrompt := generateSkillsPrompt(skills)

	// Track known skills to detect additions
	knownSkills := make(map[string]bool)
	for _, s := range skills {
		knownSkills[s.Name] = true
	}

	// Setup signal handling for interruption
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	var currentCancel context.CancelFunc
	var mu sync.Mutex

	var lastSignalTime time.Time

	go func() {
		for range sigChan {
			mu.Lock()
			if currentCancel != nil {
				fmt.Println("\n[Interrupted by user]")
				currentCancel()
				currentCancel = nil
			} else {
				if time.Since(lastSignalTime) < 1*time.Second {
					restoreTerminal()
					fmt.Println("\nExiting...")
					os.Exit(0)
				}
				lastSignalTime = time.Now()
				fmt.Println("\n(Press Ctrl+C again to exit)")
			}
			mu.Unlock()
		}
	}()

	// Run startup hooks (using background context as this is init)
	startupOutput := runSkillHooks(context.Background(), skills, "startup", nil)

	baseSystemPrompt := `You have access to tools to edit files and execute scripts (providing full shell access).
When using 'apply_udiff', provide a unified diff.
- Start hunks with '@@ ... @@'
- Use ' ' for context, '-' for removal, '+' for addition.
- **ALWAYS** include at least 2 lines of context around your changes.
- **Context is MANDATORY**: When inserting code, you must include existing lines around the insertion point. A hunk with only '+' lines is invalid (unless creating a new file).
- **How to Include Context**:
  1.  **Identify the Target**: Find the code you want to change and 2-3 lines of stable code above and below it.
  2.  **Copy Verbatim**: Copy the surrounding lines EXACTLY as they appear in the file.
  3.  **Prefix with Space**: Add a single space ' ' to the beginning of these context lines.
  4.  **Combine**: Surround your '-' (removal) and '+' (addition) lines with these ' ' (context) lines.
- **COMMON ISSUE**: The most frequent cause of failure is insufficient or mismatched context. Provide ample, unique context lines (more than 2 if needed) to ensure the patch applies correctly.
- Do not include line numbers in the hunk header.
- Ensure enough context is provided to uniquely locate the code.
- Replace entire blocks/functions rather than small internal edits to ensure uniqueness.
- If a file does not exist, treat it as empty for the 'before' state.
- **CLI PREFERENCE**: You are encouraged to use the CLI for efficiency and exploration.
- Use 'ls -R', 'grep', or 'find' to explore the file structure and search for patterns.
- **GATHER CONTEXT**: When using 'grep' to find code to edit, ALWAYS use context flags (e.g., 'grep -C 5'). You need ample unique context lines to ensure 'apply_udiff' can locate the target code unambiguously.
- Use 'cat', 'head', or 'tail' to quickly inspect file contents.
- Run standard tools (git, go, npm, etc.) directly when needed.
- Prefer shell commands for operations that are concise and standard.
- **CONTEXT MANAGEMENT**: Use 'shorten_context' to keep the session focused and save tokens.
- **When to Reset**: 
    - ONLY after completing a distinct task or sub-task.
    - Before starting a new, unrelated activity.
    - **AVOID** resetting if the user is building context (e.g., exploring files, reading docs) for an upcoming task. Wait for a definitive stopping point.
- **Goal**: Maintain a clean, concise state with only vital information for the next steps.
- **PROJECT MEMORY**:
    - **remember.txt**: This file is your long-term memory. It contains architectural decisions, current status, and lessons learned.
    - **Read First**: Always read 'remember.txt' when starting a task to ground yourself in the project context.
    - **Update Always**: Actively maintain this file. If you make a decision or learn something, add it to 'remember.txt' immediately.
    - **Use the Skill**: Use the 'remember' skill tools (or standard file tools) to curate this file.
`
	systemPrompt := baseSystemPrompt + getSkillsExplanation() + skillsPrompt

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	if startupOutput != "" {
		messages = append(messages, Message{Role: "system", Content: "Startup Instructions:\n" + startupOutput})
	}

	// Load history
	if *continueSession {
		savedMessages := loadHistory()
		if len(savedMessages) > 0 {
			for _, m := range savedMessages {
				if m.Role != "system" {
					messages = append(messages, m)
				}
			}
			fmt.Printf("Loaded %d messages from history.\n", len(messages)-1)
		}
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Welcome to Simple Agent %s (Model: %s)\n", Version, ModelName)
	if len(skills) > 0 {
		fmt.Printf("Loaded %d skills from ./skills\n", len(skills))
	}
	fmt.Println("Type your message. Press Ctrl+D (or Ctrl+Z on Windows) on a new line to send. Type /help for commands (e.g. /clear). Ctrl+C to interrupt/exit.")

	client := &http.Client{}

	var pendingInput string
	var commandHistory []string

	for {
		var input string
		if pendingInput != "" {
			fmt.Printf("> %s\n", pendingInput)
			input = pendingInput
			pendingInput = ""
		} else {
			fmt.Print("\033[1;32mUser 👤\033[0m > ")
			var err error
			input, err = readInteractiveInput(reader, commandHistory)
			if err != nil {
				if err == io.EOF {
					break
				}
				if err.Error() == "interrupted" {
					restoreTerminal()
					fmt.Println("Exiting...")
					os.Exit(0)
				}
				fmt.Printf("Error reading input: %v\n", err)
				os.Exit(1)
			}
			if strings.TrimSpace(input) == "" {
				continue
			}
			commandHistory = append(commandHistory, input)

			if handleSlashCommand(input, &messages, skills, systemPrompt, apiKey) {
				continue
			}
		}

		// Capture the start index of the current turn's messages
		startHistoryIndex := len(messages)

		messages = append(messages, Message{
			Role:    "user",
			Content: input,
		})

		// Start of turn: Create context and register cancel function
		ctx, cancel := context.WithCancel(context.Background())
		mu.Lock()
		currentCancel = cancel
		mu.Unlock()

		var lastUsage int

		// Interaction loop (handle tool calls)
		for {
			if ctx.Err() != nil {
				break
			}

			reqBody := ChatCompletionRequest{
				Model:     ModelName,
				Messages:  messages,
				Tools:     []Tool{udiffTool, runScriptTool, shortenContextTool},
				ExtraBody: json.RawMessage(`{"google": {"thinking_config": {"include_thoughts": true}}}`),
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				fmt.Printf("Error marshaling request: %v\n", err)
				break
			}

			var resp *http.Response
			var body []byte
			maxRetries := 7
			retryDelay := 2 * time.Second

			for attempt := 0; attempt <= maxRetries; attempt++ {
				if attempt > 0 {
					fmt.Printf("Retrying in %v... (Attempt %d/%d)\n", retryDelay, attempt, maxRetries)
					select {
					case <-ctx.Done():
						break
					case <-time.After(retryDelay):
						retryDelay *= 2
					}
				}

				req, err := http.NewRequestWithContext(ctx, "POST", GeminiURL, bytes.NewBuffer(jsonData))
				if err != nil {
					fmt.Printf("Error creating request: %v\n", err)
					break
				}

				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+apiKey)

				spinnerStop := make(chan struct{})
				spinnerDone := make(chan struct{})
				go startSpinner(spinnerStop, spinnerDone)

				resp, err = client.Do(req)

				close(spinnerStop)
				<-spinnerDone

				if err != nil {
					if ctx.Err() == context.Canceled {
						fmt.Println("\nRequest canceled.")
						break
					}
					fmt.Printf("Error sending request: %v\n", err)
					continue
				}

				body, err = io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					fmt.Printf("Error reading response: %v\n", err)
					continue
				}

				if resp.StatusCode == http.StatusOK {
					break
				}

				if resp.StatusCode == 400 {
					fmt.Printf("API Error (Status 400): %s\nLogging to errors.txt\n", string(body))
					f, err := os.OpenFile("errors.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
					if err == nil {
						timestamp := time.Now().Format(time.RFC3339)
						f.WriteString(fmt.Sprintf("Timestamp: %s\nError: %s\n", timestamp, string(body)))
						f.WriteString("Last Messages:\n")
						start := 0
						if len(messages) > 2 {
							start = len(messages) - 2
						}
						for i := start; i < len(messages); i++ {
							content, _ := json.Marshal(messages[i])
							f.WriteString(fmt.Sprintf("%s\n", content))
						}
						f.WriteString("--------------------------------------------------\n")
						f.Close()
					}
					break
				}

				if resp.StatusCode == 429 || resp.StatusCode >= 500 {
					fmt.Printf("API Error (Status %d): %s\n", resp.StatusCode, string(body))
					continue
				}

				fmt.Printf("API Error (Status %d): %s\n", resp.StatusCode, string(body))
				break
			}

			if resp == nil || resp.StatusCode != http.StatusOK {
				break
			}

			var chatResp ChatCompletionResponse
			if err := json.Unmarshal(body, &chatResp); err != nil {
				fmt.Printf("Error parsing response: %v\n", err)
				break
			}

			if chatResp.Error != nil {
				fmt.Printf("API Error: %s\n", chatResp.Error.Message)
				break
			}

			if len(chatResp.Choices) == 0 {
				fmt.Println("No choices returned from API")
				break
			}

			if chatResp.Usage != nil {
				lastUsage = chatResp.Usage.TotalTokens
			}

			msg := chatResp.Choices[0].Message
			messages = append(messages, msg)

			// Print thoughts if present
			if len(msg.ToolCalls) > 0 {
				extractAndPrintThoughts(msg.Content)
			}
			printThought(msg.ExtraContent)

			contextReset := false

			if len(msg.ToolCalls) > 0 {
				for _, toolCall := range msg.ToolCalls {
					if ctx.Err() != nil {
						break
					}

					printThought(toolCall.ExtraContent)

					var toolResult string
					var toolErr error

					switch toolCall.Function.Name {
					case "apply_udiff":
						fmt.Printf("\n\033[1;35m🛠  Tool Call: apply_udiff\033[0m\n")
						var args struct {
							Path string `json:"path"`
							Diff string `json:"diff"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							// Dry run first to check validity and generate helpful errors
							_, err := applyUDiff(ctx, args.Path, args.Diff, true)
							if err != nil {
								toolErr = err
							} else {
								// Show diff to user
								fmt.Printf("Proposed changes to %s:\n", args.Path)
								printColoredDiff(args.Diff)

								var confirm string
								if *autoApprove {
									fmt.Println("Auto-approving changes...")
									confirm = "y"
								} else {
									// Ask for confirmation
									fmt.Print("Apply these changes? [y/N]: ")
									confirm, _ = bufio.NewReader(os.Stdin).ReadString('\n')
								}

								if ctx.Err() != nil {
									toolErr = fmt.Errorf("interrupted by user")
								} else {
									confirm = strings.TrimSpace(confirm)

									if strings.ToLower(confirm) == "y" {
										// Pre-edit hook
										preHookOut := runSkillHooks(ctx, skills, "pre_edit", map[string]string{"path": args.Path})

										toolResult, toolErr = applyUDiff(ctx, args.Path, args.Diff, false)
										if preHookOut != "" {
											toolResult = "[Pre-Edit Hook Output]\n" + preHookOut + "\n\n" + toolResult
										}
										if toolErr == nil {
											fmt.Printf("Successfully applied diff to %s\n", args.Path)
											toolResult = "Diff applied successfully."
										}

										// Post-edit hook
										hookOut := runSkillHooks(ctx, skills, "post_edit", map[string]string{"path": args.Path})
										if hookOut != "" {
											toolResult += "\n\n[Hook Output]\n" + hookOut
										}
									} else {
										fmt.Println("Changes rejected.")
										toolResult = "User rejected the changes."
									}
								}
							}
						}


					case "run_script":
						fmt.Printf("\n\033[1;35m🛠  Tool Call: run_script\033[0m\n")
						var args struct {
							Path string   `json:"path"`
							Args []string `json:"args"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							// Pre-run hook
							preHookOut := runSkillHooks(ctx, skills, "pre_run", map[string]string{"path": args.Path, "args": strings.Join(args.Args, " ")})

							fmt.Printf("Executing script: %s %v\n", args.Path, args.Args)
							toolResult, toolErr = runSafeScript(ctx, args.Path, args.Args, skillsPrompt)
							if preHookOut != "" {
								toolResult = "[Pre-Run Hook Output]\n" + preHookOut + "\n\n" + toolResult
							}

							// Post-run hook
							hookOut := runSkillHooks(ctx, skills, "post_run", map[string]string{"path": args.Path, "args": strings.Join(args.Args, " ")})
							if hookOut != "" {
								toolResult += "\n\n[Hook Output]\n" + hookOut
							}
						}


					case "shorten_context":
						fmt.Printf("\n\033[1;35m🛠  Tool Call: shorten_context\033[0m\n")
						var args struct {
							Task   string `json:"task_description"`
							Future string `json:"future_plans"`
							Vital  string `json:"vital_information"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							fmt.Println("Summarizing context...")
							summary, err := summarizeContext(apiKey, messages, args.Task, args.Future, args.Vital)
							if err != nil {
								toolErr = fmt.Errorf("failed to summarize: %v", err)
							} else {
								if strings.TrimSpace(summary) == "" {
									summary = "(No summary provided by the model)"
								}
								// Reset context
								sysMsg := messages[0]
								messages = []Message{sysMsg}
								messages = append(messages, Message{
									Role:    "user",
									Content: fmt.Sprintf("Context has been shortened. Summary of previous conversation:\n%s", summary),
								})

								fmt.Println("Context shortened.")
								fmt.Println("Gemini (Summary):")
								printMarkdown(summary)

								contextReset = true
							}
						}

					default:
						toolErr = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
					}

					// Append tool response
					content := toolResult
					if toolErr != nil {
						fmt.Printf("Tool Error: %v\n", toolErr)
						content = fmt.Sprintf("Error: %v", toolErr)
					}

					if !contextReset {
						messages = append(messages, Message{
							Role:       "tool",
							Content:    content,
							ToolCallID: toolCall.ID,
						})
					}
				}

				if contextReset {
					break
				}

				// Check for new skills
				// Re-discover only project skills for dynamic updates
				currentProjectSkills := discoverSkills("./skills")

				// Merge again
				for _, s := range currentProjectSkills {
					skillMap[s.Name] = s
				}

				var newSkills []Skill
				for _, s := range skillMap {
					if !knownSkills[s.Name] {
						newSkills = append(newSkills, s)
						knownSkills[s.Name] = true
					}
				}

				if len(newSkills) > 0 {
					// Rebuild main skills list
					skills = []Skill{}
					for _, s := range skillMap {
						skills = append(skills, s)
					}
					skillsPrompt = generateSkillsPrompt(skills)

					var sb strings.Builder
					sb.WriteString("SYSTEM NOTICE: New skills discovered:\n")
					for _, s := range newSkills {
						sb.WriteString(fmt.Sprintf("- %s: %s\n", s.Name, s.Description))
					}

					messages = append(messages, Message{
						Role:    "system",
						Content: sb.String(),
					})
					fmt.Println(sb.String()) // Also print to console for user visibility
				}

				// Loop back to send tool outputs to model
				continue
			}

			if contextReset {
				break
			}

			// No tool calls, just print response
			cleanContent := extractAndPrintThoughts(msg.Content)
			if strings.TrimSpace(cleanContent) != "" {
				fmt.Printf("\n\033[1;34m🤖 Gemini:\033[0m\n")
				printMarkdown(cleanContent)
			}
			break
		}

		// End of turn cleanup
		mu.Lock()
		if currentCancel != nil {
			cancel()
			currentCancel = nil
		}
		mu.Unlock()

		// End of turn: Check for git changes and propose commit
		if (*gitAutoCommit || *gitForceCommit) && isGitDirty() {
			// Get conversation history for this turn
			var turnHistory []Message
			if startHistoryIndex < len(messages) {
				turnHistory = messages[startHistoryIndex:]
			}

			// If history is empty (e.g. after context reset), use the full recent context
			if len(turnHistory) == 0 && len(messages) > 0 {
				if len(messages) > 1 && messages[0].Role == "system" {
					turnHistory = messages[1:]
				} else {
					turnHistory = messages
				}
			}

			if err := performGitCommit(apiKey, turnHistory, skills, *gitForceCommit); err != nil {
				fmt.Printf("Git commit workflow failed: %v\n", err)
			}
		}

		// Check token usage
		if lastUsage > 400000 && len(messages) > 2 {
			fmt.Printf("\n[System] Context size is %d tokens (>400,000).\n", lastUsage)
			fmt.Print("Would you like to ask the model to shorten the context? [y/N]: ")
			confirm, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
				pendingInput = "The context size has exceeded 400,000 tokens. Please use the 'shorten_context' tool to summarize the conversation and reset the context."
			}
		}
		saveHistory(messages)
	}
}

func startSpinner(stopChan chan struct{}, doneChan chan struct{}) {
	defer close(doneChan)
	chars := []rune{'|', '/', '-', '\\'}
	i := 0
	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Initial print
	fmt.Printf("\r%c Waiting... (0s)", chars[0])

	for {
		select {
		case <-stopChan:
			fmt.Print("\r\033[K") // Clear line
			return
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			fmt.Printf("\r%c Waiting... (%s)", chars[i%len(chars)], elapsed)
			i++
		}
	}
}

func getLatestVersion() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/robert-at-pretension-io/simple-agent/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API status: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.Split(v, ".")
	var res []int
	for _, p := range parts {
		i, _ := strconv.Atoi(p)
		res = append(res, i)
	}
	return res
}

func isNewer(current, latest string) bool {
	c := parseVersion(current)
	l := parseVersion(latest)

	lenC := len(c)
	lenL := len(l)
	maxLen := lenC
	if lenL > maxLen {
		maxLen = lenL
	}

	for i := 0; i < maxLen; i++ {
		vC, vL := 0, 0
		if i < lenC { vC = c[i] }
		if i < lenL { vL = l[i] }
		if vL > vC { return true }
		if vL < vC { return false }
	}
	return false
}

func autoUpdate() {
	fmt.Println("Checking for updates...")

	latest, err := getLatestVersion()
	if err != nil {
		fmt.Printf("⚠️  Could not check for updates: %v\n", err)
		return
	}

	if !isNewer(Version, latest) {
		fmt.Println("✅ You are using the latest version.")
		return
	}

	fmt.Printf("⬇️  New version available: %s (Current: %s)\n", latest, Version)

	// Get current executable info to check for changes
	exe, err := os.Executable()
	if err != nil {
		return
	}
	infoBefore, err := os.Stat(exe)
	if err != nil {
		return
	}

	var cmd *exec.Cmd

	// Create temp script to update using the install script (release channel)
	// This avoids "text file busy" errors when updating the running binary
	tmpFile, err := os.CreateTemp("", "install-agent-*.sh")
	if err != nil {
		fmt.Printf("⚠️  Update failed: %v\n", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(installScript); err != nil {
		fmt.Printf("⚠️  Update failed: %v\n", err)
		return
	}
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0755)

	// Execute install script (arg1=version(empty=latest), arg2=target)
	cmd = exec.Command("/bin/sh", tmpFile.Name(), "", exe)

	// Execute and capture output
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("⚠️  Binary update failed: %v\n", err)
		if len(out) > 0 {
			fmt.Printf("Output:\n%s\n", out)
		}

		// Fallback: Try 'go install' for backward compatibility
		fmt.Println("🔄 Attempting fallback to 'go install'...")
		cmd = exec.Command("go", "install", "github.com/robert-at-pretension-io/simple-agent@latest")
		cmd.Env = append(os.Environ(), "GOPROXY=direct")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("⚠️  Fallback update failed: %v\n", err)
			if len(out) > 0 {
				fmt.Printf("Output:\n%s\n", out)
			}
			return
		}
		fmt.Println("✅ Fallback update complete via 'go install'. Please restart.")
		os.Exit(0)
	}


	// Check if binary was updated
	if infoAfter, err := os.Stat(exe); err == nil {
		if infoAfter.ModTime().After(infoBefore.ModTime()) {
			// Verify if the version actually changed to prevent restart loops
			if out, err := exec.Command(exe, "--version").CombinedOutput(); err == nil {
				newVer := strings.TrimSpace(string(out))
				if newVer == fmt.Sprintf("Simple Agent %s", Version) {
					return // Same version, continue running
				}
			}

			fmt.Println("✅ Update installed. Please restart the agent.")
			os.Exit(0)
		}
	}
}

func setupCoreSkills() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	// Create a hidden directory in user home for core skills
	CoreSkillsDir = filepath.Join(home, ".simple_agent", "core_skills")

	// Remove old version to ensure updates apply
	os.RemoveAll(CoreSkillsDir)

	err = fs.WalkDir(embeddedSkillsFS, "skills", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Rel path from "skills" root in embed
		relPath, _ := filepath.Rel("skills", path)
		targetPath := filepath.Join(CoreSkillsDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		data, err := embeddedSkillsFS.ReadFile(path)
		if err != nil {
			return err
		}

		// Write file (executable for scripts)
		return os.WriteFile(targetPath, data, 0755)
	})

	if err != nil {
		return err
	}

	return nil
}

// --- Tool Implementations ---

// validatePath ensures the path is within the current working directory
func validatePath(path string) (string, error) {
	if path == "" {
		path = "."
	}

	// Resolve virtual "skills/" path to CoreSkillsDir if needed
	if CoreSkillsDir != "" {
		cleanPath := filepath.Clean(path)
		magicPrefix := "skills" + string(os.PathSeparator)
		if strings.HasPrefix(cleanPath, magicPrefix) {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				suffix := strings.TrimPrefix(cleanPath, magicPrefix)
				candidatePath := filepath.Join(CoreSkillsDir, suffix)
				if _, err := os.Stat(candidatePath); err == nil {
					path = candidatePath
				}
			}
		}
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get CWD: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if path is within CoreSkillsDir (Read-Only/Exec allowed)
	isCore := false
	if CoreSkillsDir != "" {
		relCore, err := filepath.Rel(CoreSkillsDir, absPath)
		if err == nil && !strings.HasPrefix(relCore, "..") {
			isCore = true
		}
	}

	// Check if path is within CWD using filepath.Rel
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to check path relation: %w", err)
	}

	if strings.HasPrefix(rel, "..") && !isCore {
		return "", fmt.Errorf("access denied: path '%s' is outside the current working directory", path)
	}

	return absPath, nil
}


func parseArgs(command string) ([]string, error) {
	var args []string
	var current strings.Builder
	var inQuote rune
	escaped := false

	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}

		if r == '"' || r == '\'' {
			inQuote = r
			continue
		}

		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteRune(r)
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args, nil
}

func runSafeScript(ctx context.Context, scriptPath string, args []string, skillsPrompt string) (string, error) {
	// Validate path
	absPath, err := validatePath(scriptPath)
	if err != nil {
		return "", fmt.Errorf("%w\n\nREMINDER: run_script can only execute scripts defined within a 'skills' directory (Local or Core).\n%s", err, skillsPrompt)
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("script not found: %w\n\nREMINDER: run_script can only execute scripts defined within the 'skills' directory.\n%s", err, skillsPrompt)
	}
	if info.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file\n\nREMINDER: run_script can only execute scripts defined within the 'skills' directory.\n%s", skillsPrompt)
	}

	// Check if it is inside a "scripts" folder within "skills"
	cwd, _ := os.Getwd()
	localSkillsDir := filepath.Join(cwd, "skills")

	// Validate it's in either Local or Core skills dir
	isLocal := strings.HasPrefix(absPath, localSkillsDir)
	isCore := CoreSkillsDir != "" && strings.HasPrefix(absPath, CoreSkillsDir)

	if !isLocal && !isCore {
		return "", fmt.Errorf("script must be inside a 'skills' directory (Local or Core).\n%s", skillsPrompt)
	}

	// Check for 'scripts' in the path components
	// We use string(os.PathSeparator) to be cross-platform
	sep := string(os.PathSeparator)
	if !strings.Contains(absPath, sep+"scripts"+sep) {
		return "", fmt.Errorf("script must be inside a 'scripts' folder.\n%s", skillsPrompt)
	}

	// Determine execution method
	var cmd *exec.Cmd
	ext := filepath.Ext(absPath)

	switch ext {
	case ".py":
		cmdArgs := append([]string{absPath}, args...)
		cmd = exec.CommandContext(ctx, "python3", cmdArgs...)
	case ".sh":
		cmdArgs := append([]string{absPath}, args...)
		cmd = exec.CommandContext(ctx, "bash", cmdArgs...)
	case ".js":
		cmdArgs := append([]string{absPath}, args...)
		cmd = exec.CommandContext(ctx, "node", cmdArgs...)
	default:
		// Try to execute directly
		cmd = exec.CommandContext(ctx, absPath, args...)
	}

	out, err := cmd.CombinedOutput()
	output := string(out)

	// Output size check to prevent context overflow
	const MaxOutputChars = 50000 // ~12.5k tokens
	if len(output) > MaxOutputChars {
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			outputDir := filepath.Join(home, ".simple_agent", "outputs")
			_ = os.MkdirAll(outputDir, 0755)
			
			filename := fmt.Sprintf("output_%d.txt", time.Now().UnixNano())
			filePath := filepath.Join(outputDir, filename)
			
			if writeErr := os.WriteFile(filePath, out, 0644); writeErr == nil {
				output = fmt.Sprintf("Output too large (%d chars). Saved to %s\nRead this file to see the results.", len(output), filePath)
			}
		}
	}

	if err != nil {
		return output, fmt.Errorf("script execution failed: %w\nOutput:\n%s", err, output)
	}
	return output, nil
}


// applyUDiff applies a unified diff to a file
func applyUDiff(ctx context.Context, path string, diff string, dryRun bool) (string, error) {
	absPath, err := validatePath(path)
	if err != nil {
		return "", err
	}

	// Protect CoreSkillsDir from modification
	if CoreSkillsDir != "" && strings.HasPrefix(absPath, CoreSkillsDir) {
		return "", fmt.Errorf("access denied: cannot modify core skills in '%s'", CoreSkillsDir)
	}

	// Read original file
	var content string
	data, err := os.ReadFile(absPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to read file: %w", err)
		}
		content = "" // New file
	} else {
		content = string(data)
	}

	// Normalize line endings to \n
	content = strings.ReplaceAll(content, "\r\n", "\n")

	hunks := parseHunks(diff)
	if len(hunks) == 0 {
		return "", fmt.Errorf("no valid hunks found in diff")
	}

	// Apply hunks
	newContent := content
	for i, hunk := range hunks {
		// Check context cancellation
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		// Create search block
		searchBlock := strings.Join(hunk.SearchLines, "\n")
		replaceBlock := strings.Join(hunk.ReplaceLines, "\n")

		// If search block is empty (creating a new file), we just append/replace
		if len(hunk.SearchLines) == 0 && content == "" {
			newContent = replaceBlock
			continue
		}

		// Check for pure insertion without context in existing file
		if len(hunk.SearchLines) == 0 && content != "" {
			return "", fmt.Errorf("hunk %d failed to apply: pure insertion (no context lines) is not allowed in existing file.\nPlease provide at least 2 lines of context (' ') around the new code to uniquely locate the insertion point.", i+1)
		}

		// Verify uniqueness of the search block
		matches := strings.Count(newContent, searchBlock)
		if matches > 1 {
			return "", fmt.Errorf("hunk %d failed to apply: ambiguous context. The search block matches %d times in the file.\nPlease provide more context lines to uniquely identify the code to replace.", i+1, matches)
		}

		// Check if search block exists
		if matches == 0 {
			// Fuzzy search for error reporting
			fileLines := strings.Split(newContent, "\n")
			bestIdx, score := findBestMatch(fileLines, hunk.SearchLines)

			// Threshold for suggestion (e.g. 50% match)
			if bestIdx != -1 && score > 0.5 {
				start := bestIdx - 5
				if start < 0 {
					start = 0
				}
				end := bestIdx + len(hunk.SearchLines) + 5
				if end > len(fileLines) {
					end = len(fileLines)
				}

				snippet := strings.Join(fileLines[start:end], "\n")
				return "", fmt.Errorf("hunk %d failed to apply: context not found.\nProbable match found at lines %d-%d (score %.2f):\n```\n%s\n```\nPlease verify the context lines and try again.", i+1, start+1, end, score, snippet)
			}

			return "", fmt.Errorf("hunk %d failed to apply: context not found.\nSearch Block:\n%s", i+1, searchBlock)
		}

		// Perform replacement (replace 1 occurrence)
		newContent = strings.Replace(newContent, searchBlock, replaceBlock, 1)
	}

	if dryRun {
		return newContent, nil
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write back to file
	err = os.WriteFile(absPath, []byte(newContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "Success", nil
}

func findBestMatch(fileLines []string, searchLines []string) (int, float64) {
	if len(searchLines) == 0 || len(fileLines) < len(searchLines) {
		return -1, 0.0
	}

	bestScore := 0.0
	bestIdx := -1

	for i := 0; i <= len(fileLines)-len(searchLines); i++ {
		matches := 0
		for j := 0; j < len(searchLines); j++ {
			// Compare trimmed lines to be lenient on whitespace
			if strings.TrimSpace(fileLines[i+j]) == strings.TrimSpace(searchLines[j]) {
				matches++
			}
		}
		score := float64(matches) / float64(len(searchLines))
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return bestIdx, bestScore
}

type Hunk struct {
	SearchLines  []string
	ReplaceLines []string
}

func parseHunks(diff string) []Hunk {
	lines := strings.Split(diff, "\n")
	var hunks []Hunk
	var currentHunk *Hunk

	for _, line := range lines {
		line = strings.TrimRight(line, "\r") // Handle Windows line endings in diff string

		// Check for hunk header
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			currentHunk = &Hunk{
				SearchLines:  []string{},
				ReplaceLines: []string{},
			}
			continue
		}

		// If we haven't found a hunk header yet, skip (e.g. ---/+++ headers)
		if currentHunk == nil {
			continue
		}

		if strings.HasPrefix(line, " ") {
			// Context line: present in both
			content := line[1:]
			currentHunk.SearchLines = append(currentHunk.SearchLines, content)
			currentHunk.ReplaceLines = append(currentHunk.ReplaceLines, content)
		} else if strings.HasPrefix(line, "-") {
			// Removal: present in search only
			content := line[1:]
			currentHunk.SearchLines = append(currentHunk.SearchLines, content)
		} else if strings.HasPrefix(line, "+") {
			// Addition: present in replace only
			content := line[1:]
			currentHunk.ReplaceLines = append(currentHunk.ReplaceLines, content)
		}
		// Ignore other lines
	}

	// Append last hunk
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}

	return hunks
}

func printThought(extraContent json.RawMessage) {
	if len(extraContent) == 0 {
		return
	}
	var content struct {
		Google struct {
			Thought string `json:"thought"`
		} `json:"google"`
	}
	if err := json.Unmarshal(extraContent, &content); err == nil && content.Google.Thought != "" {
		fmt.Printf("\n\033[90m─── [Thought] ───\033[0m\n")
		printMarkdown(content.Google.Thought)
		fmt.Printf("\033[90m───────────────────\033[0m\n")
	}
}

func extractAndPrintThoughts(content string) string {
	re := regexp.MustCompile(`(?s)<thought>(.*?)</thought>`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			fmt.Printf("\n\033[90m─── [Thought] ───\033[0m\n")
			printMarkdown(strings.TrimSpace(match[1]))
			fmt.Printf("\033[90m───────────────────\033[0m\n")
		}
	}
	return re.ReplaceAllString(content, "")
}

func printMarkdown(content string) {
	lines := strings.Split(content, "\n")
	inCodeBlock := false

	// ANSI codes
	reset := "\033[0m"
	bold := "\033[1m"
	cyan := "\033[36m"
	blue := "\033[34m"

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCodeBlock = !inCodeBlock
			fmt.Println(cyan + line + reset)
			continue
		}

		if inCodeBlock {
			fmt.Println(cyan + line + reset)
			continue
		}

		// Headers
		if strings.HasPrefix(line, "#") {
			fmt.Println(bold + blue + line + reset)
			continue
		}

		// Inline formatting (simple)
		// Bold **text**
		line = regexp.MustCompile(`\*\*(.*?)\*\*`).ReplaceAllString(line, bold+"$1"+reset)
		// Code `text`
		line = regexp.MustCompile("`([^`]+)`").ReplaceAllString(line, cyan+"$1"+reset)

		fmt.Println(line)
	}
}

func printColoredDiff(diff string) {
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			fmt.Printf("\033[32m%s\033[0m\n", line)
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			fmt.Printf("\033[31m%s\033[0m\n", line)
		} else {
			fmt.Println(line)
		}
	}
}

func summarizeContext(apiKey string, history []Message, task, future, vital string) (string, error) {
	var historyBuf bytes.Buffer
	for i, msg := range history {
		if i == 0 {
			continue
		} // Skip system prompt
		historyBuf.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		if msg.Content == "" && len(msg.ToolCalls) > 0 {
			historyBuf.WriteString(fmt.Sprintf("%s: [Tool Call: %s]\n", msg.Role, msg.ToolCalls[0].Function.Name))
		}
	}

	instructions := fmt.Sprintf(`1. **Current Task**: %s
2. **Future Plans**: %s
3. **Vital Information**: %s

Ensure the summary is concise but retains all information necessary to continue working on the task and future plans.
Preserve code snippets or specific data mentioned in "Vital Information".`, task, future, vital)

	prompt := fmt.Sprintf(`Please summarize the provided conversation history, adhering to the following constraints:

%s

Conversation History:
%s

---
friendly reminder: Please summarize the conversation history above based on the following constraints:
%s
`, instructions, historyBuf.String(), instructions)

	reqBody := ChatCompletionRequest{
		Model: ModelName,
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", GeminiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}

	spinnerStop := make(chan struct{})
	spinnerDone := make(chan struct{})
	go startSpinner(spinnerStop, spinnerDone)

	resp, err := client.Do(req)

	close(spinnerStop)
	<-spinnerDone

	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API Error (Status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from API")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// --- Git Integration ---

func isGitDirty() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		// If git fails (e.g. not a repo), assume not dirty
		return false
	}
	return len(bytes.TrimSpace(out)) > 0
}

func generateCommitMessage(apiKey string, history []Message) (string, error) {
	// Convert history to a transcript string to avoid tool call complexity with Flash
	var historyBuf bytes.Buffer
	for _, msg := range history {
		historyBuf.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				historyBuf.WriteString(fmt.Sprintf("Tool Call: %s (%s)\n", tc.Function.Name, tc.Function.Arguments))
			}
		}
	}

	if historyBuf.Len() == 0 {
		return "", fmt.Errorf("no conversation history available to generate commit message")
	}

	systemPrompt := "You are an expert developer. Generate a tight git commit message (less than 15 words) describing the changes made in the provided conversation history. Output ONLY the commit message. Do not use markdown or quotes."

	reqBody := ChatCompletionRequest{
		Model: FlashModelName,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: historyBuf.String()},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshaling request: %v", err)
	}

	req, err := http.NewRequest("POST", GeminiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}

	spinnerStop := make(chan struct{})
	spinnerDone := make(chan struct{})
	go startSpinner(spinnerStop, spinnerDone)

	resp, err := client.Do(req)

	close(spinnerStop)
	<-spinnerDone

	if err != nil {
		return "", fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API Error (Status %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from API")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}

func gitCommit(message string) error {
	// Commit tracked files only (modified/deleted)
	// We avoid 'git add .' to prevent accidentally committing untracked files (e.g. debug logs, temp files).
	// Users should explicitly add new files if they intend to commit them.
	commitCmd := exec.Command("git", "commit", "-am", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v\n%s", err, out)
	}
	return nil
}

func performGitCommit(apiKey string, history []Message, skills []Skill, force bool) error {
	if !isGitDirty() {
		return fmt.Errorf("git clean")
	}

	commitMsg, err := generateCommitMessage(apiKey, history)
	if err != nil {
		return fmt.Errorf("failed to generate commit message: %v", err)
	}

	// Pre-commit hook
	hookOut := runSkillHooks(context.Background(), skills, "pre_commit", map[string]string{"message": commitMsg})
	if hookOut != "" {
		fmt.Printf("\n[Pre-Commit Hook Output]\n%s\n", hookOut)
	}

	fmt.Printf("\n[Git] Proposed commit message: %s\n", commitMsg)

	confirm := "y"
	if !force {
		fmt.Print("Commit these changes? [y/N]: ")
		userIn, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		confirm = strings.TrimSpace(userIn)
	}

	if strings.ToLower(confirm) == "y" {
		if err := gitCommit(commitMsg); err != nil {
			return fmt.Errorf("git commit failed: %v", err)
		}
		fmt.Println("Changes committed successfully.")
	} else {
		fmt.Println("Commit aborted.")
	}
	return nil
}

func handleSlashCommand(input string, messages *[]Message, skills []Skill, systemPrompt string, apiKey string) bool {
	cmd := strings.TrimSpace(input)
	if !strings.HasPrefix(cmd, "/") {
		return false
	}

	switch cmd {
	case "/commit":
		var history []Message
		for _, m := range *messages {
			if m.Role != "system" {
				history = append(history, m)
			}
		}
		if err := performGitCommit(apiKey, history, skills, false); err != nil {
			if err.Error() == "git clean" {
				fmt.Println("Nothing to commit (working directory clean).")
			} else {
				fmt.Printf("Error: %v\n", err)
			}
		}
		return true
	case "/clear":
		*messages = []Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
		}
		saveHistory(*messages)
		fmt.Println("Conversation history cleared.")
		return true
	case "/skills":
		fmt.Println("Available Skills:")
		for _, s := range skills {
			fmt.Printf("- %s (v%s): %s\n", s.Name, s.Version, s.Description)
		}
		return true
	case "/history":
		fmt.Printf("History contains %d messages.\n", len(*messages))
		return true
	case "/help":
		fmt.Println("Available Commands:")
		fmt.Println("  /clear   - Clear conversation history")
		fmt.Println("  /commit  - Generate and propose a git commit")
		fmt.Println("  /skills  - List available skills")
		fmt.Println("  /history - Show history stats")
		fmt.Println("  /help    - Show this help message")
		fmt.Println("  /exit    - Exit the agent")
		return true
	case "/exit", "/quit":
		fmt.Println("Exiting...")
		os.Exit(0)
		return true
	}

	fmt.Printf("Unknown command: %s\n", cmd)
	return true
}

func getHistoryPath() string {
	return ".simple_agent_history.json"
}

func loadHistory() []Message {
	path := getHistoryPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return []Message{}
	}
	var messages []Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return []Message{}
	}
	return messages
}

func saveHistory(messages []Message) {
	path := getHistoryPath()
	data, err := json.MarshalIndent(messages, "", "  ")
	if err != nil {
		fmt.Printf("Warning: Failed to save history: %v\n", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Printf("Warning: Failed to save history: %v\n", err)
	}
}
