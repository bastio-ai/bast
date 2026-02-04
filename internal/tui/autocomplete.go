package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bastio-ai/bast/internal/files"
)

// checkForMention scans input for an active @mention and triggers search if needed
func (m Model) checkForMention() (Model, tea.Cmd) {
	value := m.textInput.Value()
	cursor := m.textInput.Position()

	// Find the last @ before cursor
	atPos := -1
	for i := cursor - 1; i >= 0; i-- {
		if value[i] == '@' {
			atPos = i
			break
		}
		// Stop if we hit a space (no active mention)
		if value[i] == ' ' {
			break
		}
	}

	if atPos == -1 {
		// No @ found, close suggestions
		m.showSuggestions = false
		m.suggestions = nil
		return m, nil
	}

	// Extract the mention text (between @ and cursor)
	mentionText := value[atPos+1 : cursor]

	// Check if we've already searched for this
	if mentionText == m.lastMentionText && m.showSuggestions {
		return m, nil
	}

	m.mentionStart = atPos
	m.lastMentionText = mentionText
	m.searchingFiles = true

	// Trigger async search
	return m, m.searchFiles(mentionText)
}

// searchFiles returns a command that searches for files matching the prefix
func (m Model) searchFiles(prefix string) tea.Cmd {
	cwd := m.shellCtx.CWD
	return func() tea.Msg {
		results := files.ListFiles(cwd, prefix, files.MaxSuggestions)
		return SuggestionsMsg{Suggestions: results}
	}
}

// insertSuggestion inserts the selected suggestion into the text input
func (m Model) insertSuggestion() (tea.Model, tea.Cmd) {
	if len(m.suggestions) == 0 || m.selectedIndex >= len(m.suggestions) {
		return m, nil
	}

	selected := m.suggestions[m.selectedIndex]
	value := m.textInput.Value()
	cursor := m.textInput.Position()

	// Build new value: text before @, @suggestion, text after cursor
	newValue := value[:m.mentionStart] + "@" + selected
	if cursor < len(value) {
		newValue += value[cursor:]
	}

	m.textInput.SetValue(newValue)
	// Move cursor to end of inserted path
	m.textInput.SetCursor(m.mentionStart + 1 + len(selected))

	// Close suggestions
	m.showSuggestions = false
	m.suggestions = nil
	m.lastMentionText = ""

	return m, nil
}
