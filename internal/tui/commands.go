package tui

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/auth"
	"github.com/bastio-ai/bast/internal/config"
	"github.com/bastio-ai/bast/internal/files"
	"github.com/bastio-ai/bast/internal/safety"
	"github.com/bastio-ai/bast/internal/shell"
	"github.com/bastio-ai/bast/internal/tools"
)

// classifyIntent returns a command that classifies the user's intent
func (m Model) classifyIntent(query string) tea.Cmd {
	return func() tea.Msg {
		cleanQuery := files.StripMentions(query)
		result, err := m.provider.ClassifyIntent(context.Background(), cleanQuery)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return IntentClassifiedMsg{Result: result, Query: query}
	}
}

// chat returns a command that generates a chat response
func (m Model) chat(query string, intentResult *ai.IntentResult) tea.Cmd {
	shellCtx := m.shellCtx
	conversationHistory := m.conversationHistory
	return func() tea.Msg {
		// Use history context if auto-detected from intent classification
		var ctx ai.ShellContext
		if intentResult != nil && intentResult.NeedsHistory {
			ctx = shell.GetContextWithHistory()
		} else {
			ctx = shellCtx
		}

		// Parse explicit @file mentions
		mentions := files.ParseMentions(query)

		// Detect implicit file references (e.g., "the readme")
		refs := files.DetectFileReferences(query)

		// Collect all unique file paths
		seen := make(map[string]bool)
		var paths []string

		// Add explicit mentions first
		for _, mention := range mentions {
			if !seen[mention] {
				seen[mention] = true
				paths = append(paths, mention)
			}
		}

		// Add detected references (resolve to actual files)
		for _, ref := range refs {
			if seen[ref] {
				continue
			}
			// Try to find the actual file
			if path, err := files.FindFile(shellCtx.CWD, ref); err == nil {
				if !seen[path] {
					seen[path] = true
					paths = append(paths, path)
				}
			}
		}

		// Read files (max 100KB total)
		fileContents := files.ReadFiles(shellCtx.CWD, paths, files.MaxTotalFileBytes)

		chatCtx := ai.ChatContext{
			Files:   fileContents,
			History: conversationHistory,
		}
		// Strip @mentions from query to avoid AI interpreting @ syntax as suspicious
		cleanQuery := files.StripMentions(query)
		result, err := m.provider.Chat(context.Background(), cleanQuery, ctx, chatCtx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return ChatResponseMsg{Result: result, Query: query}
	}
}

// generateCommand returns a command that generates a shell command
func (m Model) generateCommand(query string) tea.Cmd {
	shellCtx := m.shellCtx
	return func() tea.Msg {
		cleanQuery := files.StripMentions(query)
		result, err := m.provider.GenerateCommand(context.Background(), cleanQuery, shellCtx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return CommandGeneratedMsg{Result: result}
	}
}

// chatAboutCommand returns a command that generates a chat response about a specific command
func (m Model) chatAboutCommand(query string, command string) tea.Cmd {
	shellCtx := m.shellCtx
	conversationHistory := m.conversationHistory
	return func() tea.Msg {
		// Add context about the generated command to conversation
		historyWithCommand := append(conversationHistory,
			ai.ConversationMessage{Role: "assistant", Content: fmt.Sprintf("I generated this command: %s", command)},
		)

		chatCtx := ai.ChatContext{
			History: historyWithCommand,
		}
		cleanQuery := files.StripMentions(query)
		result, err := m.provider.Chat(context.Background(), cleanQuery, shellCtx, chatCtx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return ChatResponseMsg{Result: result, Query: query}
	}
}

// explainCommand returns a command that explains a shell command
func (m Model) explainCommand(command string) tea.Cmd {
	return func() tea.Msg {
		explanation, err := m.provider.ExplainCommand(context.Background(), command)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return CommandExplainedMsg{Explanation: explanation}
	}
}

// isDangerousCommand checks if a command matches any dangerous patterns
func isDangerousCommand(command string) bool {
	return safety.IsDangerousCommand(command)
}

// selectModel returns a command that saves the selected model to config
func (m Model) selectModel(modelID string) (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return ErrorMsg{Err: err}
		}
		cfg.Model = modelID
		if err := config.Save(cfg); err != nil {
			return ErrorMsg{Err: err}
		}
		return ModelSelectedMsg{Model: modelID}
	}
}

// fixCommand returns a command that analyzes and fixes a failed command
func (m Model) fixCommand() tea.Cmd {
	shellCtx := m.shellCtx
	return func() tea.Msg {
		// Get context with history to access last command and error
		ctx := shell.GetContextWithHistory()

		failedCmd := ctx.LastCommand
		errorOutput := ctx.LastError
		if errorOutput == "" {
			errorOutput = ctx.LastOutput // Sometimes errors go to stdout
		}

		if failedCmd == "" && errorOutput == "" {
			return ErrorMsg{Err: fmt.Errorf("no failed command found. Run a command first, then use /fix")}
		}

		result, err := m.provider.FixCommand(context.Background(), failedCmd, errorOutput, shellCtx)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return FixResultMsg{Result: result, FailedCmd: failedCmd}
	}
}

// runAgent returns a command that runs an agentic task with tool use
func (m Model) runAgent(query string, sendUpdates func(tea.Msg)) tea.Cmd {
	shellCtx := m.shellCtx
	conversationHistory := m.conversationHistory
	return func() tea.Msg {
		// Create tool registry with built-in tools
		registry := tools.NewRegistry()
		cwd, _ := os.Getwd()
		tools.RegisterBuiltins(registry, cwd)

		// Load default plugins (shipped with bast)
		if err := tools.RegisterDefaultPlugins(registry, cwd); err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to load default plugins: %v\n", err)
		}

		// Load user plugins (can override defaults)
		if err := tools.RegisterUserPlugins(registry); err != nil {
			// Log warning but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to load user plugins: %v\n", err)
		}

		// Configure Bastio Agent Security if credentials are available
		if securityCfg := auth.GetBastioSecurityConfig(); securityCfg != nil {
			// Generate a new session ID for this agent invocation
			sessionID := uuid.New().String()

			securityClient := tools.NewBastioSecurityClient(
				securityCfg.BaseURL,
				securityCfg.ProxyID,
				securityCfg.APIKey,
				sessionID,
			)
			registry.SetSecurityClient(securityClient)
		}

		// Parse file mentions
		mentions := files.ParseMentions(query)
		refs := files.DetectFileReferences(query)

		seen := make(map[string]bool)
		var paths []string
		for _, mention := range mentions {
			if !seen[mention] {
				seen[mention] = true
				paths = append(paths, mention)
			}
		}
		for _, ref := range refs {
			if seen[ref] {
				continue
			}
			if path, err := files.FindFile(shellCtx.CWD, ref); err == nil {
				if !seen[path] {
					seen[path] = true
					paths = append(paths, path)
				}
			}
		}

		fileContents := files.ReadFiles(shellCtx.CWD, paths, files.MaxTotalFileBytes)

		chatCtx := ai.ChatContext{
			Files:   fileContents,
			History: conversationHistory,
		}

		// Callback to send tool call updates to the TUI
		onToolCall := func(call ai.ToolCall) {
			if sendUpdates != nil {
				sendUpdates(ToolCallMsg{Call: call})
			}
		}

		agentCfg := ai.AgentConfig{
			MaxIterations: 10,
			Registry:      registry,
			OnToolCall:    onToolCall,
		}

		cleanQuery := files.StripMentions(query)
		result, err := m.provider.RunAgent(context.Background(), cleanQuery, shellCtx, chatCtx, agentCfg)
		if err != nil {
			return ErrorMsg{Err: err}
		}
		return AgentResponseMsg{Result: result, Query: query}
	}
}
