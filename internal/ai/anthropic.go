package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/bastio-ai/bast/internal/tools"
)

// DefaultAPITimeout is the default timeout for API calls
const DefaultAPITimeout = 30 * time.Second

// AnthropicProvider implements the Provider interface using Anthropic's Claude API
type AnthropicProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

// ProviderConfig holds configuration for creating an Anthropic provider
type ProviderConfig struct {
	APIKey   string
	Model    string
	BaseURL  string // Optional custom base URL (e.g., for Bastio gateway)
	DeviceID string // Device ID for Bastio User-Agent header
}

// NewAnthropicProvider creates a new Anthropic provider
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	return NewAnthropicProviderWithConfig(ProviderConfig{
		APIKey: apiKey,
		Model:  model,
	})
}

// CLIVersion is the version string for the CLI
const CLIVersion = "1.0.0"

// NewAnthropicProviderWithConfig creates a new Anthropic provider with full configuration
func NewAnthropicProviderWithConfig(cfg ProviderConfig) *AnthropicProvider {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	// Add Bastio CLI User-Agent header when using Bastio gateway
	if cfg.DeviceID != "" {
		userAgent := fmt.Sprintf("bastio-cli/%s device/%s", CLIVersion, cfg.DeviceID)
		opts = append(opts, option.WithHeader("User-Agent", userAgent))
	}

	// Add debug middleware to intercept and log raw HTTP responses
	// This helps diagnose issues with SDK JSON unmarshaling
	if os.Getenv("BAST_DEBUG_HTTP") == "1" {
		opts = append(opts, option.WithMiddleware(func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			resp, err := next(req)
			if err != nil {
				return resp, err
			}
			// Read and log response body
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "DEBUG: Failed to read response body: %v\n", readErr)
				return resp, err
			}
			fmt.Fprintf(os.Stderr, "DEBUG RAW HTTP RESPONSE:\n%s\n", string(body))
			// Restore body for SDK
			resp.Body = io.NopCloser(bytes.NewReader(body))
			return resp, err
		}))
	}

	client := anthropic.NewClient(opts...)
	return &AnthropicProvider{
		client: client,
		model:  anthropic.Model(cfg.Model),
	}
}

// SetModel updates the model used for API calls
func (p *AnthropicProvider) SetModel(model string) {
	p.model = anthropic.Model(model)
}

func (p *AnthropicProvider) GenerateCommand(ctx context.Context, query string, shellCtx ShellContext) (*CommandResult, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	systemPrompt := `You are bast, an AI shell assistant. Your job is to generate shell commands based on the user's request.

IMPORTANT RULES:
1. Respond with ONLY the shell command - no explanations, no markdown, no code blocks
2. The command should be safe and appropriate for the user's environment
3. Use the provided context (current directory, OS, shell, git status) to generate appropriate commands
4. If the request is ambiguous, generate the most likely intended command
5. Never include commands that could be destructive without explicit confirmation markers
6. For git operations, consider the current branch and repository state

Current environment:
- Working directory: %s
- Operating system: %s
- Shell: %s
- User: %s`

	if shellCtx.LastCommand != "" {
		systemPrompt += fmt.Sprintf("\n- Last command: %s (exit status: %d)", shellCtx.LastCommand, shellCtx.ExitStatus)
	}

	formattedSystem := fmt.Sprintf(systemPrompt, shellCtx.CWD, shellCtx.OS, shellCtx.Shell, shellCtx.User)

	// Add git context if available
	gitContext := formatGitContext(shellCtx.Git)
	if gitContext != "" {
		formattedSystem += gitContext
	}

	// Add history context when available
	if len(shellCtx.History) > 0 {
		formattedSystem += "\n\nRecent command history:\n"
		for _, cmd := range shellCtx.History {
			formattedSystem += fmt.Sprintf("$ %s\n", cmd)
		}
	}

	if shellCtx.LastOutput != "" {
		formattedSystem += fmt.Sprintf("\nLast command output:\n%s\n", shellCtx.LastOutput)
	}

	if shellCtx.LastError != "" {
		formattedSystem += fmt.Sprintf("\nLast command stderr:\n%s\n", shellCtx.LastError)
	}

	message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(256),
		System: []anthropic.TextBlockParam{
			{Text: formattedSystem},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(query)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate command: %w", err)
	}

	// Extract text from response
	var command string
	for _, block := range message.Content {
		if block.Type == "text" {
			command = strings.TrimSpace(block.Text)
			break
		}
	}

	if command == "" {
		return nil, fmt.Errorf("no command generated")
	}

	// Clean up command if it's wrapped in code blocks
	command = cleanCommand(command)

	return &CommandResult{
		Command: command,
	}, nil
}

