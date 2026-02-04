package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View implements tea.Model
func (m Model) View() string {
	contentWidth := ContentWidth(m.width)
	var b strings.Builder

	b.WriteString(HeaderStyle.Render("bast"))
	b.WriteString(" ")
	b.WriteString(DescStyle.Render("AI Shell Assistant"))
	b.WriteString("\n\n")

	switch m.mode {
	case ModeInput:
		b.WriteString(m.renderInputMode(contentWidth))
	case ModeLoading:
		b.WriteString(m.renderLoadingMode())
	case ModeConfirm:
		b.WriteString(m.renderConfirmMode(contentWidth))
	case ModeChat:
		b.WriteString(m.renderChatMode(contentWidth))
	case ModeModelSelect:
		b.WriteString(m.renderModelSelectMode(contentWidth))
	case ModeAgent:
		b.WriteString(m.renderAgentMode(contentWidth))
	case ModeFix:
		b.WriteString(m.renderFixMode(contentWidth))
	}

	return FrameStyle(m.width, m.height).Render(b.String())
}

// renderInputMode renders the input mode view
func (m Model) renderInputMode(contentWidth int) string {
	var b strings.Builder

	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	if m.showSlashMenu && len(m.slashCommands) > 0 {
		b.WriteString(m.renderSlashMenu(contentWidth))
		b.WriteString("\n")
	} else if m.searchingFiles {
		b.WriteString(HelpStyle.Render("Searching files..."))
		b.WriteString("\n")
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString(m.renderSuggestions(contentWidth))
		b.WriteString("\n")
	}

	if m.err != nil {
		wrapped := lipgloss.NewStyle().Width(contentWidth).Render(
			ErrorStyle.Render(fmt.Sprintf("Error: %s", m.err.Error())))
		b.WriteString(wrapped)
		b.WriteString("\n")
	}

	if m.showSlashMenu && len(m.slashCommands) > 0 {
		b.WriteString(HelpStyle.Render("↑↓ navigate • Tab/Enter select • Esc cancel"))
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString(HelpStyle.Render("↑↓ navigate • Tab/Enter select • Esc cancel"))
	} else {
		b.WriteString(HelpStyle.Render("Enter to submit • Esc to quit"))
	}

	return b.String()
}

// renderLoadingMode renders the loading mode view
func (m Model) renderLoadingMode() string {
	var b strings.Builder

	b.WriteString(m.spinner.View())
	b.WriteString(" ")
	if m.loadingMessage != "" {
		b.WriteString(DescStyle.Render(m.loadingMessage))
	} else {
		b.WriteString(DescStyle.Render("Processing..."))
	}

	return b.String()
}

