package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/bastio-ai/bast/internal/auth"
	"github.com/bastio-ai/bast/internal/config"
)

// getBastPath returns the absolute path to the bast executable
func getBastPath() string {
	exePath, err := os.Executable()
	if err != nil {
		// Fall back to "bast" if we can't get the executable path
		return "bast"
	}
	return exePath
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize bast configuration",
	Long:  `Interactive setup wizard to configure bast with your API key and preferences.`,
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to bast setup!")
	fmt.Println()

	// Check if config already exists
	if config.ConfigExists() {
		fmt.Print("Configuration already exists. Overwrite? [y/N]: ")
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Setup cancelled.")
			return nil
		}
		fmt.Println()
	}

	cfg := &config.Config{
		Mode:     config.DefaultMode,
		Provider: config.DefaultProvider,
		Model:    config.DefaultModel,
		Gateway:  config.DefaultGateway,
	}

	// Ask about Bastio
	fmt.Println("Do you want to use Bastio AI Security? (recommended)")
	fmt.Println("Bastio adds enterprise-grade security including PII detection,")
	fmt.Println("jailbreak prevention, and threat detection.")
	fmt.Println()
	fmt.Println("[Y] Yes, secure with Bastio  [n] No, connect directly")
	fmt.Print("> ")
	bastioChoice, _ := reader.ReadString('\n')
	bastioChoice = strings.TrimSpace(strings.ToLower(bastioChoice))

	useBastio := bastioChoice != "n" && bastioChoice != "no"

	if useBastio {
		cfg.Gateway = config.GatewayBastio
		if err := runBastioSetup(reader, cfg); err != nil {
			return err
		}
	} else {
		cfg.Gateway = config.GatewayDirect
		if err := runDirectSetup(reader, cfg); err != nil {
			return err
		}
	}

	// Select model
	fmt.Println()
	fmt.Println("Select model:")
	fmt.Println("1. claude-sonnet-4-5-20250929 (recommended)")
	fmt.Println("2. claude-haiku-4-5-20251001 (faster, cheaper)")
	fmt.Println("3. claude-opus-4-5-20251101 (most capable)")
	fmt.Print("> ")
	modelChoice, _ := reader.ReadString('\n')
	modelChoice = strings.TrimSpace(modelChoice)

	switch modelChoice {
	case "2":
		cfg.Model = "claude-haiku-4-5-20251001"
	case "3":
		cfg.Model = "claude-opus-4-5-20251101"
	default:
		cfg.Model = "claude-sonnet-4-5-20250929"
	}

	// Select mode
	fmt.Println()
	fmt.Println("Select execution mode:")
	fmt.Println("1. safe - Always confirm before executing (recommended)")
	fmt.Println("2. yolo - Execute commands without confirmation")
	fmt.Print("> ")
	modeChoice, _ := reader.ReadString('\n')
	modeChoice = strings.TrimSpace(modeChoice)

	if modeChoice == "2" {
		cfg.Mode = "yolo"
	} else {
		cfg.Mode = "safe"
	}

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	configPath, _ := config.DefaultConfigPath()
	bastPath := getBastPath()
	fmt.Println()
	fmt.Printf("Configuration saved to %s\n", configPath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Add to ~/.zshrc: eval \"$(%s hook zsh)\"\n", bastPath)
	fmt.Println("  2. Restart your terminal or run: source ~/.zshrc")
	fmt.Println("  3. Press Ctrl+A to launch bast")

	if useBastio {
		fmt.Println()
		fmt.Println("View security dashboard: https://bastio.com/dashboard/cli")
	}

	return nil
}

