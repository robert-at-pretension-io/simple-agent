package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// --- Tool Definition ---

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

// --- Main ---

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	messages := []Message{
		{
			Role: "system",
			Content: `Act as an expert software developer.
You have access to a tool 'apply_udiff' to edit files.
When using 'apply_udiff', provide a unified diff.
- Start hunks with '@@ ... @@'
- Use ' ' for context, '-' for removal, '+' for addition.
- Do not include line numbers in the hunk header.
- Ensure enough context is provided to uniquely locate the code.
- Replace entire blocks/functions rather than small internal edits to ensure uniqueness.
- If a file does not exist, treat it as empty for the 'before' state.
`,
		},
	}

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("Welcome to Gemini REPL (%s)\n", ModelName)
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
				Model:    ModelName,
				Messages: messages,
				Tools:    []Tool{udiffTool},
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

					if toolCall.Function.Name == "apply_udiff" {
						fmt.Printf("[Tool Call: apply_udiff]\n")

						var args struct {
							Path string `json:"path"`
							Diff string `json:"diff"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							messages = append(messages, Message{
								Role:       "tool",
								Content:    fmt.Sprintf("Error parsing arguments: %v", err),
								ToolCallID: toolCall.ID,
							})
							continue
						}

						_, err := applyUDiff(args.Path, args.Diff)
						if err != nil {
							fmt.Printf("Error applying diff: %v\n", err)
							messages = append(messages, Message{
								Role:       "tool",
								Content:    fmt.Sprintf("Error applying diff: %v", err),
								ToolCallID: toolCall.ID,
							})
						} else {
							fmt.Printf("Successfully applied diff to %s\n", args.Path)
							messages = append(messages, Message{
								Role:       "tool",
								Content:    "Diff applied successfully.",
								ToolCallID: toolCall.ID,
							})
						}
					}
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

// --- UDiff Logic ---

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
