package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/config"
)

// handleKeyMsg handles keyboard input based on current mode
func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeInput:
		return m.handleInputModeKey(msg)
	case ModeLoading:
		return m.handleLoadingModeKey(msg)
	case ModeConfirm:
		return m.handleConfirmModeKey(msg)
	case ModeChat:
		return m.handleChatModeKey(msg)
	case ModeModelSelect:
		return m.handleModelSelectModeKey(msg)
	case ModeAgent:
		return m.handleAgentModeKey(msg)
	case ModeFix:
		return m.handleFixModeKey(msg)
	}

	// Update text input for unhandled modes
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleInputModeKey handles keys in input mode
func (m Model) handleInputModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle slash command menu navigation when visible
	if m.showSlashMenu && len(m.slashCommands) > 0 {
		switch msg.String() {
		case "up":
			if m.slashCursor > 0 {
				m.slashCursor--
			}
			return m, nil
		case "down":
			if m.slashCursor < len(m.slashCommands)-1 {
				m.slashCursor++
			}
			return m, nil
		case "tab", "enter":
			return m.executeSlashCommand(m.slashCommands[m.slashCursor].Name)
		case "esc":
			m.showSlashMenu = false
			m.textInput.SetValue("")
			return m, nil
		}
	}

	// Handle autocomplete navigation when suggestions are visible
	if m.showSuggestions && len(m.suggestions) > 0 {
		switch msg.String() {
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil
		case "down":
			if m.selectedIndex < len(m.suggestions)-1 {
				m.selectedIndex++
			}
			return m, nil
		case "tab", "enter":
			return m.insertSuggestion()
		case "esc":
			m.showSuggestions = false
			m.suggestions = nil
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.showSlashMenu {
			m.showSlashMenu = false
			m.textInput.SetValue("")
			return m, nil
		}
		if m.showSuggestions {
			m.showSuggestions = false
			m.suggestions = nil
			return m, nil
		}
		return m, tea.Quit
	case "enter":
		if m.showSlashMenu && len(m.slashCommands) > 0 {
			return m.executeSlashCommand(m.slashCommands[m.slashCursor].Name)
		}
		if m.showSuggestions {
			return m.insertSuggestion()
		}
		query := strings.TrimSpace(m.textInput.Value())
		if query == "" {
			return m, nil
		}
		// Intercept slash commands before intent classification
		if strings.HasPrefix(query, "/") {
			return m.handleSlashCommand(query)
		}
		m.mode = ModeLoading
		m.loadingMessage = "Classifying intent..."
		m.pendingQuery = query
		m.err = nil
		return m, tea.Batch(m.spinner.Tick, m.classifyIntent(query))
	}

	// Let textinput handle the key first
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Check for slash command after keystroke
	m = m.checkForSlashCommand()

	// Check for @mention after keystroke (only if not showing slash menu)
	if !m.showSlashMenu {
		var searchCmd tea.Cmd
		m, searchCmd = m.checkForMention()
		if searchCmd != nil {
			return m, tea.Batch(cmd, searchCmd)
		}
	}

	return m, cmd
}

// handleLoadingModeKey handles keys in loading mode
func (m Model) handleLoadingModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	}
	return m, nil
}

