package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/auth"
	"github.com/bastio-ai/bast/internal/config"
	"github.com/bastio-ai/bast/internal/tui"
)

var (
	queryFlag      string
	outputFileFlag string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Launch the bast TUI",
	Long:  `Launch the interactive TUI to generate shell commands using AI.`,
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&queryFlag, "query", "q", "", "Initial query to process")
	runCmd.Flags().StringVar(&outputFileFlag, "output-file", "", "Write output to file (for shell integration)")
}

func runTUI(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve credentials based on gateway mode
	providerCfg, err := auth.ResolveProviderConfig(cfg)
	if err != nil {
		// Print user-friendly instructions and return the error
		fmt.Println(auth.FormatSetupInstructions(err))
		return err
	}

	// Create provider
	provider := ai.NewAnthropicProviderWithConfig(providerCfg)

	// Create and run TUI
	model := tui.NewModel(provider, queryFlag, outputFileFlag)
	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// The TUI prints BAST_COMMAND:xxx when a command is selected
	// The shell hook parses this to insert the command
	_ = finalModel

	return nil
}
