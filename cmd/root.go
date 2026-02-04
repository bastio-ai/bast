package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "bast",
	Short: "AI Shell Assistant",
	Long: `bast is an AI-powered shell assistant that generates shell commands
using natural language. It integrates with your shell to provide
contextual command suggestions.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global flags can be added here
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path")
}
