package ai

import (
	"context"
	"encoding/json"

	"github.com/bastio-ai/bast/internal/files"
	"github.com/bastio-ai/bast/internal/tools"
)

// Intent represents what the user wants to accomplish
type Intent string

const (
	IntentCommand Intent = "command" // User wants a shell command generated
	IntentChat    Intent = "chat"    // User wants information or conversation
	IntentAgent   Intent = "agent"   // User wants an agentic task with tool use
)

// IntentResult holds the classification result
type IntentResult struct {
	Intent       Intent
	Confidence   float64 // 0.0 to 1.0
	Reasoning    string  // Brief explanation (for debugging)
	NeedsHistory bool    // true when user asks about command history
}

// CommandResult represents the result of a command generation request
type CommandResult struct {
	Command     string
	Explanation string
}

// FixResult represents the result of an error fix request
type FixResult struct {
	FixedCommand string
	Explanation  string
	WasFixed     bool // true if a fix was suggested, false if no fix needed
}

// ChatResult holds the response for chat intents
type ChatResult struct {
	Response string
}

// AgentResult holds the result of an agentic task
type AgentResult struct {
	Response   string       // Final response text
	ToolCalls  []ToolCall   // All tool calls made during execution
	Iterations int          // Number of API round-trips
}

// ToolCall represents a single tool invocation during agentic execution
type ToolCall struct {
	ID       string          // Tool use ID from the API
	Name     string          // Tool name
	Input    json.RawMessage // Tool input parameters
	Output   string          // Tool execution output
	IsError  bool            // Whether the tool execution failed
}

// AgentConfig holds configuration for agentic execution
type AgentConfig struct {
	MaxIterations int              // Maximum number of tool-use iterations (default 10)
	Registry      *tools.Registry  // Tool registry to use
	OnToolCall    func(ToolCall)   // Optional callback for each tool call
}

// ConversationMessage represents a single message in a conversation
type ConversationMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// ChatContext holds additional context for chat responses
type ChatContext struct {
	Files   []files.FileContent   // File contents to include in the prompt
	History []ConversationMessage // Conversation history for multi-turn chats
}

// Provider defines the interface for AI providers
type Provider interface {
	// GenerateCommand generates a shell command based on the user's query and context
	GenerateCommand(ctx context.Context, query string, shellCtx ShellContext) (*CommandResult, error)

	// ExplainCommand provides an explanation for a given command
	ExplainCommand(ctx context.Context, command string) (string, error)

	// ClassifyIntent determines whether the user wants a command or a chat response
	ClassifyIntent(ctx context.Context, query string) (*IntentResult, error)

	// Chat provides a conversational response to the user's query
	Chat(ctx context.Context, query string, shellCtx ShellContext, chatCtx ChatContext) (*ChatResult, error)

	// RunAgent executes an agentic task with tool use
	RunAgent(ctx context.Context, query string, shellCtx ShellContext, chatCtx ChatContext, cfg AgentConfig) (*AgentResult, error)

	// FixCommand analyzes a failed command and suggests a fix
	FixCommand(ctx context.Context, failedCmd string, errorOutput string, shellCtx ShellContext) (*FixResult, error)

	// ExplainOutput analyzes command output and provides an explanation
	ExplainOutput(ctx context.Context, output string, prompt string, shellCtx ShellContext) (*ChatResult, error)

	// SetModel updates the model used for API calls
	SetModel(model string)
}

// GitContext contains information about the current git repository
type GitContext struct {
	IsRepo           bool     // True if current directory is in a git repo
	Branch           string   // Current branch name
	HasUncommitted   bool     // True if there are uncommitted changes
	HasUntracked     bool     // True if there are untracked files
	HasStaged        bool     // True if there are staged changes
	MergeInProgress  bool     // True if a merge is in progress
	RebaseInProgress bool     // True if a rebase is in progress
	Summary          string   // Brief summary for prompts
}

// ShellContext contains information about the current shell environment
type ShellContext struct {
	CWD         string
	LastCommand string
	LastOutput  string   // stdout of last command (truncated)
	LastError   string   // stderr of last command (truncated)
	ExitStatus  int
	OS          string
	Shell       string
	User        string
	History     []string // recent commands from history file
	Git         *GitContext // Git repository context (nil if not in repo)
}