func (p *AnthropicProvider) ExplainCommand(ctx context.Context, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	systemPrompt := `You are bast, an AI shell assistant. Explain the given shell command in a clear, concise way.

RULES:
1. Break down each part of the command
2. Explain what the command does
3. Note any potential risks or side effects
4. Keep the explanation brief but informative`

	message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(512),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(fmt.Sprintf("Explain this command: %s", command))),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to explain command: %w", err)
	}

	var explanation string
	for _, block := range message.Content {
		if block.Type == "text" {
			explanation = strings.TrimSpace(block.Text)
			break
		}
	}

	return explanation, nil
}

func (p *AnthropicProvider) ClassifyIntent(ctx context.Context, query string) (*IntentResult, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	systemPrompt := `You are an intent classifier. Analyze the user's input and determine if they want:
1. "command" - a shell command to be generated and executed
2. "chat" - information, explanation, summary, or conversation

Respond with ONLY a JSON object:
{"intent": "command" or "chat", "confidence": 0.0-1.0, "reasoning": "brief explanation", "needs_history": true/false}

Set needs_history to true when the user is asking about their command history, recent commands, or what they ran previously.

Examples:
- "list all files" → {"intent": "command", "confidence": 0.95, "reasoning": "clear request for ls command", "needs_history": false}
- "what does ls do" → {"intent": "chat", "confidence": 0.9, "reasoning": "asking for explanation", "needs_history": false}
- "summarize the readme" → {"intent": "chat", "confidence": 0.85, "reasoning": "wants content summary, not a command", "needs_history": false}
- "show me the readme" → {"intent": "command", "confidence": 0.7, "reasoning": "likely wants cat/less command", "needs_history": false}
- "delete all tmp files" → {"intent": "command", "confidence": 0.9, "reasoning": "wants rm command", "needs_history": false}
- "how do I find large files" → {"intent": "chat", "confidence": 0.8, "reasoning": "asking for guidance", "needs_history": false}
- "how do permissions work" → {"intent": "chat", "confidence": 0.9, "reasoning": "asking for conceptual explanation", "needs_history": false}
- "how should I understand the output" → {"intent": "chat", "confidence": 0.9, "reasoning": "asking for help interpreting something", "needs_history": false}
- "explain how git branching works" → {"intent": "chat", "confidence": 0.95, "reasoning": "wants conceptual explanation", "needs_history": false}
- "what was the last command I ran" → {"intent": "chat", "confidence": 0.95, "reasoning": "asking about command history", "needs_history": true}
- "show my recent commands" → {"intent": "chat", "confidence": 0.9, "reasoning": "wants to see history", "needs_history": true}
- "what commands have I run" → {"intent": "chat", "confidence": 0.9, "reasoning": "asking about history", "needs_history": true}`

	message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(256),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(query)),
		},
	}, option.WithHeader("X-Bastio-Internal", "intent-classifier"))
	if err != nil {
		return nil, fmt.Errorf("failed to classify intent: %w", err)
	}

	var responseText string
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText = strings.TrimSpace(block.Text)
			break
		}
	}

	// Parse JSON response - first strip any markdown code block wrappers
	responseText = extractJSON(responseText)

	var result struct {
		Intent       string  `json:"intent"`
		Confidence   float64 `json:"confidence"`
		Reasoning    string  `json:"reasoning"`
		NeedsHistory bool    `json:"needs_history"`
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		// If parsing still fails after cleanup, default to chat (safer than executing commands)
		return &IntentResult{
			Intent:       IntentChat,
			Confidence:   0.5,
			Reasoning:    "failed to parse classification, defaulting to chat",
			NeedsHistory: false,
		}, nil
	}

	intent := IntentCommand
	if result.Intent == "chat" {
		intent = IntentChat
	}

	return &IntentResult{
		Intent:       intent,
		Confidence:   result.Confidence,
		Reasoning:    result.Reasoning,
		NeedsHistory: result.NeedsHistory,
	}, nil
}

