package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/bastio-ai/bast/internal/auth"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Bastio authentication",
	Long:  `Commands to manage authentication with Bastio AI Security Gateway.`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Bastio",
	Long:  `Authenticate with Bastio using the OAuth Device Flow. This will open your browser to complete the login.`,
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Bastio",
	Long:  `Clear stored Bastio credentials.`,
	RunE:  runLogout,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	Long:  `Display current Bastio authentication status and proxy information.`,
	RunE:  runStatus,
}

// Aliases at root level
var loginAliasCmd = &cobra.Command{
	Use:    "login",
	Short:  "Log in to Bastio (alias for 'auth login')",
	Hidden: true,
	RunE:   runLogin,
}

var logoutAliasCmd = &cobra.Command{
	Use:    "logout",
	Short:  "Log out from Bastio (alias for 'auth logout')",
	Hidden: true,
	RunE:   runLogout,
}

func init() {
	// Add auth subcommand to root
	rootCmd.AddCommand(authCmd)

	// Add subcommands to auth
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(statusCmd)

	// Add aliases to root
	rootCmd.AddCommand(loginAliasCmd)
	rootCmd.AddCommand(logoutAliasCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), auth.DefaultDeviceFlowTimeout)
	defer cancel()

	authenticator := auth.NewAuthenticator()

	fmt.Println("Logging in to Bastio...")
	fmt.Println()

	// Start the device flow
	authResp, err := authenticator.StartLogin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start login: %w", err)
	}

	opened, fallback := auth.OpenBrowserWithFallback(authResp.VerificationURL)
	if opened {
		fmt.Println("Opening browser for authentication...")
		fmt.Println()
	} else {
		fmt.Println(fallback)
		fmt.Println()
	}

	// Display the user code
	fmt.Println("┌──────────────────────────────────────┐")
	fmt.Printf("│  Enter this code: %-18s │\n", authResp.UserCode)
	fmt.Println("│  Waiting for authorization... ⣾      │")
	fmt.Println("└──────────────────────────────────────┘")
	fmt.Println()

	// Poll for the token
	creds, err := authenticator.CompleteLogin(ctx, authResp.DeviceCode, authResp.Interval, authResp.DeviceID)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Println("✓ Successfully logged in to Bastio!")
	fmt.Println()

	if creds.HasProxyCredentials() {
		fmt.Printf("Proxy ID: %s\n", creds.ProxyID)
		fmt.Println()

		// Prompt for Anthropic API key
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Enter your Anthropic API key to complete setup:")
		fmt.Println("(Get one at https://console.anthropic.com/)")
		fmt.Println("(Press Enter to skip - you can add it later in the Bastio dashboard)")
		fmt.Print("> ")
		apiKey, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKey)

		if apiKey != "" {
			fmt.Print("Storing API key with Bastio... ")
			if err := authenticator.StoreProviderKey(ctx, creds.ProxyAPIKey, "anthropic", apiKey); err != nil {
				fmt.Println("✗")
				fmt.Printf("Warning: Failed to store API key: %v\n", err)
				fmt.Println("You can add it later in the Bastio dashboard.")
			} else {
				fmt.Println("✓")
				fmt.Println()
				fmt.Println("Setup complete! You can now use bast.")
			}
		} else {
			fmt.Println()
			fmt.Println("Skipped. Add your Anthropic API key in the Bastio dashboard to use bast.")
		}
	}

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Check if logged in
	if !auth.CredentialsExist() {
		fmt.Println("Not currently logged in to Bastio.")
		return nil
	}

	fmt.Print("Are you sure you want to log out? [y/N]: ")
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		fmt.Println("Logout cancelled.")
		return nil
	}

	authenticator := auth.NewAuthenticator()
	if err := authenticator.Logout(); err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	fmt.Println("✓ Successfully logged out from Bastio.")
	fmt.Println()
	fmt.Println("Your Anthropic API key stored in Bastio has not been deleted.")
	fmt.Println("To remove it, visit: https://bastio.com/dashboard/proxies")

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	authenticator := auth.NewAuthenticator()

	status, err := authenticator.GetStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	fmt.Println("Bastio Authentication Status")
	fmt.Println("────────────────────────────")
	fmt.Println()

	if !status.LoggedIn {
		fmt.Println("Status: Not logged in")
		fmt.Println()
		fmt.Println("Run 'bast auth login' or 'bast init' to get started.")
		return nil
	}

	fmt.Println("Status: Logged in")
	fmt.Println()

	if status.HasProxySetup {
		fmt.Println("Proxy: Configured")
		fmt.Printf("Proxy ID: %s\n", status.ProxyID)
		fmt.Printf("Gateway URL: %s\n", status.BastioGatewayURL)
	} else {
		fmt.Println("Proxy: Not configured")
		fmt.Println()
		fmt.Println("Run 'bast init' to create a CLI proxy.")
	}

	fmt.Println()
	fmt.Printf("Credentials file: %s\n", status.CredentialsPath)
	fmt.Println()
	fmt.Println("Dashboard: https://bastio.com/dashboard/cli")

	return nil
}
