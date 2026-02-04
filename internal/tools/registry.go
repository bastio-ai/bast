package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Registry manages the collection of available tools
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	security *BastioSecurityClient // Optional - nil if not using Bastio
}

// NewRegistry creates a new tool registry
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q already registered", name)
	}

	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetDefinitions returns tool definitions for the AI API
func (r *Registry) GetDefinitions() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	defs := make([]Definition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, Definition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return defs
}

// Execute runs a tool by name with the given input
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (*Result, error) {
	tool, ok := r.Get(name)
	if !ok {
		return &Result{
			Output:  fmt.Sprintf("unknown tool: %s", name),
			IsError: true,
		}, nil
	}

	return tool.Execute(ctx, input)
}

// SetSecurityClient configures optional Bastio security validation
func (r *Registry) SetSecurityClient(client *BastioSecurityClient) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.security = client
}

// ExecuteCall executes a tool call and returns the result
func (r *Registry) ExecuteCall(ctx context.Context, call Call) CallResult {
	// If Bastio security is configured, validate the tool call first
	r.mu.RLock()
	security := r.security
	r.mu.RUnlock()

	if security != nil {
		validationResult, err := security.ValidateToolCall(ctx, call)
		if err != nil {
			// Log validation error but don't block execution
			LogWarning(call.Name, fmt.Sprintf("validation failed: %v", err), nil)
		} else {
			switch validationResult.Action {
			case ActionBlock:
				return CallResult{
					CallID:  call.ID,
					Content: fmt.Sprintf("Blocked by security policy: %s", validationResult.Message),
					IsError: true,
				}
			case ActionRequireApproval:
				return CallResult{
					CallID:  call.ID,
					Content: fmt.Sprintf("Requires human approval: %s", validationResult.Message),
					IsError: true,
				}
			case ActionWarn:
				LogWarning(call.Name, validationResult.Message, validationResult.ThreatsDetected)
				// Continue to execution
			// ActionAllow - continue to execution
			}
		}
	}

	// Execute the tool
	result, err := r.Execute(ctx, call.Name, call.Input)
	if err != nil {
		return CallResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("error executing tool: %v", err),
			IsError: true,
		}
	}

	// If security is configured and we have output, scan it
	if security != nil && result.Output != "" && !result.IsError {
		scanResult, err := security.ScanContent(ctx, call.Name, result.Output)
		if err != nil {
			// Log scan error but don't fail - output scanning is best-effort
			LogWarning(call.Name, fmt.Sprintf("content scan failed: %v", err), nil)
		} else {
			switch scanResult.Action {
			case ScanActionBlock:
				return CallResult{
					CallID:  call.ID,
					Content: fmt.Sprintf("Output blocked by security policy: %s", scanResult.Message),
					IsError: true,
				}
			case ScanActionSanitize:
				result.Output = scanResult.ProcessedContent
			case ScanActionWarn:
				LogWarning(call.Name, fmt.Sprintf("content warning: %s", scanResult.Message), scanResult.ThreatsDetected)
			// ScanActionAllow - use output as-is
			}
		}
	}

	return CallResult{
		CallID:  call.ID,
		Content: result.Output,
		IsError: result.IsError,
	}
}

// DefaultRegistry is the global tool registry
var DefaultRegistry = NewRegistry()

// Register adds a tool to the default registry
func Register(tool Tool) error {
	return DefaultRegistry.Register(tool)
}