// handleConfirmModeKey handles keys in confirm mode
func (m Model) handleConfirmModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "enter", "y":
		query := strings.TrimSpace(m.textInput.Value())

		// For dangerous commands, require "yes" confirmation
		if m.isDangerous && !m.dangerConfirmed {
			if strings.ToLower(query) == "yes" {
				m.dangerConfirmed = true
				m.textInput.SetValue("")
				return m, nil
			} else if query != "" {
				// Treat as follow-up question about the dangerous command
				m.mode = ModeLoading
				m.loadingMessage = "Getting response..."
				m.pendingQuery = query
				m.textInput.SetValue("")
				return m, tea.Batch(m.spinner.Tick, m.chatAboutCommand(query, m.command))
			}
			// Empty enter on dangerous command - do nothing
			return m, nil
		}

		if query != "" {
			// Send as follow-up question with command context
			m.mode = ModeLoading
			m.loadingMessage = "Getting response..."
			m.pendingQuery = query
			m.textInput.SetValue("")
			return m, tea.Batch(m.spinner.Tick, m.chatAboutCommand(query, m.command))
		}

		// No text - execute the command
		if m.outputFile != "" {
			os.WriteFile(m.outputFile, []byte("BAST_COMMAND:"+m.command), 0600)
		} else {
			fmt.Printf("BAST_COMMAND:%s\n", m.command)
		}
		return m, tea.Quit

	case "e":
		// Edit mode - go back to input with command as value
		m.mode = ModeInput
		m.textInput.SetValue(m.command)
		m.textInput.Focus()
		m.command = ""
		m.explanation = ""
		m.resetAutocomplete()
		return m, textinput.Blink

	case "c":
		// Copy to clipboard (placeholder - would need clipboard library)
		return m, nil

	case "?":
		// Explain command
		if m.explanation == "" {
			return m, m.explainCommand(m.command)
		}
		// Toggle explanation off
		m.explanation = ""
		return m, nil

	case "n":
		// New query
		m.mode = ModeInput
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.command = ""
		m.explanation = ""
		m.resetAutocomplete()
		return m, textinput.Blink

	default:
		// Pass to textInput for typing follow-up questions
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// handleChatModeKey handles keys in chat mode
func (m Model) handleChatModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle slash command menu navigation when visible
	if m.showSlashMenu && len(m.slashCommands) > 0 {
		switch msg.String() {
		case "up":
			if m.slashCursor > 0 {
				m.slashCursor--
			}
			return m, nil
		case "down":
			if m.slashCursor < len(m.slashCommands)-1 {
				m.slashCursor++
			}
			return m, nil
		case "tab", "enter":
			return m.executeSlashCommand(m.slashCommands[m.slashCursor].Name)
		case "esc":
			m.showSlashMenu = false
			m.textInput.SetValue("")
			return m, nil
		}
	}

	// Handle autocomplete navigation when suggestions are visible
	if m.showSuggestions && len(m.suggestions) > 0 {
		switch msg.String() {
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil
		case "down":
			if m.selectedIndex < len(m.suggestions)-1 {
				m.selectedIndex++
			}
			return m, nil
		case "tab", "enter":
			return m.insertSuggestion()
		case "esc":
			m.showSuggestions = false
			m.suggestions = nil
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// If input has text, clear it; otherwise quit
		if m.textInput.Value() != "" {
			m.textInput.SetValue("")
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+n":
		// New conversation - clear history and go to input mode
		m.conversationHistory = nil
		m.chatResponse = ""
		m.mode = ModeInput
		m.textInput.SetValue("")
		m.textInput.Focus()
		// Clear viewport content
		if m.viewportReady {
			m.chatViewport.SetContent("")
		}
		m.resetAutocomplete()
		return m, textinput.Blink

	case "up":
		// Scroll up when input is empty
		if m.textInput.Value() == "" {
			m.chatViewport.ScrollUp(1)
			return m, nil
		}

	case "down":
		// Scroll down when input is empty
		if m.textInput.Value() == "" {
			m.chatViewport.ScrollDown(1)
			return m, nil
		}

	case "pgup", "ctrl+u":
		m.chatViewport.HalfPageUp()
		return m, nil

	case "pgdown", "ctrl+d":
		m.chatViewport.HalfPageDown()
		return m, nil

	case "enter":
		query := strings.TrimSpace(m.textInput.Value())
		if query == "" {
			return m, nil
		}
		m.mode = ModeLoading
		m.loadingMessage = "Classifying intent..."
		m.textInput.SetValue("")
		return m, tea.Batch(m.spinner.Tick, m.classifyIntent(query))
	}

	// Pass key to text input for typing
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Check for slash command after keystroke
	m = m.checkForSlashCommand()

	// Check for @mention after keystroke (only if not showing slash menu)
	if !m.showSlashMenu {
		var searchCmd tea.Cmd
		m, searchCmd = m.checkForMention()
		if searchCmd != nil {
			return m, tea.Batch(cmd, searchCmd)
		}
	}
	return m, cmd
}

// checkForSlashCommand checks if input starts with "/" and shows the command menu
func (m Model) checkForSlashCommand() Model {
	val := m.textInput.Value()
	if strings.HasPrefix(val, "/") {
		matches := FilterCommands(val)
		if len(matches) > 0 {
			m.showSlashMenu = true
			m.slashCommands = matches
			if m.slashCursor >= len(matches) {
				m.slashCursor = 0
			}
		} else {
			m.showSlashMenu = false
		}
	} else {
		m.showSlashMenu = false
	}
	return m
}

// executeSlashCommand executes the selected slash command from the menu
func (m Model) executeSlashCommand(cmdName string) (tea.Model, tea.Cmd) {
	m.showSlashMenu = false

	// Commands that require arguments: set prefix and let user continue typing
	if cmdName == "/agent" {
		m.textInput.SetValue("/agent ")
		m.textInput.SetCursor(len("/agent "))
		return m, nil
	}

	// Commands without arguments: execute immediately
	m.textInput.SetValue("")
	return m.handleSlashCommand(cmdName)
}

// handleFixModeKey handles keys in fix mode
func (m Model) handleFixModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit

	case "enter", "y":
		// Execute the fixed command if available
		if m.fixResult != nil && m.fixResult.WasFixed && m.command != "" {
			// For dangerous commands, require confirmation
			if m.isDangerous && !m.dangerConfirmed {
				query := strings.TrimSpace(m.textInput.Value())
				if strings.ToLower(query) == "yes" {
					m.dangerConfirmed = true
					m.textInput.SetValue("")
					return m, nil
				}
				return m, nil
			}

			// Output the fixed command
			if m.outputFile != "" {
				os.WriteFile(m.outputFile, []byte("BAST_COMMAND:"+m.command), 0600)
			} else {
				fmt.Printf("BAST_COMMAND:%s\n", m.command)
			}
			return m, tea.Quit
		}
		return m, nil

	case "n":
		// New query - go back to input mode
		m.mode = ModeInput
		m.fixResult = nil
		m.command = ""
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.resetAutocomplete()
		return m, textinput.Blink
	}

	// Pass to textInput for typing
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleSlashCommand handles slash commands like /model
func (m Model) handleSlashCommand(query string) (tea.Model, tea.Cmd) {
	switch {
	case strings.HasPrefix(query, "/model"):
		// Load current model from config
		cfg, err := config.Load()
		if err != nil {
			m.err = fmt.Errorf("failed to load config: %w", err)
			return m, nil
		}
		m.currentModel = cfg.Model
		m.modelOptions = ai.GetModelsForProvider(cfg.Provider)
		m.modelCursor = 0
		m.customModelInput = false
		m.mode = ModeModelSelect
		m.textInput.SetValue("")
		m.err = nil
		return m, nil
	case strings.HasPrefix(query, "/agent"):
		// Extract query after /agent command
		agentQuery := strings.TrimSpace(strings.TrimPrefix(query, "/agent"))
		if agentQuery == "" {
			m.err = fmt.Errorf("usage: /agent <task description>")
			return m, nil
		}
		m.mode = ModeLoading
		m.loadingMessage = "Running agent..."
		m.pendingQuery = agentQuery
		m.agentToolCalls = nil // Reset tool calls
		m.agentResult = nil
		m.err = nil
		// Note: We can't easily send updates during execution in the current architecture.
		// Tool calls will be shown in the final result.
		return m, tea.Batch(m.spinner.Tick, m.runAgent(agentQuery, nil))
	case strings.HasPrefix(query, "/fix"):
		m.mode = ModeLoading
		m.loadingMessage = "Analyzing error..."
		m.fixResult = nil
		m.command = ""
		m.err = nil
		return m, tea.Batch(m.spinner.Tick, m.fixCommand())
	default:
		m.err = fmt.Errorf("unknown command: %s", query)
		return m, nil
	}
}