func (p *AnthropicProvider) Chat(ctx context.Context, query string, shellCtx ShellContext, chatCtx ChatContext) (*ChatResult, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	systemPrompt := fmt.Sprintf(`You are bast, an AI shell assistant. The user is asking a question or wants information.
Provide a helpful, concise response.

Current environment:
- Working directory: %s
- Operating system: %s
- Shell: %s

Keep responses brief and terminal-friendly (no long paragraphs).
If the user asks for something that would be better accomplished with a command, suggest they rephrase their request.`, shellCtx.CWD, shellCtx.OS, shellCtx.Shell)

	if shellCtx.LastCommand != "" {
		systemPrompt += fmt.Sprintf("\n- Last command: %s (exit status: %d)", shellCtx.LastCommand, shellCtx.ExitStatus)
	}

	// Add git context if available
	gitContext := formatGitContext(shellCtx.Git)
	if gitContext != "" {
		systemPrompt += gitContext
	}

	// Add history context when available
	if len(shellCtx.History) > 0 {
		systemPrompt += "\n\nRecent command history:\n"
		for _, cmd := range shellCtx.History {
			systemPrompt += fmt.Sprintf("$ %s\n", cmd)
		}
	}

	if shellCtx.LastOutput != "" {
		systemPrompt += fmt.Sprintf("\nLast command output:\n%s\n", shellCtx.LastOutput)
	}

	if shellCtx.LastError != "" {
		systemPrompt += fmt.Sprintf("\nLast command stderr:\n%s\n", shellCtx.LastError)
	}

	// Append file contents if available
	if len(chatCtx.Files) > 0 {
		systemPrompt += "\n\nFile contents available for reference:"
		for _, f := range chatCtx.Files {
			if f.Error == "" {
				systemPrompt += fmt.Sprintf("\n\n--- %s ---\n%s", f.Path, f.Content)
			} else {
				systemPrompt += fmt.Sprintf("\n\n--- %s ---\n[Error: %s]", f.Path, f.Error)
			}
		}
	}

	// Build message array from conversation history + current query
	var messages []anthropic.MessageParam
	for _, msg := range chatCtx.History {
		if msg.Role == "user" {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		} else {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(query)))

	message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(1024),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: messages,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate chat response: %w", err)
	}

	var response string
	for _, block := range message.Content {
		if block.Type == "text" {
			response = strings.TrimSpace(block.Text)
			break
		}
	}

	return &ChatResult{
		Response: response,
	}, nil
}

// extractJSON extracts JSON from a response that may be wrapped in markdown code blocks
func extractJSON(text string) string {
	text = strings.TrimSpace(text)
	// Remove markdown code block wrappers
	if strings.HasPrefix(text, "```json") {
		text = strings.TrimPrefix(text, "```json")
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
	} else if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		if idx := strings.LastIndex(text, "```"); idx != -1 {
			text = text[:idx]
		}
	}
	return strings.TrimSpace(text)
}

// cleanCommand removes markdown code block formatting if present
func cleanCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)

	// Remove ```bash or ```sh or ``` prefix
	if strings.HasPrefix(cmd, "```") {
		lines := strings.Split(cmd, "\n")
		if len(lines) > 1 {
			// Remove first line (```bash) and last line (```)
			lines = lines[1:]
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			cmd = strings.Join(lines, "\n")
		}
	}

	// Remove single backticks
	cmd = strings.Trim(cmd, "`")

	return strings.TrimSpace(cmd)
}

// formatGitContext formats git context for inclusion in prompts
func formatGitContext(git *GitContext) string {
	if git == nil || !git.IsRepo {
		return ""
	}

	var ctx strings.Builder
	ctx.WriteString("\nGit Repository Context:\n")
	ctx.WriteString(fmt.Sprintf("- Branch: %s\n", git.Branch))

	if git.HasStaged {
		ctx.WriteString("- Has staged changes\n")
	}
	if git.HasUncommitted {
		ctx.WriteString("- Has uncommitted changes\n")
	}
	if git.HasUntracked {
		ctx.WriteString("- Has untracked files\n")
	}
	if git.MergeInProgress {
		ctx.WriteString("- MERGE IN PROGRESS\n")
	}
	if git.RebaseInProgress {
		ctx.WriteString("- REBASE IN PROGRESS\n")
	}

	return ctx.String()
}

