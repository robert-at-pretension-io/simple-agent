package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	GeminiURL = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
	ModelName = "gemini-3-pro-preview"
)

// --- API Structures ---

type ChatCompletionRequest struct {
	Model     string          `json:"model"`
	Messages  []Message       `json:"messages"`
	Tools     []Tool          `json:"tools,omitempty"`
	ExtraBody json.RawMessage `json:"extra_body,omitempty"`
}

type Message struct {
	Role         string           `json:"role"`
	Content      string           `json:"content,omitempty"`
	ToolCalls    []ToolCall       `json:"tool_calls,omitempty"`
	ToolCallID   string           `json:"tool_call_id,omitempty"`
	ExtraContent json.RawMessage  `json:"extra_content,omitempty"`
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
		Description: "Execute a command line script or shell command.",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"command": {
					"type": "string",
					"description": "The command to execute (e.g., 'python scripts/analyze.py' or 'ls -la')"
				}
			},
			"required": ["command"]
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

// --- Skills System ---

type Skill struct {
	Name           string
	Description    string
	Path           string
	DefinitionFile string
}

var skillsExplanation = `
# Skills System Philosophy

You have the ability to discover and use "Skills". Skills are specialized capabilities defined in files within the 'skills' directory.

## Purpose
Skills bridge the gap between general reasoning and specific, repeatable tasks. They allow you to:
1.  **Extend Capabilities**: Learn new workflows (e.g., "deploy to AWS", "audit code") without core updates.
2.  **Encapsulate Logic**: Hide complex details in scripts and instructions.
3.  **Autonomy**: You read the "manual" (SKILL.md) and drive execution.

## How to Invoke Skills
1.  **Discover**: The system provides a list of available skills.
2.  **Learn**: If a user request matches a skill, use 'read_file' to read its 'SKILL.md'.
3.  **Execute**: Follow the instructions in 'SKILL.md', often using 'run_script' to execute provided scripts.

## Creating and Managing Skills
You can also create new skills to solve problems!
- **Specific vs. General**: Create specific skills for complex, recurring problems. However, prefer general skills that can be reused.
- **Auditing**: If you find too many specific skills cluttering the system, suggest consolidating them or removing obsolete ones.
- **Best Practices**:
    - **Concise**: Only add necessary context.
    - **Scripts**: Prefer writing utility scripts (e.g., Python, Bash) over asking for manual execution steps.
    - **Self-Contained**: A skill should include everything needed to run it (scripts, instructions).

When faced with a new, complex task that might be repeated, consider creating a new skill for it.
`

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
	var name, description string
	inFrontmatter := false
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
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
			if strings.HasPrefix(line, "name:") {
				name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			} else if strings.HasPrefix(line, "description:") {
				description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
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
		Path:           absPath,
		DefinitionFile: defFile,
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
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", s.Name, s.Description))
		sb.WriteString(fmt.Sprintf("  Definition: %s\n", s.DefinitionFile))
	}
	return sb.String()
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

	baseSystemPrompt := `Act as an expert software developer.
You have access to tools to edit files, read files, list files, search files, and execute scripts.
When using 'apply_udiff', provide a unified diff.
- Start hunks with '@@ ... @@'
- Use ' ' for context, '-' for removal, '+' for addition.
- Do not include line numbers in the hunk header.
- Ensure enough context is provided to uniquely locate the code.
- Replace entire blocks/functions rather than small internal edits to ensure uniqueness.
- If a file does not exist, treat it as empty for the 'before' state.
`
	systemPrompt := baseSystemPrompt + skillsExplanation + skillsPrompt

	messages := []Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("Welcome to Gemini REPL (%s)\n", ModelName)
	if len(skills) > 0 {
		fmt.Printf("Loaded %d skills from ./skills\n", len(skills))
	}
	fmt.Println("Type your message and press Enter. Ctrl+C to exit.")

	client := &http.Client{}

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		if strings.TrimSpace(input) == "" {
			continue
		}

		messages = append(messages, Message{
			Role:    "user",
			Content: input,
		})

		// Interaction loop (handle tool calls)
		for {
			reqBody := ChatCompletionRequest{
				Model:     ModelName,
				Messages:  messages,
				Tools:     []Tool{udiffTool, readFileTool, runScriptTool, listFilesTool, searchFilesTool},
				ExtraBody: json.RawMessage(`{"google": {"thinking_config": {"include_thoughts": true}}}`),
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				fmt.Printf("Error marshaling request: %v\n", err)
				break
			}

			req, err := http.NewRequest("POST", GeminiURL, bytes.NewBuffer(jsonData))
			if err != nil {
				fmt.Printf("Error creating request: %v\n", err)
				break
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+apiKey)

			resp, err := client.Do(req)
			if err != nil {
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

			msg := chatResp.Choices[0].Message
			messages = append(messages, msg)

			// Print thoughts if present
			printThought(msg.ExtraContent)

			if len(msg.ToolCalls) > 0 {
				for _, toolCall := range msg.ToolCalls {
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
							toolResult, toolErr = applyUDiff(args.Path, args.Diff)
							if toolErr == nil {
								fmt.Printf("Successfully applied diff to %s\n", args.Path)
								toolResult = "Diff applied successfully."
							}
						}

					case "read_file":
						fmt.Printf("[Tool Call: read_file]\n")
						var args struct {
							Path string `json:"path"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							fmt.Printf("Reading file: %s\n", args.Path)
							toolResult, toolErr = readFile(args.Path)
						}

					case "run_script":
						fmt.Printf("[Tool Call: run_script]\n")
						var args struct {
							Command string `json:"command"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							toolErr = fmt.Errorf("error parsing arguments: %v", err)
						} else {
							fmt.Printf("Executing: %s\n", args.Command)
							toolResult, toolErr = runScript(args.Command)
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

					default:
						toolErr = fmt.Errorf("unknown tool: %s", toolCall.Function.Name)
					}

					// Append tool response
					content := toolResult
					if toolErr != nil {
						fmt.Printf("Tool Error: %v\n", toolErr)
						content = fmt.Sprintf("Error: %v", toolErr)
					}

					messages = append(messages, Message{
						Role:       "tool",
						Content:    content,
						ToolCallID: toolCall.ID,
					})
				}
				// Loop back to send tool outputs to model
				continue
			}

			// No tool calls, just print response
			fmt.Printf("Gemini: %s\n", msg.Content)
			break
		}
	}
}