// handleAgentModeKey handles keys in agent mode
func (m Model) handleAgentModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle slash command menu navigation when visible
	if m.showSlashMenu && len(m.slashCommands) > 0 {
		switch msg.String() {
		case "up":
			if m.slashCursor > 0 {
				m.slashCursor--
			}
			return m, nil
		case "down":
			if m.slashCursor < len(m.slashCommands)-1 {
				m.slashCursor++
			}
			return m, nil
		case "tab", "enter":
			return m.executeSlashCommand(m.slashCommands[m.slashCursor].Name)
		case "esc":
			m.showSlashMenu = false
			m.textInput.SetValue("")
			return m, nil
		}
	}

	// Handle autocomplete navigation when suggestions are visible
	if m.showSuggestions && len(m.suggestions) > 0 {
		switch msg.String() {
		case "up":
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}
			return m, nil
		case "down":
			if m.selectedIndex < len(m.suggestions)-1 {
				m.selectedIndex++
			}
			return m, nil
		case "tab", "enter":
			return m.insertSuggestion()
		case "esc":
			m.showSuggestions = false
			m.suggestions = nil
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// If input has text, clear it; otherwise quit
		if m.textInput.Value() != "" {
			m.textInput.SetValue("")
			return m, nil
		}
		return m, tea.Quit

	case "ctrl+n":
		// New conversation - clear history and go to input mode
		m.conversationHistory = nil
		m.agentResult = nil
		m.agentToolCalls = nil
		m.mode = ModeInput
		m.textInput.SetValue("")
		m.textInput.Focus()
		// Clear viewport content
		if m.viewportReady {
			m.chatViewport.SetContent("")
		}
		m.resetAutocomplete()
		return m, textinput.Blink

	case "up":
		// Scroll up when input is empty
		if m.textInput.Value() == "" {
			m.chatViewport.ScrollUp(1)
			return m, nil
		}

	case "down":
		// Scroll down when input is empty
		if m.textInput.Value() == "" {
			m.chatViewport.ScrollDown(1)
			return m, nil
		}

	case "pgup", "ctrl+u":
		m.chatViewport.HalfPageUp()
		return m, nil

	case "pgdown", "ctrl+d":
		m.chatViewport.HalfPageDown()
		return m, nil

	case "enter":
		query := strings.TrimSpace(m.textInput.Value())
		if query == "" {
			return m, nil
		}
		// Check for slash commands
		if strings.HasPrefix(query, "/") {
			return m.handleSlashCommand(query)
		}
		// Run another agent task
		m.mode = ModeLoading
		m.loadingMessage = "Running agent..."
		m.agentToolCalls = nil
		m.agentResult = nil
		m.textInput.SetValue("")
		return m, tea.Batch(m.spinner.Tick, m.runAgent(query, nil))
	}

	// Pass key to text input for typing
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Check for slash command after keystroke
	m = m.checkForSlashCommand()

	// Check for @mention after keystroke (only if not showing slash menu)
	if !m.showSlashMenu {
		var searchCmd tea.Cmd
		m, searchCmd = m.checkForMention()
		if searchCmd != nil {
			return m, tea.Batch(cmd, searchCmd)
		}
	}
	return m, cmd
}

// handleModelSelectModeKey handles keys in model selection mode
func (m Model) handleModelSelectModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.customModelInput {
		// Handle text input for custom model
		switch msg.String() {
		case "enter":
			customModel := strings.TrimSpace(m.textInput.Value())
			if customModel != "" {
				return m.selectModel(customModel)
			}
			return m, nil
		case "esc":
			m.customModelInput = false
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Describe what you want to do..."
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "up", "k":
		if m.modelCursor > 0 {
			m.modelCursor--
		}
	case "down", "j":
		if m.modelCursor < len(m.modelOptions) { // +1 for Custom option
			m.modelCursor++
		}
	case "enter":
		if m.modelCursor == len(m.modelOptions) {
			// Custom option selected
			m.customModelInput = true
			m.textInput.SetValue("")
			m.textInput.Placeholder = "Enter model ID..."
			m.textInput.Focus()
			return m, textinput.Blink
		}
		return m.selectModel(m.modelOptions[m.modelCursor].ID)
	case "esc":
		m.mode = ModeInput
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Describe what you want to do..."
		return m, textinput.Blink
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}
