package tools

import (
	"context"
	"encoding/json"
)

// Tool defines the interface that all tools must implement
type Tool interface {
	// Name returns the unique identifier for this tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// InputSchema returns the JSON schema for the tool's input parameters
	InputSchema() InputSchema

	// Execute runs the tool with the given input and returns the result
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

// InputSchema defines the JSON schema for tool input parameters
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a single property in the input schema
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

// Result represents the output of a tool execution
type Result struct {
	Output  string `json:"output"`            // The tool's output
	IsError bool   `json:"is_error,omitempty"` // True if this represents an error
}

// Definition represents a tool definition for the AI API
type Definition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"input_schema"`
}

// Call represents a tool call from the AI
type Call struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// CallResult represents the result of executing a tool call
type CallResult struct {
	CallID  string `json:"call_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}