// --- Tool Implementations ---

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(data), nil
}

func runScript(command string) (string, error) {
	// Using sh -c to allow for shell features (pipes, etc) and script execution
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command failed: %w\nOutput:\n%s", err, string(out))
	}
	return string(out), nil
}

func listFiles(path string) (string, error) {
	if path == "" {
		path = "."
	}
	entries, err := os.ReadDir(path)
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
	if root == "" {
		root = "."
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}
	var sb strings.Builder
	matchCount := 0

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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
func applyUDiff(path string, diff string) (string, error) {
	// Read original file
	var content string
	data, err := os.ReadFile(path)
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
	for _, hunk := range hunks {
		// Create search block
		searchBlock := strings.Join(hunk.SearchLines, "\n")
		replaceBlock := strings.Join(hunk.ReplaceLines, "\n")

		// If search block is empty (creating a new file), we just append/replace
		if len(hunk.SearchLines) == 0 && content == "" {
			newContent = replaceBlock
			continue
		}

		// Check if search block exists
		if !strings.Contains(newContent, searchBlock) {
			return "", fmt.Errorf("hunk failed to apply: context not found.\nSearch Block:\n%s", searchBlock)
		}

		// Perform replacement (replace 1 occurrence)
		newContent = strings.Replace(newContent, searchBlock, replaceBlock, 1)
	}

	// Write back to file
	err = os.WriteFile(path, []byte(newContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "Success", nil
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
		fmt.Printf("[Thought] %s\n", content.Google.Thought)
	}
}
