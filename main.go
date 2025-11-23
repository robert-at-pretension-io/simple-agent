package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
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
	Content      string          `json:"content,omitempty"`
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
		Description: "Apply a unified diff to a file. The diff should be in standard unified format (diff -U0), including headers.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The file path to modify"
				},
				"diff": {
					"type": "string",
					"description": "The unified diff content. Must include @@ ... @@ headers for hunks."
				}
			},
			"required": ["path", "diff"]
		}`),
	},
}

var readFileTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "read_file",
		Description: "Read the contents of a file.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The path to the file to read"
				},
				"start_line": {
					"type": "integer",
					"description": "The line number to start reading from (1-based, optional)"
				},
				"end_line": {
					"type": "integer",
					"description": "The line number to stop reading at (1-based, inclusive, optional)"
				}
			},
			"required": ["path"]
		}`),
	},
}

var runScriptTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "run_script",
		Description: "Execute a script from a skill's scripts directory.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The path to the script to execute (must be within a 'scripts' directory of a skill)"
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

var listFilesTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "list_files",
		Description: "List files and directories at a given path.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The directory path to list (default: .)"
				}
			}
		}`),
	},
}

var searchFilesTool = Tool{
	Type: "function",
	Function: FunctionDefinition{
		Name:        "search_files",
		Description: "Search for a text pattern in files within a directory.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {
					"type": "string",
					"description": "The directory path to search in (default: .)"
				},
				"regex": {
					"type": "string",
					"description": "The regular expression pattern to search for"
				}
			},
			"required": ["regex"]
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
      - ` + "`pre_view` / `post_view`" + `: Runs before/after ` + "`read_file`" + `.
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
2.  **Learn**: If a user request matches a skill, use 'read_file' to read its 'SKILL.md'.
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

	return Skill{
		Name:           name,
		Description:    description,
		Version:        version,
		Dependencies:   dependencies,
		Path:           absPath,
		DefinitionFile: defFile,
		Hooks:          hooks,
	}, nil
}

func generateSkillsPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n# Available Skills\n")
	sb.WriteString("You can perform complex tasks by using the following skills.\n")
	sb.WriteString("To use one, read the definition file first using 'read_file'.\n\n")

	for _, s := range skills {
		sb.WriteString(fmt.Sprintf("- **%s**", s.Name))
		if s.Version != "" {
			sb.WriteString(fmt.Sprintf(" (v%s)", s.Version))
		}
		sb.WriteString(fmt.Sprintf(": %s\n", s.Description))
		if len(s.Dependencies) > 0 {
			sb.WriteString(fmt.Sprintf("  Dependencies: %s\n", strings.Join(s.Dependencies, ", ")))
		}
		sb.WriteString(fmt.Sprintf("  Definition: %s\n", s.DefinitionFile))
	}
	return sb.String()
}

func runSkillHooks(ctx context.Context, skills []Skill, event string, context map[string]string) string {
	var output strings.Builder
	for _, skill := range skills {
		if cmdTemplate, ok := skill.Hooks[event]; ok {
			// Prepare command
			cmdStr := cmdTemplate
			// Replace {skill_path}
			cmdStr = strings.ReplaceAll(cmdStr, "{skill_path}", skill.Path)
			// Replace context variables
			for k, v := range context {
				cmdStr = strings.ReplaceAll(cmdStr, "{"+k+"}", v)
			}

			// Parse command string into script path and args
			parts := strings.Fields(cmdStr)
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

// --- Main ---

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	// Discover skills
	skills := discoverSkills("./skills")
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

	go func() {
		for range sigChan {
			mu.Lock()
			if currentCancel != nil {
				fmt.Println("\n[Interrupted by user]")
				currentCancel()
				currentCancel = nil
			} else {
				fmt.Println("\nExiting...")
				os.Exit(0)
			}
			mu.Unlock()
		}
	}()

	// Run startup hooks (using background context as this is init)
	runSkillHooks(context.Background(), skills, "startup", nil)

	baseSystemPrompt := `You have access to tools to edit files, read files, list files, search files, and execute scripts.
