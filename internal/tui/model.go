package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/files"
	"github.com/bastio-ai/bast/internal/shell"
)

// Mode represents the current TUI mode
type Mode int

const (
	ModeInput Mode = iota
	ModeLoading
	ModeConfirm
	ModeChat        // Display chat response
	ModeModelSelect // Model selection menu
	ModeAgent       // Agentic task execution
	ModeFix         // Fix failed command
)

// Model is the main Bubble Tea model
type Model struct {
	mode     Mode
	textInput textinput.Model
	spinner   spinner.Model
	provider  ai.Provider
	shellCtx  ai.ShellContext

	// Command state
	command         string
	explanation     string
	chatResponse    string // Response for chat intent
	pendingQuery    string // Query being processed (for routing after classification)
	err             error
	isDangerous     bool   // True if current command matches dangerous patterns
	dangerConfirmed bool   // True if user has confirmed a dangerous command

	// Display dimensions
	width  int
	height int

	// Startup state
	initialQuery string
	outputFile   string // Path to write BAST_COMMAND output (for shell integration)

	// Loading state
	loadingMessage string // Current operation being performed

	// Autocomplete state
	showSuggestions  bool
	suggestions      []string
	selectedIndex    int
	mentionStart     int    // Position of "@" in input
	lastMentionText  string // Last searched mention text (to avoid duplicate searches)
	searchingFiles   bool   // True while file search is in progress

	// Conversation history for multi-turn chat
	conversationHistory []ai.ConversationMessage

	// Markdown renderer for chat responses
	markdownRenderer *glamour.TermRenderer

	// Viewport for scrollable chat content
	chatViewport  viewport.Model
	viewportReady bool // True once viewport initialized with dimensions

	// Model selection state
	modelOptions     []ai.ModelOption
	modelCursor      int
	customModelInput bool   // true when typing custom model ID
	currentModel     string // loaded from config on init

	// Slash command menu state
	showSlashMenu bool
	slashCommands []SlashCommand
	slashCursor   int

	// Agent mode state
	agentResult    *ai.AgentResult // Result of agentic execution
	agentToolCalls []ai.ToolCall   // Live tool calls during execution

	// Fix mode state
	fixResult *ai.FixResult // Result of fix command analysis

}

