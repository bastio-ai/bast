package tui

import (
	"github.com/bastio-ai/bast/internal/ai"
)

// CommandGeneratedMsg is sent when the AI generates a command
type CommandGeneratedMsg struct {
	Result *ai.CommandResult
}

// CommandExplainedMsg is sent when the AI explains a command
type CommandExplainedMsg struct {
	Explanation string
}

// IntentClassifiedMsg is sent when intent classification completes
type IntentClassifiedMsg struct {
	Result *ai.IntentResult
	Query  string // Original query (needed for next step)
}

// ChatResponseMsg is sent when a chat response is ready
type ChatResponseMsg struct {
	Result *ai.ChatResult
	Query  string // Original query (needed to add to conversation history)
}

// ErrorMsg is sent when an error occurs
type ErrorMsg struct {
	Err error
}

func (e ErrorMsg) Error() string {
	return e.Err.Error()
}

// SuggestionsMsg is sent when file search results are ready
type SuggestionsMsg struct {
	Suggestions []string
}

// ModelSelectedMsg is sent when a model is selected
type ModelSelectedMsg struct {
	Model string
}

// AgentResponseMsg is sent when an agentic task completes
type AgentResponseMsg struct {
	Result *ai.AgentResult
	Query  string
}

// ToolCallMsg is sent during agentic execution for each tool call
type ToolCallMsg struct {
	Call ai.ToolCall
}

// FixResultMsg is sent when fix command analysis completes
type FixResultMsg struct {
	Result    *ai.FixResult
	FailedCmd string
}
