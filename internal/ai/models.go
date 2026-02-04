package ai

// ModelOption represents a selectable AI model
type ModelOption struct {
	ID          string
	Name        string
	Description string
}

// AnthropicModels is the list of available Anthropic Claude models
var AnthropicModels = []ModelOption{
	{ID: "claude-sonnet-4-5-20250929", Name: "Claude Sonnet 4.5", Description: "Balanced (recommended)"},
	{ID: "claude-haiku-4-5-20251001", Name: "Claude Haiku 4.5", Description: "Fast & cheap"},
	{ID: "claude-opus-4-5-20251101", Name: "Claude Opus 4.5", Description: "Most capable"},
	{ID: "claude-sonnet-4-20250514", Name: "Claude Sonnet 4", Description: "Previous gen"},
	{ID: "claude-opus-4-20250514", Name: "Claude Opus 4", Description: "Previous gen capable"},
}

// GetModelsForProvider returns the available models for a given provider
func GetModelsForProvider(provider string) []ModelOption {
	switch provider {
	case "anthropic":
		return AnthropicModels
	default:
		return nil
	}
}