// NewModel creates a new TUI model
func NewModel(provider ai.Provider, initialQuery string, outputFile string) Model {
	ti := textinput.New()
	ti.Placeholder = "Describe what you want to do..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 60
	ti.PromptStyle = PromptStyle
	ti.Prompt = "❯ "

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	shellCtx := shell.GetContext()

	// Initialize markdown renderer with dark style
	// Note: WithAutoStyle() sends OSC escape sequences that conflict with Bubble Tea
	// Use a default width; will be updated on WindowSizeMsg
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
	)

	m := Model{
		mode:             ModeInput,
		textInput:        ti,
		spinner:          s,
		provider:         provider,
		shellCtx:         shellCtx,
		initialQuery:     initialQuery,
		outputFile:       outputFile,
		markdownRenderer: renderer,
	}

	// If initial query provided, set it and prepare loading message
	if initialQuery != "" {
		ti.SetValue(initialQuery)
		m.textInput = ti
		m.loadingMessage = "Classifying intent..."
	}

	return m
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}

	// If we have an initial query, start classifying intent immediately
	if m.initialQuery != "" {
		m.mode = ModeLoading
		cmds = append(cmds, m.spinner.Tick, m.classifyIntent(m.initialQuery))
	}

	return tea.Batch(cmds...)
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Re-create markdown renderer with new width
		contentWidth := ContentWidth(msg.Width)
		renderer, _ := glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(contentWidth),
		)
		m.markdownRenderer = renderer

		// Calculate viewport height (total - frame border/padding - header - input area)
		viewportHeight := msg.Height - 12 // Approximate: 2 border + 4 padding + 3 header + 3 input
		if viewportHeight < 1 {
			viewportHeight = 1
		}

		if !m.viewportReady {
			m.chatViewport = viewport.New(contentWidth, viewportHeight)
			m.viewportReady = true
		} else {
			m.chatViewport.Width = contentWidth
			m.chatViewport.Height = viewportHeight
		}

		if m.mode == ModeChat {
			m.chatViewport.SetContent(m.renderConversationContent())
		}
		return m, nil

	case CommandGeneratedMsg:
		m.mode = ModeConfirm
		m.command = msg.Result.Command
		m.explanation = msg.Result.Explanation
		m.isDangerous = isDangerousCommand(msg.Result.Command)
		m.dangerConfirmed = false
		m.textInput.SetValue("") // Clear any previous input
		m.textInput.Focus()      // Ready for follow-up questions
		m.resetAutocomplete()
		return m, textinput.Blink

	case CommandExplainedMsg:
		m.explanation = msg.Explanation
		return m, nil

	case IntentClassifiedMsg:
		if msg.Result.Intent == ai.IntentChat {
			// Route to chat handler, passing intent result for history detection
			m.loadingMessage = "Getting response..."
			return m, m.chat(msg.Query, msg.Result)
		}
		// Default to command generation
		m.loadingMessage = "Generating command..."
		return m, m.generateCommand(msg.Query)

	case ChatResponseMsg:
		m.mode = ModeChat
		m.chatResponse = msg.Result.Response
		// Append to conversation history (strip mentions to avoid policy violations in future context)
		m.conversationHistory = append(m.conversationHistory,
			ai.ConversationMessage{Role: "user", Content: files.StripMentions(msg.Query)},
			ai.ConversationMessage{Role: "assistant", Content: msg.Result.Response},
		)
		m.textInput.SetValue("") // Clear input for follow-up
		m.textInput.Focus()      // Ready for follow-up
		m.resetAutocomplete()
		// Update viewport with new content and scroll to bottom
		if m.viewportReady {
			m.chatViewport.SetContent(m.renderConversationContent())
			m.chatViewport.GotoBottom()
		}
		return m, textinput.Blink

	case ErrorMsg:
		m.err = msg.Err
		m.mode = ModeInput
		return m, nil

	case SuggestionsMsg:
		m.suggestions = msg.Suggestions
		m.selectedIndex = 0
		m.showSuggestions = len(msg.Suggestions) > 0
		m.searchingFiles = false
		return m, nil

	case ModelSelectedMsg:
		m.currentModel = msg.Model
		m.provider.SetModel(msg.Model)
		m.mode = ModeInput
		m.customModelInput = false
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Describe what you want to do..."
		m.textInput.Focus()
		return m, textinput.Blink

	case ToolCallMsg:
		// Append tool call to live list during agent execution
		m.agentToolCalls = append(m.agentToolCalls, msg.Call)
		// Update viewport content with new tool call
		if m.viewportReady {
			m.chatViewport.SetContent(m.renderAgentContent())
			m.chatViewport.GotoBottom()
		}
		return m, nil

	case AgentResponseMsg:
		m.mode = ModeAgent
		m.agentResult = msg.Result
		// Append to conversation history
		m.conversationHistory = append(m.conversationHistory,
			ai.ConversationMessage{Role: "user", Content: msg.Query},
			ai.ConversationMessage{Role: "assistant", Content: msg.Result.Response},
		)
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.resetAutocomplete()
		// Update viewport with final content
		if m.viewportReady {
			m.chatViewport.SetContent(m.renderAgentContent())
			m.chatViewport.GotoBottom()
		}
		return m, textinput.Blink

	case FixResultMsg:
		m.mode = ModeFix
		m.fixResult = msg.Result
		// If a fix was found, set it as the pending command
		if msg.Result.WasFixed && msg.Result.FixedCommand != "" {
			m.command = msg.Result.FixedCommand
			m.isDangerous = isDangerousCommand(msg.Result.FixedCommand)
			m.dangerConfirmed = false
		}
		m.textInput.SetValue("")
		m.textInput.Focus()
		m.resetAutocomplete()
		return m, textinput.Blink

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// resetAutocomplete clears all autocomplete state
func (m *Model) resetAutocomplete() {
	m.showSuggestions = false
	m.selectedIndex = 0
	m.suggestions = nil
	m.mentionStart = 0
	m.lastMentionText = ""
	m.searchingFiles = false
}

// SelectedCommand returns the command that was selected by the user
func (m Model) SelectedCommand() string {
	return m.command
}