func runBastioSetup(reader *bufio.Reader, cfg *config.Config) error {
	fmt.Println()

	// Check if already logged in
	creds, _ := auth.LoadCredentials()
	if creds != nil && creds.HasValidToken() {
		fmt.Println("You're already logged in to Bastio!")
		if creds.HasProxyCredentials() {
			fmt.Println("Your CLI proxy is configured.")
			cfg.Bastio.ProxyID = creds.ProxyID
			return nil
		}
		// Has token but no proxy - need to create one
		return runProxySetup(reader, cfg, creds)
	}

	// Ask if they have an account
	fmt.Println("Do you have a Bastio account?")
	fmt.Println("[Y] Yes, log me in  [n] No, create one")
	fmt.Print("> ")
	hasAccount, _ := reader.ReadString('\n')
	hasAccount = strings.TrimSpace(strings.ToLower(hasAccount))

	if hasAccount == "n" || hasAccount == "no" {
		fmt.Println()
		fmt.Println("Opening browser to create account...")
		opened, fallback := auth.OpenBrowserWithFallback("https://bastio.com/signup?ref=bast")
		if !opened {
			fmt.Println(fallback)
		}
		fmt.Println()
		fmt.Println("After creating your account, press Enter to continue...")
		reader.ReadString('\n')
	}

	// Perform device flow authentication
	ctx, cancel := context.WithTimeout(context.Background(), auth.DefaultDeviceFlowTimeout)
	defer cancel()

	authenticator := auth.NewAuthenticator()

	fmt.Println()
	fmt.Println("Opening browser for authentication...")

	authResp, err := authenticator.StartLogin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start login: %w", err)
	}

	verifyURL := authResp.VerificationURL

	opened, fallback := auth.OpenBrowserWithFallback(verifyURL)
	if !opened {
		fmt.Println(fallback)
	}
	fmt.Println()

	// Display the user code
	fmt.Println("┌──────────────────────────────────────┐")
	fmt.Printf("│  Enter this code: %-18s │\n", authResp.UserCode)
	fmt.Println("│  Waiting for authorization... ⣾      │")
	fmt.Println("└──────────────────────────────────────┘")
	fmt.Println()

	// Poll for the token
	creds, err = authenticator.CompleteLogin(ctx, authResp.DeviceCode, authResp.Interval, authResp.DeviceID)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Println("✓ Connected to Bastio!")
	fmt.Println()

	return runProxySetup(reader, cfg, creds)
}

func runProxySetup(reader *bufio.Reader, cfg *config.Config, creds *auth.Credentials) error {
	// Get Anthropic API key
	fmt.Println("Enter your Anthropic API key:")
	fmt.Println("(Stored securely with Bastio, never saved locally)")
	fmt.Println("(Get one at https://console.anthropic.com/)")
	fmt.Print("> ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		fmt.Println("API key is required for Bastio setup.")
		return fmt.Errorf("API key required")
	}

	fmt.Println()
	fmt.Print("Creating secure proxy... ")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	authenticator := auth.NewAuthenticator()
	proxyResp, err := authenticator.CreateProxy(ctx, creds.AccessToken, apiKey, cfg.Model)
	if err != nil {
		fmt.Println("✗")
		return fmt.Errorf("failed to create proxy: %w", err)
	}

	// Update credentials with proxy info
	if err := authenticator.UpdateProxyCredentials(ctx, proxyResp); err != nil {
		fmt.Println("✗")
		return fmt.Errorf("failed to save proxy credentials: %w", err)
	}

	cfg.Bastio.ProxyID = proxyResp.ProxyID

	fmt.Println("✓")
	fmt.Println()

	return nil
}

func runDirectSetup(reader *bufio.Reader, cfg *config.Config) error {
	fmt.Println()

	// Get API key
	fmt.Println("Enter your Anthropic API key:")
	fmt.Println("(Get one at https://console.anthropic.com/)")
	fmt.Print("> ")
	apiKey, _ := reader.ReadString('\n')
	apiKey = strings.TrimSpace(apiKey)

	if apiKey == "" {
		fmt.Println("API key is required. You can also set ANTHROPIC_API_KEY environment variable.")
		return nil
	}

	cfg.APIKey = apiKey

	return nil
}