When using 'apply_udiff', provide a unified diff.
- Start hunks with '@@ ... @@'
- Use ' ' for context, '-' for removal, '+' for addition.
- **ALWAYS** include at least 2 lines of context around your changes.
- Do not include line numbers in the hunk header.
- Ensure enough context is provided to uniquely locate the code.
- Replace entire blocks/functions rather than small internal edits to ensure uniqueness.
- If a file does not exist, treat it as empty for the 'before' state.
`
	systemPrompt := baseSystemPrompt + getSkillsExplanation() + skillsPrompt

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Welcome to Gemini REPL (%s)\n", ModelName)
	if len(skills) > 0 {
		fmt.Printf("Loaded %d skills from ./skills\n", len(skills))
	}
	fmt.Println("Type your message. Press Ctrl+D (or Ctrl+Z on Windows) on a new line to send. Ctrl+C to interrupt/exit.")

	client := &http.Client{}

	var pendingInput string

	for {
		var input string
		if pendingInput != "" {
			fmt.Printf("> %s\n", pendingInput)
			input = pendingInput
			pendingInput = ""
		} else {
			fmt.Print("> ")
			var inputLines []string
			for {
				line, err := reader.ReadString('\n')
				if line != "" {
					inputLines = append(inputLines, strings.TrimRight(line, "\r\n"))
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					fmt.Printf("Error reading input: %v\n", err)
					os.Exit(1)
				}
			}

			if len(inputLines) == 0 {
				break
			}

			input = strings.Join(inputLines, "\n")
			if strings.TrimSpace(input) == "" {
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
				Tools:     []Tool{udiffTool, readFileTool, runScriptTool, listFilesTool, searchFilesTool, shortenContextTool},
				ExtraBody: json.RawMessage(`{"google": {"thinking_config": {"include_thoughts": true}}}`),
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				fmt.Printf("Error marshaling request: %v\n", err)
				break
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

			resp, err := client.Do(req)

			close(spinnerStop)
			<-spinnerDone

			if err != nil {
				if ctx.Err() == context.Canceled {
					fmt.Println("\nRequest canceled.")
					break
				}
				fmt.Printf("Error sending request: %v\n", err)
				break
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("Error reading response: %v\n", err)
				break
			}

			if resp.StatusCode != http.StatusOK {
				fmt.Printf("API Error (Status %d): %s\n", resp.StatusCode, string(body))
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
						fmt.Printf("[Tool Call: apply_udiff]\n")
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

								// Ask for confirmation
								fmt.Print("Apply these changes? [y/N]: ")
								confirm, _ := reader.ReadString('\n')
								if ctx.Err() != nil {
									toolErr = fmt.Errorf("interrupted by user")
								} else {
									confirm = strings.TrimSpace(confirm)

									if strings.ToLower(confirm) == "y" {
										// Pre-edit hook
										runSkillHooks(ctx, skills, "pre_edit", map[string]string{"path": args.Path})

										toolResult, toolErr = applyUDiff(ctx, args.Path, args.Diff, false)
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

					case "read_file":
						fmt.Printf("[Tool Call: read_file]\n")
						var args struct {
							Path      string `json:"path"`
							StartLine int    `json:"start_line"`
							EndLine   int    `json:"end_line"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							// Pre-view hook
							runSkillHooks(ctx, skills, "pre_view", map[string]string{"path": args.Path})

							fmt.Printf("Reading file: %s (lines %d-%d)\n", args.Path, args.StartLine, args.EndLine)
							toolResult, toolErr = readFile(ctx, args.Path, args.StartLine, args.EndLine)

							// Post-view hook
							hookOut := runSkillHooks(ctx, skills, "post_view", map[string]string{"path": args.Path})
							if hookOut != "" {
								toolResult += "\n\n[Hook Output]\n" + hookOut
							}
						}

					case "run_script":
						fmt.Printf("[Tool Call: run_script]\n")
						var args struct {
							Path string   `json:"path"`
							Args []string `json:"args"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							// Pre-run hook
							runSkillHooks(ctx, skills, "pre_run", map[string]string{"path": args.Path, "args": strings.Join(args.Args, " ")})

							fmt.Printf("Executing script: %s %v\n", args.Path, args.Args)
							toolResult, toolErr = runSafeScript(ctx, args.Path, args.Args, skillsPrompt)

							// Post-run hook
							hookOut := runSkillHooks(ctx, skills, "post_run", map[string]string{"path": args.Path, "args": strings.Join(args.Args, " ")})
							if hookOut != "" {
								toolResult += "\n\n[Hook Output]\n" + hookOut
							}
						}

					case "list_files":
						fmt.Printf("[Tool Call: list_files]\n")
						var args struct {
							Path string `json:"path"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							fmt.Printf("Listing files in: %s\n", args.Path)
							toolResult, toolErr = listFiles(args.Path)
						}

					case "search_files":
						fmt.Printf("[Tool Call: search_files]\n")
						var args struct {
							Path  string `json:"path"`
							Regex string `json:"regex"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							fmt.Printf("Searching files in %s for: %s\n", args.Path, args.Regex)
							toolResult, toolErr = searchFiles(args.Path, args.Regex)
						}

					case "shorten_context":
						fmt.Printf("[Tool Call: shorten_context]\n")
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
				currentSkills := discoverSkills("./skills")
				var newSkills []Skill
				for _, s := range currentSkills {
					if !knownSkills[s.Name] {
						newSkills = append(newSkills, s)
						knownSkills[s.Name] = true
					}
				}

				if len(newSkills) > 0 {
					skills = currentSkills // Update main skills list
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
				fmt.Println("Gemini:")
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
		if isGitDirty() {
			// Get conversation history for this turn
			var turnHistory []Message
			if startHistoryIndex < len(messages) {
				turnHistory = messages[startHistoryIndex:]
			}

			commitMsg, err := generateCommitMessage(apiKey, turnHistory)
			if err != nil {
				fmt.Printf("Failed to generate commit message: %v\n", err)
			} else {
				// Pre-commit hook
				hookOut := runSkillHooks(context.Background(), skills, "pre_commit", map[string]string{"message": commitMsg})
				if hookOut != "" {
					fmt.Printf("\n[Pre-Commit Hook Output]\n%s\n", hookOut)
				}

				fmt.Printf("\n[Git] Proposed commit message: %s\n", commitMsg)
				fmt.Print("Commit these changes? [y/N]: ")
				confirm, _ := reader.ReadString('\n')
				confirm = strings.TrimSpace(confirm)

				if strings.ToLower(confirm) == "y" {
					if err := gitCommit(commitMsg); err != nil {
						fmt.Printf("Git commit failed: %v\n", err)
					} else {
						fmt.Println("Changes committed successfully.")
					}
				}
			}
		}

		// Check token usage
		if lastUsage > 400000 && len(messages) > 2 {
			fmt.Printf("\n[System] Context size is %d tokens (>400,000).\n", lastUsage)
			fmt.Print("Would you like to ask the model to shorten the context? [y/N]: ")
			confirm, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(confirm)) == "y" {
				pendingInput = "The context size has exceeded 400,000 tokens. Please use the 'shorten_context' tool to summarize the conversation and reset the context."
			}
		}
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

// --- Tool Implementations ---

// validatePath ensures the path is within the current working directory
func validatePath(path string) (string, error) {
	if path == "" {
		path = "."
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get CWD: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if path is within CWD using filepath.Rel
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return "", fmt.Errorf("failed to check path relation: %w", err)
	}

	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("access denied: path '%s' is outside the current working directory", path)
	}

	return absPath, nil
}

func readFile(ctx context.Context, path string, startLine, endLine int) (string, error) {
	absPath, err := validatePath(path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	content := string(data)

	if startLine == 0 && endLine == 0 {
		return content, nil
	}

	lines := strings.Split(content, "\n")
	if startLine < 1 {
		startLine = 1
	}
	if endLine == 0 || endLine > len(lines) {
		endLine = len(lines)
	}

	if startLine > endLine || startLine > len(lines) {
		return "", fmt.Errorf("invalid line range: %d-%d (file has %d lines)", startLine, endLine, len(lines))
	}

	return strings.Join(lines[startLine-1:endLine], "\n"), nil
}

func runSafeScript(ctx context.Context, scriptPath string, args []string, skillsPrompt string) (string, error) {
	// Validate path
	absPath, err := validatePath(scriptPath)
	if err != nil {
		return "", fmt.Errorf("%w\n\nREMINDER: run_script can only execute scripts defined within the 'skills' directory.\n%s", err, skillsPrompt)
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
	skillsDir := filepath.Join(cwd, "skills")

	if !strings.HasPrefix(absPath, skillsDir) {
		return "", fmt.Errorf("script must be inside the 'skills' directory.\n%s", skillsPrompt)
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
	if err != nil {
		return string(out), fmt.Errorf("script execution failed: %w\nOutput:\n%s", err, string(out))
	}
	return string(out), nil
}

func listFiles(path string) (string, error) {
	absPath, err := validatePath(path)
	if err != nil {
		return "", err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to list directory: %w", err)
	}
	var sb strings.Builder
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		suffix := ""
		if entry.IsDir() {
			suffix = "/"
		}
		sb.WriteString(fmt.Sprintf("%s%s (%d bytes)\n", entry.Name(), suffix, info.Size()))
	}
	return sb.String(), nil
}

func searchFiles(root string, pattern string) (string, error) {
	absPath, err := validatePath(root)
	if err != nil {
		return "", err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}
	var sb strings.Builder
	matchCount := 0

	err = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				sb.WriteString(fmt.Sprintf("%s:%d: %s\n", path, i+1, strings.TrimSpace(line)))
				matchCount++
				if matchCount > 100 {
					return fmt.Errorf("too many matches")
				}
			}
		}
		return nil
	})

	if err != nil && err.Error() != "too many matches" {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if matchCount == 0 {
		return "No matches found.", nil
	}
	if matchCount > 100 {
		sb.WriteString("... (results truncated)")
	}
	return sb.String(), nil
}

// applyUDiff applies a unified diff to a file
func applyUDiff(ctx context.Context, path string, diff string, dryRun bool) (string, error) {
	absPath, err := validatePath(path)
	if err != nil {
		return "", err
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

		// Check if search block exists
		if !strings.Contains(newContent, searchBlock) {
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
		fmt.Printf("\033[90m[Thought]\033[0m\n")
		printMarkdown(content.Google.Thought)
	}
}

func extractAndPrintThoughts(content string) string {
	re := regexp.MustCompile(`(?s)<thought>(.*?)</thought>`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			fmt.Printf("\033[90m[Thought]\033[0m\n")
			printMarkdown(strings.TrimSpace(match[1]))
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
	// Stage all changes
	addCmd := exec.Command("git", "add", ".")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %v\n%s", err, out)
	}

	// Commit
	commitCmd := exec.Command("git", "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v\n%s", err, out)
	}
	return nil
}