// renderConfirmMode renders the confirm mode view
func (m Model) renderConfirmMode(contentWidth int) string {
	var b strings.Builder

	// Show danger warning if command is dangerous
	if m.isDangerous {
		warningMsg := "⚠️  WARNING: This command may be destructive!"
		b.WriteString(ErrorStyle.Render(warningMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(DescStyle.Render("Generated command:"))
	b.WriteString("\n")
	wrapped := lipgloss.NewStyle().Width(contentWidth).Render(CommandStyle.Render(m.command))
	b.WriteString(wrapped)
	b.WriteString("\n")

	if m.explanation != "" {
		wrappedExplanation := ExplanationStyle.Width(contentWidth).Render(m.explanation)
		b.WriteString(wrappedExplanation)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	if m.isDangerous && !m.dangerConfirmed {
		b.WriteString(ErrorStyle.Render("Type 'yes' to confirm execution of this dangerous command"))
	} else {
		b.WriteString(m.renderHelp())
	}
	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("Or type a follow-up question and press Enter..."))

	return b.String()
}

// renderChatMode renders the chat mode view
func (m Model) renderChatMode(contentWidth int) string {
	var b strings.Builder

	if m.viewportReady && len(m.conversationHistory) > 0 {
		// Show scroll indicator if not at top
		if m.chatViewport.YOffset > 0 {
			b.WriteString(HelpStyle.Render("↑ more above"))
			b.WriteString("\n")
		}
		b.WriteString(m.chatViewport.View())
		// Show scroll indicator if not at bottom
		if !m.chatViewport.AtBottom() {
			b.WriteString("\n")
			b.WriteString(HelpStyle.Render("↓ more below"))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	if m.showSlashMenu && len(m.slashCommands) > 0 {
		b.WriteString(m.renderSlashMenu(contentWidth))
		b.WriteString("\n")
	} else if m.searchingFiles {
		b.WriteString(HelpStyle.Render("Searching files..."))
		b.WriteString("\n")
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString(m.renderSuggestions(contentWidth))
		b.WriteString("\n")
	}

	if m.showSlashMenu && len(m.slashCommands) > 0 {
		b.WriteString(HelpStyle.Render("↑↓ navigate • Tab/Enter select • Esc cancel"))
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString(HelpStyle.Render("↑↓ navigate • Tab/Enter select • Esc cancel"))
	} else {
		b.WriteString(HelpStyle.Render("Enter: send • ↑↓: scroll • Ctrl+N: new • Esc: quit"))
	}

	return b.String()
}

// renderConversationContent renders conversation history for the viewport
func (m Model) renderConversationContent() string {
	if len(m.conversationHistory) == 0 {
		return ""
	}
	contentWidth := ContentWidth(m.width)
	var b strings.Builder
	for i, msg := range m.conversationHistory {
		if msg.Role == "user" {
			b.WriteString(PromptStyle.Render("You: "))
			b.WriteString(msg.Content)
		} else {
			b.WriteString(DescStyle.Render("AI: "))
			styled, err := m.markdownRenderer.Render(msg.Content)
			if err != nil {
				styled = lipgloss.NewStyle().Width(contentWidth).Render(msg.Content)
			}
			styled = strings.TrimSuffix(styled, "\n")
			b.WriteString(styled)
		}
		if i < len(m.conversationHistory)-1 {
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// renderSlashMenu renders the slash command menu dropdown
func (m Model) renderSlashMenu(contentWidth int) string {
	innerWidth := contentWidth - 4
	var b strings.Builder
	for i, cmd := range m.slashCommands {
		if i > 0 {
			b.WriteString("\n")
		}
		line := fmt.Sprintf("%s - %s", cmd.Name, cmd.Description)
		if i == m.slashCursor {
			b.WriteString(SuggestionSelectedStyle.Width(innerWidth).Render("> " + line))
		} else {
			b.WriteString(SuggestionStyle.Width(innerWidth).Render("  " + line))
		}
	}
	return SuggestionBoxStyle.Render(b.String())
}

// renderSuggestions renders the file suggestion dropdown
func (m Model) renderSuggestions(contentWidth int) string {
	// Account for box border (2) and padding (2) to get inner width
	innerWidth := contentWidth - 4
	var b strings.Builder
	for i, suggestion := range m.suggestions {
		if i > 0 {
			b.WriteString("\n")
		}
		if i == m.selectedIndex {
			b.WriteString(SuggestionSelectedStyle.Width(innerWidth).Render("> " + suggestion))
		} else {
			b.WriteString(SuggestionStyle.Width(innerWidth).Render("  " + suggestion))
		}
	}
	return SuggestionBoxStyle.Render(b.String())
}

// renderHelp renders the help bar for confirm mode
func (m Model) renderHelp() string {
	keys := []struct {
		key  string
		desc string
	}{
		{"Enter", "execute"},
		{"e", "edit"},
		{"?", "explain"},
		{"n", "new"},
		{"Esc", "cancel"},
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %s",
			KeyStyle.Render(k.key),
			DescStyle.Render(k.desc),
		))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(parts, "  "))
}

// renderAgentMode renders the agent execution mode view
func (m Model) renderAgentMode(contentWidth int) string {
	var b strings.Builder

	if m.viewportReady {
		// Show scroll indicator if not at top
		if m.chatViewport.YOffset > 0 {
			b.WriteString(HelpStyle.Render("↑ more above"))
			b.WriteString("\n")
		}
		b.WriteString(m.chatViewport.View())
		// Show scroll indicator if not at bottom
		if !m.chatViewport.AtBottom() {
			b.WriteString("\n")
			b.WriteString(HelpStyle.Render("↓ more below"))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())
	b.WriteString("\n")

	if m.showSlashMenu && len(m.slashCommands) > 0 {
		b.WriteString(m.renderSlashMenu(contentWidth))
		b.WriteString("\n")
	} else if m.searchingFiles {
		b.WriteString(HelpStyle.Render("Searching files..."))
		b.WriteString("\n")
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString(m.renderSuggestions(contentWidth))
		b.WriteString("\n")
	}

	if m.showSlashMenu && len(m.slashCommands) > 0 {
		b.WriteString(HelpStyle.Render("↑↓ navigate • Tab/Enter select • Esc cancel"))
	} else if m.showSuggestions && len(m.suggestions) > 0 {
		b.WriteString(HelpStyle.Render("↑↓ navigate • Tab/Enter select • Esc cancel"))
	} else {
		b.WriteString(HelpStyle.Render("Enter: send • ↑↓: scroll • Ctrl+N: new • Esc: quit"))
	}

	return b.String()
}

// renderAgentContent renders the agent execution content for the viewport
func (m Model) renderAgentContent() string {
	contentWidth := ContentWidth(m.width)
	var b strings.Builder

	// Show tool calls
	toolCalls := m.agentToolCalls
	if m.agentResult != nil {
		toolCalls = m.agentResult.ToolCalls
	}

	if len(toolCalls) > 0 {
		b.WriteString(DescStyle.Render("Tool Calls:"))
		b.WriteString("\n")
		for _, call := range toolCalls {
			// Tool name and input
			toolLine := fmt.Sprintf("  %s %s", KeyStyle.Render(call.Name), string(call.Input))
			wrapped := lipgloss.NewStyle().Width(contentWidth).Render(toolLine)
			b.WriteString(wrapped)
			b.WriteString("\n")

			// Tool output (truncated if too long)
			output := call.Output
			if len(output) > 500 {
				output = output[:500] + "..."
			}
			if call.IsError {
				b.WriteString(ErrorStyle.Render("    Error: " + output))
			} else if output != "" {
				outputLines := strings.Split(output, "\n")
				if len(outputLines) > 5 {
					outputLines = append(outputLines[:5], "...")
				}
				for _, line := range outputLines {
					b.WriteString(HelpStyle.Render("    " + line))
					b.WriteString("\n")
				}
			}
			b.WriteString("\n")
		}
	}

	// Show final response
	if m.agentResult != nil && m.agentResult.Response != "" {
		b.WriteString("\n")
		b.WriteString(DescStyle.Render("Response:"))
		b.WriteString("\n")
		styled, err := m.markdownRenderer.Render(m.agentResult.Response)
		if err != nil {
			styled = lipgloss.NewStyle().Width(contentWidth).Render(m.agentResult.Response)
		}
		styled = strings.TrimSuffix(styled, "\n")
		b.WriteString(styled)

		// Show iteration count
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render(fmt.Sprintf("Completed in %d iteration(s) with %d tool call(s)",
			m.agentResult.Iterations, len(m.agentResult.ToolCalls))))
	}

	return b.String()
}

// renderFixMode renders the fix mode view
func (m Model) renderFixMode(contentWidth int) string {
	var b strings.Builder

	if m.fixResult == nil {
		b.WriteString(DescStyle.Render("No fix result available"))
		return b.String()
	}

	// Show the analysis result
	if m.fixResult.WasFixed && m.fixResult.FixedCommand != "" {
		// Show danger warning if the fixed command is dangerous
		if m.isDangerous {
			warningMsg := "WARNING: This command may be destructive!"
			b.WriteString(ErrorStyle.Render(warningMsg))
			b.WriteString("\n\n")
		}

		b.WriteString(DescStyle.Render("Suggested fix:"))
		b.WriteString("\n")
		wrapped := lipgloss.NewStyle().Width(contentWidth).Render(CommandStyle.Render(m.command))
		b.WriteString(wrapped)
		b.WriteString("\n")

		if m.fixResult.Explanation != "" {
			b.WriteString("\n")
			wrappedExplanation := ExplanationStyle.Width(contentWidth).Render(m.fixResult.Explanation)
			b.WriteString(wrappedExplanation)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		if m.isDangerous && !m.dangerConfirmed {
			b.WriteString(ErrorStyle.Render("Type 'yes' to confirm execution of this command"))
		} else {
			b.WriteString(m.renderFixHelp())
		}
	} else {
		// No automatic fix available, show explanation
		b.WriteString(DescStyle.Render("Analysis:"))
		b.WriteString("\n")
		if m.fixResult.Explanation != "" {
			wrappedExplanation := ExplanationStyle.Width(contentWidth).Render(m.fixResult.Explanation)
			b.WriteString(wrappedExplanation)
		} else {
			b.WriteString(HelpStyle.Render("Could not determine a fix for this error."))
		}
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("n: new query • Esc: quit"))
	}

	b.WriteString("\n\n")
	b.WriteString(m.textInput.View())

	return b.String()
}

// renderFixHelp renders the help bar for fix mode
func (m Model) renderFixHelp() string {
	keys := []struct {
		key  string
		desc string
	}{
		{"Enter", "execute fix"},
		{"n", "new query"},
		{"Esc", "cancel"},
	}

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %s",
			KeyStyle.Render(k.key),
			DescStyle.Render(k.desc),
		))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(parts, "  "))
}

// renderModelSelectMode renders the model selection menu
func (m Model) renderModelSelectMode(contentWidth int) string {
	var b strings.Builder

	b.WriteString(DescStyle.Render("Select Model"))
	b.WriteString("\n\n")

	// Render model options
	for i, opt := range m.modelOptions {
		cursor := "  "
		if i == m.modelCursor {
			cursor = "> "
		}

		line := fmt.Sprintf("%s%s", cursor, opt.Name)
		if opt.ID == m.currentModel {
			line += " (current)"
		}
		if opt.Description != "" {
			line += " - " + opt.Description
		}

		if i == m.modelCursor {
			b.WriteString(SuggestionSelectedStyle.Width(contentWidth).Render(line))
		} else {
			b.WriteString(SuggestionStyle.Width(contentWidth).Render(line))
		}
		b.WriteString("\n")
	}

	// Custom option
	cursor := "  "
	if m.modelCursor == len(m.modelOptions) {
		cursor = "> "
	}
	customLine := cursor + "Custom..."
	if m.modelCursor == len(m.modelOptions) {
		b.WriteString(SuggestionSelectedStyle.Width(contentWidth).Render(customLine))
	} else {
		b.WriteString(SuggestionStyle.Width(contentWidth).Render(customLine))
	}
	b.WriteString("\n")

	// Show text input when in custom mode
	if m.customModelInput {
		b.WriteString("\n")
		b.WriteString(m.textInput.View())
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(HelpStyle.Render("↑↓ navigate • Enter select • Esc back"))

	return b.String()
}