// detectProjectContext analyzes the working directory to determine project type and structure
func detectProjectContext(cwd string) string {
	var ctx strings.Builder

	// Check for Go project
	if content, err := os.ReadFile(filepath.Join(cwd, "go.mod")); err == nil {
		ctx.WriteString("\nProject Context:\n")
		ctx.WriteString("- Type: Go Application\n")
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				ctx.WriteString(fmt.Sprintf("- Module: %s\n", strings.TrimPrefix(line, "module ")))
			}
			if strings.HasPrefix(line, "go ") {
				ctx.WriteString(fmt.Sprintf("- Go Version: %s\n", strings.TrimPrefix(line, "go ")))
			}
		}
	} else if _, err := os.Stat(filepath.Join(cwd, "package.json")); err == nil {
		// Check for Node.js project
		ctx.WriteString("\nProject Context:\n")
		ctx.WriteString("- Type: Node.js Application\n")
	} else if _, err := os.Stat(filepath.Join(cwd, "Cargo.toml")); err == nil {
		// Check for Rust project
		ctx.WriteString("\nProject Context:\n")
		ctx.WriteString("- Type: Rust Application\n")
	} else if _, err := os.Stat(filepath.Join(cwd, "pyproject.toml")); err == nil {
		// Check for Python project (pyproject.toml)
		ctx.WriteString("\nProject Context:\n")
		ctx.WriteString("- Type: Python Application\n")
	} else if _, err := os.Stat(filepath.Join(cwd, "requirements.txt")); err == nil {
		// Check for Python project (requirements.txt)
		ctx.WriteString("\nProject Context:\n")
		ctx.WriteString("- Type: Python Application\n")
	}

	// List top-level directories for structure overview
	entries, err := os.ReadDir(cwd)
	if err == nil {
		var dirs []string
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dirs = append(dirs, e.Name()+"/")
			}
		}
		if len(dirs) > 0 {
			// Only add header if we haven't already
			if ctx.Len() == 0 {
				ctx.WriteString("\nProject Context:\n")
			}
			ctx.WriteString(fmt.Sprintf("- Structure: %s\n", strings.Join(dirs, ", ")))
		}
	}

	return ctx.String()
}

// FixCommand analyzes a failed command and suggests a fix
func (p *AnthropicProvider) FixCommand(ctx context.Context, failedCmd string, errorOutput string, shellCtx ShellContext) (*FixResult, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	systemPrompt := fmt.Sprintf(`You are bast, an AI shell assistant helping to fix failed commands.

The user ran a command that failed. Your job is to:
1. Analyze the error output
2. Determine what went wrong
3. Suggest a corrected command that should work

IMPORTANT RULES:
1. Respond with ONLY a JSON object: {"fixed_command": "...", "explanation": "...", "was_fixed": true/false}
2. If the error is easily fixable (typo, wrong flag, missing dependency), provide a fixed command
3. If the error requires manual intervention (missing file, permissions issue that needs sudo), explain what to do
4. Set was_fixed to true if you provided a working fixed command, false if only explanation
5. Keep explanations concise (1-2 sentences)

Current environment:
- Working directory: %s
- Operating system: %s
- Shell: %s
- User: %s`, shellCtx.CWD, shellCtx.OS, shellCtx.Shell, shellCtx.User)

	userPrompt := fmt.Sprintf("Failed command: %s\n\nError output:\n%s", failedCmd, errorOutput)

	message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(512),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to analyze error: %w", err)
	}

	var responseText string
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText = strings.TrimSpace(block.Text)
			break
		}
	}

	// Parse JSON response
	responseText = extractJSON(responseText)

	var result struct {
		FixedCommand string `json:"fixed_command"`
		Explanation  string `json:"explanation"`
		WasFixed     bool   `json:"was_fixed"`
	}

	if err := json.Unmarshal([]byte(responseText), &result); err != nil {
		// Fallback: try to extract useful info even if not valid JSON
		return &FixResult{
			Explanation: responseText,
			WasFixed:    false,
		}, nil
	}

	return &FixResult{
		FixedCommand: cleanCommand(result.FixedCommand),
		Explanation:  result.Explanation,
		WasFixed:     result.WasFixed,
	}, nil
}

