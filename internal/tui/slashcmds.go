package tui

import "strings"

// SlashCommand represents a slash command available in the TUI
type SlashCommand struct {
	Name        string // e.g., "/model"
	Description string // e.g., "Change AI model"
}

// AvailableCommands is the list of all available slash commands
var AvailableCommands = []SlashCommand{
	{Name: "/model", Description: "Change AI model"},
	{Name: "/agent", Description: "Run agentic task with tools"},
	{Name: "/fix", Description: "Fix last failed command"},
}

// FilterCommands returns commands matching the prefix
func FilterCommands(prefix string) []SlashCommand {
	var matches []SlashCommand
	for _, cmd := range AvailableCommands {
		if strings.HasPrefix(cmd.Name, prefix) {
			matches = append(matches, cmd)
		}
	}
	return matches
}
