package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

const (
	GeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/openai/"
	ModelName     = "gemini-3.0-pro-preview"
)

// Tool definition for apply_udiff
var udiffTool = openai.Tool{
	Type: openai.ToolTypeFunction,
	Function: &openai.FunctionDefinition{
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

func main() {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Println("Please set GEMINI_API_KEY environment variable.")
		os.Exit(1)
	}

	config := openai.DefaultConfig(apiKey)
	config.BaseURL = GeminiBaseURL
	client := openai.NewClientWithConfig(config)

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
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

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := scanner.Text()
		if strings.TrimSpace(input) == "" {
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: input,
		})

		// Interaction loop (handle tool calls)
		for {
			resp, err := client.CreateChatCompletion(
				context.Background(),
				openai.ChatCompletionRequest{
					Model:    ModelName,
					Messages: messages,
					Tools:    []openai.Tool{udiffTool},
				},
			)

			if err != nil {
				fmt.Printf("Error: %v\n", err)
				break
			}

			msg := resp.Choices[0].Message
			messages = append(messages, msg)

			if len(msg.ToolCalls) > 0 {
				for _, toolCall := range msg.ToolCalls {
					if toolCall.Function.Name == "apply_udiff" {
						fmt.Printf("[Tool Call: apply_udiff]\n")
						
						var args struct {
							Path string `json:"path"`
							Diff string `json:"diff"`
						}
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
							messages = append(messages, openai.ChatCompletionMessage{
								Role:       openai.ChatMessageRoleTool,
								Content:    fmt.Sprintf("Error parsing arguments: %v", err),
								ToolCallID: toolCall.ID,
							})
							continue
						}

						output, err := applyUDiff(args.Path, args.Diff)
						if err != nil {
							fmt.Printf("Error applying diff: %v\n", err)
							messages = append(messages, openai.ChatCompletionMessage{
								Role:       openai.ChatMessageRoleTool,
								Content:    fmt.Sprintf("Error applying diff: %v", err),
								ToolCallID: toolCall.ID,
							})
						} else {
							fmt.Printf("Successfully applied diff to %s\n", args.Path)
							messages = append(messages, openai.ChatCompletionMessage{
								Role:       openai.ChatMessageRoleTool,
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
	// We apply them sequentially. Since we are doing search/replace, 
	// we need to be careful about order if multiple hunks touch the same file.
	// Usually diffs are generated top-to-bottom.
	
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
		// We count occurrences to ensure uniqueness if possible, 
		// but for this simple implementation, we'll just replace the first occurrence.
		// A more robust implementation would check for uniqueness.
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