// ExplainOutput analyzes command output and provides an explanation
func (p *AnthropicProvider) ExplainOutput(ctx context.Context, output string, prompt string, shellCtx ShellContext) (*ChatResult, error) {
	ctx, cancel := context.WithTimeout(ctx, DefaultAPITimeout)
	defer cancel()

	systemPrompt := fmt.Sprintf(`You are bast, an AI shell assistant helping to explain command output.

The user has piped command output to you for analysis. Your job is to:
1. Understand what the output represents
2. Highlight important information
3. Answer any specific questions the user has

Keep your response concise and terminal-friendly.

Current environment:
- Working directory: %s
- Operating system: %s
- Shell: %s`, shellCtx.CWD, shellCtx.OS, shellCtx.Shell)

	userPrompt := output
	if prompt != "" {
		userPrompt = fmt.Sprintf("Output to analyze:\n%s\n\nUser's question: %s", output, prompt)
	} else {
		userPrompt = fmt.Sprintf("Explain this output:\n%s", output)
	}

	message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: int64(1024),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to explain output: %w", err)
	}

	var response string
	for _, block := range message.Content {
		if block.Type == "text" {
			response = strings.TrimSpace(block.Text)
			break
		}
	}

	return &ChatResult{
		Response: response,
	}, nil
}

// AgentAPITimeout is the timeout for agentic API calls (longer due to multi-turn)
const AgentAPITimeout = 5 * time.Minute

// DefaultMaxIterations is the default max tool-use iterations
const DefaultMaxIterations = 10

