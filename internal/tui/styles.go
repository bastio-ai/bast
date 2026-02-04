package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F9FAFB") // Light

	// Container styles
	AppStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	// Input prompt
	PromptStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Command display
	CommandStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// Error messages
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// Explanation box
	ExplanationStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Padding(1).
				MarginTop(1).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(mutedColor)

	// Key hints
	KeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Description text
	DescStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Suggestion dropdown styles
	SuggestionBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1).
				MarginTop(0)

	SuggestionStyle = lipgloss.NewStyle().
				Foreground(textColor)

	SuggestionSelectedStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(primaryColor).
				Bold(true)

	// History badge style
	HistoryBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Background(lipgloss.Color("#064E3B")).
				Padding(0, 1).
				Bold(true)
)

// FrameStyle returns a style for the main TUI frame
func FrameStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width - 2).   // Account for border
		Height(height - 2). // Account for border
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor)
}

// ContentWidth returns available width for content inside the frame
func ContentWidth(terminalWidth int) int {
	// Frame border (2) + frame padding (4) = 6
	return terminalWidth - 6
}
