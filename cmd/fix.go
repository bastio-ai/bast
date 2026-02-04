package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/auth"
	"github.com/bastio-ai/bast/internal/config"
	"github.com/bastio-ai/bast/internal/shell"
)

var fixCmd = &cobra.Command{
	Use:   "fix [error-output]",
	Short: "Analyze and fix a failed command",
	Long: `Analyze the last failed command and suggest a fix.

Usage:
  bast fix                        # Fix last failed command using env vars
  bast fix "permission denied"    # Provide error context manually
  command 2>&1 | bast fix -       # Pipe error output to fix`,
	RunE: runFix,
}

func init() {
	rootCmd.AddCommand(fixCmd)
}

func runFix(cmd *cobra.Command, args []string) error {
	// Load config
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
	shellCtx := shell.GetContextWithHistory()

	// Determine failed command and error output
	var failedCmd, errorOutput string

	// Check if input is being piped
	stat, _ := os.Stdin.Stat()
	isPiped := (stat.Mode() & os.ModeCharDevice) == 0

	if isPiped || (len(args) > 0 && args[0] == "-") {
		// Read from stdin
		var sb strings.Builder
		scanner := bufio.NewScanner(os.Stdin)
		// Increase buffer size for large outputs
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)
		for scanner.Scan() {
			sb.WriteString(scanner.Text())
			sb.WriteString("\n")
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		errorOutput = strings.TrimSpace(sb.String())
		failedCmd = shellCtx.LastCommand
	} else if len(args) > 0 {
		// Error output provided as argument
		errorOutput = strings.Join(args, " ")
		failedCmd = shellCtx.LastCommand
	} else {
		// Use environment variables set by shell hook
		failedCmd = shellCtx.LastCommand
		errorOutput = shellCtx.LastError
		if errorOutput == "" {
			errorOutput = shellCtx.LastOutput // Sometimes errors go to stdout
		}
	}

	if failedCmd == "" && errorOutput == "" {
		fmt.Println("No failed command or error output found.")
		fmt.Println("\nUsage:")
		fmt.Println("  bast fix                     # Uses BAST_LAST_CMD and BAST_LAST_ERROR env vars")
		fmt.Println("  bast fix \"error message\"     # Provide error context")
		fmt.Println("  command 2>&1 | bast fix -    # Pipe error output")
		return nil
	}

	// Display what we're analyzing
	if failedCmd != "" {
		fmt.Printf("Analyzing: %s\n", failedCmd)
	}
	if errorOutput != "" {
		// Truncate for display
		displayError := errorOutput
		if len(displayError) > 200 {
			displayError = displayError[:200] + "..."
		}
		fmt.Printf("Error: %s\n", displayError)
	}
	fmt.Println()

	// Call AI to fix the command
	ctx := context.Background()
	result, err := provider.FixCommand(ctx, failedCmd, errorOutput, shellCtx)
	if err != nil {
		return fmt.Errorf("failed to analyze error: %w", err)
	}

	// Display result
	if result.WasFixed && result.FixedCommand != "" {
		fmt.Println("Suggested fix:")
		fmt.Printf("  %s\n", result.FixedCommand)
		if result.Explanation != "" {
			fmt.Printf("\n%s\n", result.Explanation)
		}
		// Print in format that can be captured by shell hook
		fmt.Printf("\nBAST_FIX:%s\n", result.FixedCommand)
	} else {
		fmt.Println("Analysis:")
		fmt.Printf("  %s\n", result.Explanation)
	}

	return nil
}