// RunAgent executes an agentic task with tool use
func (p *AnthropicProvider) RunAgent(ctx context.Context, query string, shellCtx ShellContext, chatCtx ChatContext, cfg AgentConfig) (*AgentResult, error) {
	// Set defaults
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = DefaultMaxIterations
	}

	// Build system prompt with dynamic tool list
	var toolList strings.Builder
	if cfg.Registry != nil {
		for _, tool := range cfg.Registry.List() {
			fmt.Fprintf(&toolList, "- %s: %s\n", tool.Name(), tool.Description())
		}
	}

	systemPrompt := fmt.Sprintf(`You are bast, an AI shell assistant with access to tools for executing commands and working with files.

You MUST use the available tools to complete tasks. Do not suggest commands for the user to run - execute them directly using tools.

Available tools:
%sAlways take action with tools rather than providing instructions. Choose the most appropriate tool for each task based on the descriptions above.

Current environment:
- Working directory: %s
- Operating system: %s
- Shell: %s
- User: %s`, toolList.String(), shellCtx.CWD, shellCtx.OS, shellCtx.Shell, shellCtx.User)

	// Add project context
	projectCtx := detectProjectContext(shellCtx.CWD)
	if projectCtx != "" {
		systemPrompt += projectCtx
	}

	// Add git context if available
	gitContext := formatGitContext(shellCtx.Git)
	if gitContext != "" {
		systemPrompt += gitContext
	}

	if shellCtx.LastCommand != "" {
		systemPrompt += fmt.Sprintf("\n- Last command: %s (exit status: %d)", shellCtx.LastCommand, shellCtx.ExitStatus)
	}

	if len(shellCtx.History) > 0 {
		systemPrompt += "\n\nRecent command history:\n"
		for _, cmd := range shellCtx.History {
			systemPrompt += fmt.Sprintf("$ %s\n", cmd)
		}
	}

	if shellCtx.LastOutput != "" {
		systemPrompt += fmt.Sprintf("\nLast command output:\n%s\n", shellCtx.LastOutput)
	}

	if shellCtx.LastError != "" {
		systemPrompt += fmt.Sprintf("\nLast command stderr:\n%s\n", shellCtx.LastError)
	}

	if len(chatCtx.Files) > 0 {
		systemPrompt += "\n\nFile contents available for reference:"
		for _, f := range chatCtx.Files {
			if f.Error == "" {
				systemPrompt += fmt.Sprintf("\n\n--- %s ---\n%s", f.Path, f.Content)
			} else {
				systemPrompt += fmt.Sprintf("\n\n--- %s ---\n[Error: %s]", f.Path, f.Error)
			}
		}
	}

	// Build initial messages from conversation history
	var messages []anthropic.MessageParam
	for _, msg := range chatCtx.History {
		if msg.Role == "user" {
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content)))
		} else {
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content)))
		}
	}
	messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(query)))

	// Build tool definitions from registry
	var apiTools []anthropic.ToolUnionParam
	if cfg.Registry != nil {
		for _, tool := range cfg.Registry.List() {
			schema := tool.InputSchema()
			// Convert our schema to the Anthropic format
			properties := make(map[string]any)
			for name, prop := range schema.Properties {
				propDef := map[string]any{
					"type":        prop.Type,
					"description": prop.Description,
				}
				if len(prop.Enum) > 0 {
					propDef["enum"] = prop.Enum
				}
				properties[name] = propDef
			}

			inputSchema := anthropic.ToolInputSchemaParam{
				Properties: properties,
				Required:   schema.Required,
			}

			toolParam := anthropic.ToolParam{
				Name:        tool.Name(),
				Description: anthropic.String(tool.Description()),
				InputSchema: inputSchema,
			}
			apiTools = append(apiTools, anthropic.ToolUnionParam{OfTool: &toolParam})
		}
	}

	result := &AgentResult{
		ToolCalls: []ToolCall{},
	}

	// Agentic loop
	for iteration := 0; iteration < cfg.MaxIterations; iteration++ {
		result.Iterations = iteration + 1

		// Use OfAny on first iteration to force tool use
		// Use OfAuto on subsequent iterations to allow completion
		var toolChoice anthropic.ToolChoiceUnionParam
		if iteration == 0 {
			toolChoice = anthropic.ToolChoiceUnionParam{
				OfAny: &anthropic.ToolChoiceAnyParam{},
			}
		} else {
			toolChoice = anthropic.ToolChoiceUnionParam{
				OfAuto: &anthropic.ToolChoiceAutoParam{},
			}
		}

		// Make API call
		message, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     p.model,
			MaxTokens: int64(4096),
			System: []anthropic.TextBlockParam{
				{Text: systemPrompt},
			},
			Messages:   messages,
			Tools:      apiTools,
			ToolChoice: toolChoice,
		}, option.WithHeader("X-Bastio-Internal", "agent"))
		if err != nil {
			return nil, fmt.Errorf("failed to run agent: %w", err)
		}

		// Process response blocks
		var toolResults []anthropic.ContentBlockParamUnion
		var responseText strings.Builder

		// Debug logging for ContentBlockUnion fields
		if os.Getenv("BAST_DEBUG_HTTP") == "1" {
			fmt.Fprintf(os.Stderr, "DEBUG: Content block count=%d\n", len(message.Content))
			for i, block := range message.Content {
				fmt.Fprintf(os.Stderr, "DEBUG: Block[%d] Type=%q ID=%q Name=%q Input=%v\n",
					i, block.Type, block.ID, block.Name, block.Input)
			}
		}

		for _, block := range message.Content {
			switch block.Type {
			case "text":
				responseText.WriteString(block.Text)

			case "tool_use":
				// Access tool_use fields directly from ContentBlockUnion
				// (AsToolUse() relies on JSON.raw which may not be populated by gateway)

				// Validate tool name is non-empty
				if block.Name == "" {
					fmt.Fprintf(os.Stderr, "Warning: Received tool_use block with empty name, skipping\n")
					continue
				}

				toolCall := ToolCall{
					ID:   block.ID,
					Name: block.Name,
				}

				// Get raw input JSON
				if block.Input != nil {
					toolCall.Input = block.Input
				}

				// Execute tool if registry available
				if cfg.Registry != nil {
					toolResult := cfg.Registry.ExecuteCall(ctx, tools.Call{
						ID:    block.ID,
						Name:  block.Name,
						Input: toolCall.Input,
					})
					toolCall.Output = toolResult.Content
					toolCall.IsError = toolResult.IsError

					// Build tool result for next API call
					toolResults = append(toolResults, anthropic.NewToolResultBlock(
						block.ID,
						toolResult.Content,
						toolResult.IsError,
					))
				}

				result.ToolCalls = append(result.ToolCalls, toolCall)

				// Call callback if provided
				if cfg.OnToolCall != nil {
					cfg.OnToolCall(toolCall)
				}
			}
		}

		// If no tool calls, we're done
		if len(toolResults) == 0 {
			result.Response = strings.TrimSpace(responseText.String())
			return result, nil
		}

		// Add assistant message and tool results to continue conversation
		messages = append(messages, message.ToParam())
		messages = append(messages, anthropic.NewUserMessage(toolResults...))
	}

	return result, fmt.Errorf("max iterations (%d) reached", cfg.MaxIterations)
}
