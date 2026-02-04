package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/auth"
	"github.com/bastio-ai/bast/internal/config"
	"github.com/bastio-ai/bast/internal/shell"
	"github.com/bastio-ai/bast/internal/stdin"
)

var explainCmd = &cobra.Command{
	Use:   "explain [command or prompt]",
	Short: "Explain a command or piped output",
	Long: `Explain what a command does or analyze piped output.

Command mode (no pipe):
  bast explain "git stash"                          # Explain what command does
  bast explain "find . -name '*.go' -exec wc -l {}"  # Break down complex command

Output mode (with pipe):
  kubectl get pods | bast explain                    # Explain the output
  kubectl get pods | bast explain "any failing?"     # Ask specific question
  cat error.log | bast explain "why is it crashing"  # Analyze logs
  docker ps | bast explain                           # Explain container status`,
	RunE: runExplain,
}

func init() {
	rootCmd.AddCommand(explainCmd)
}

func runExplain(cmd *cobra.Command, args []string) error {
	// Load config first
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve credentials
	providerCfg, err := auth.ResolveProviderConfig(cfg)
	if err != nil {
		fmt.Println(auth.FormatSetupInstructions(err))
		return err
	}

	// Create provider
	provider := ai.NewAnthropicProviderWithConfig(providerCfg)

	// Get shell context
	shellCtx := shell.GetContext()

	// Determine mode: command mode vs output mode
	if len(args) > 0 && !stdin.IsPiped() {
		// Command mode: explain what the command does (no execution)
		command := strings.Join(args, " ")
		return explainCommand(command, provider, shellCtx)
	}

	if stdin.IsPiped() {
		// Output mode: explain piped output
		return explainOutput(provider, shellCtx, args)
	}

	// No input - show usage
	fmt.Println("No command or piped input provided.")
	fmt.Println("\nUsage:")
	fmt.Println("  bast explain \"git stash\"              # Explain what a command does")
	fmt.Println("  kubectl get pods | bast explain       # Explain piped output")
	fmt.Println("  cat error.log | bast explain \"why?\"   # Ask question about output")
	return nil
}

// explainCommand explains what a command does without executing it
func explainCommand(command string, provider *ai.AnthropicProvider, shellCtx ai.ShellContext) error {
	ctx := context.Background()
	explanation, err := provider.ExplainCommand(ctx, command)
	if err != nil {
		return fmt.Errorf("failed to explain command: %w", err)
	}

	fmt.Fprintln(os.Stdout, explanation)
	return nil
}

// explainOutput explains piped output
func explainOutput(provider *ai.AnthropicProvider, shellCtx ai.ShellContext, args []string) error {
	// Read piped input
	input, err := stdin.Read()
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	if input == "" {
		fmt.Println("No input received.")
		fmt.Println("\nNote: The pipe '|' only captures stdout. If the command outputs errors,")
		fmt.Println("use: command 2>&1 | bast explain")
		return nil
	}

	// Truncate if too large
	input = stdin.Truncate(input, stdin.MaxInputSize)

	// Get optional prompt from args
	var prompt string
	if len(args) > 0 {
		prompt = args[0]
	}

	// Call AI to explain the output
	ctx := context.Background()
	result, err := provider.ExplainOutput(ctx, input, prompt, shellCtx)
	if err != nil {
		return fmt.Errorf("failed to explain output: %w", err)
	}

	// Print the explanation
	fmt.Fprintln(os.Stdout, result.Response)
	return nil
}
