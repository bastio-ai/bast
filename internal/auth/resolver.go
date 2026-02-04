package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/config"
)

// ErrNoAPIKey is returned when no API key is configured
type ErrNoAPIKey struct {
	ConfigExists bool
	ConfigPath   string
}

func (e *ErrNoAPIKey) Error() string {
	if !e.ConfigExists {
		return "no API key configured"
	}
	return "config file exists but has no API key"
}

// ResolveProviderConfig determines which credentials and base URL to use
// based on the configuration and environment variables.
//
// Resolution order:
//  1. Check BAST_GATEWAY=direct env override → use direct mode
//  2. Check BASTIO_API_KEY env var → use Bastio with that key
//  3. Check if Bastio credentials exist → use Bastio automatically
//  4. Fall back to direct mode with ANTHROPIC_API_KEY or config
func ResolveProviderConfig(cfg *config.Config) (ai.ProviderConfig, error) {
	providerCfg := ai.ProviderConfig{
		Model: cfg.Model,
	}

	// 1. Check for explicit direct mode override
	if os.Getenv("BAST_GATEWAY") == "direct" {
		return resolveDirectCredentials(cfg, providerCfg)
	}

	// 2. Check for BASTIO_API_KEY env var (explicit Bastio key)
	if bastioKey := os.Getenv("BASTIO_API_KEY"); bastioKey != "" {
		providerCfg.APIKey = bastioKey
		providerCfg.BaseURL = GetBastioGatewayURL()
		return providerCfg, nil
	}

	// 3. Check if Bastio credentials exist (auto-detect)
	creds, _ := LoadCredentials()
	if creds != nil && creds.HasProxyCredentials() {
		providerCfg.APIKey = creds.ProxyAPIKey
		providerCfg.DeviceID = creds.DeviceID
		// Use explicit guard endpoint with proxy_id
		// SDK adds /v1/messages, so final URL is: {base}/v1/guard/{proxy_id}/v1/messages
		providerCfg.BaseURL = fmt.Sprintf("%s/v1/guard/%s", GetBastioBaseURL(), creds.ProxyID)
		return providerCfg, nil
	}

	// 4. Fall back to direct mode
	return resolveDirectCredentials(cfg, providerCfg)
}

// ErrBastioNotConfigured is returned when Bastio gateway is enabled but not configured
type ErrBastioNotConfigured struct{}

func (e *ErrBastioNotConfigured) Error() string {
	return "Bastio gateway is enabled but not configured"
}

func resolveBastioCredentials(providerCfg ai.ProviderConfig) (ai.ProviderConfig, error) {
	// Load credentials from credentials.yaml
	creds, err := LoadCredentials()
	if err != nil {
		return providerCfg, fmt.Errorf("failed to load Bastio credentials: %w", err)
	}

	if creds == nil || !creds.HasProxyCredentials() {
		return providerCfg, &ErrBastioNotConfigured{}
	}

	providerCfg.APIKey = creds.ProxyAPIKey
	providerCfg.BaseURL = GetBastioGatewayURL()

	return providerCfg, nil
}

func resolveDirectCredentials(cfg *config.Config, providerCfg ai.ProviderConfig) (ai.ProviderConfig, error) {
	// Try environment variables first
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("BAST_API_KEY")
	}

	// Fall back to config file
	if apiKey == "" {
		apiKey = cfg.GetEffectiveAPIKey()
	}

	if apiKey == "" {
		// Determine if config file exists for better error message
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".config", "bast", "config.yaml")
		_, err := os.Stat(configPath)
		return providerCfg, &ErrNoAPIKey{
			ConfigExists: err == nil,
			ConfigPath:   configPath,
		}
	}

	providerCfg.APIKey = apiKey
	// BaseURL remains empty for direct Anthropic API access

	return providerCfg, nil
}

// BastioSecurityConfig holds configuration for Bastio Agent Security
type BastioSecurityConfig struct {
	BaseURL string
	ProxyID string
	APIKey  string
}

// GetBastioSecurityConfig extracts Bastio security configuration from credentials.
// Returns nil if Bastio is not configured or BAST_GATEWAY=direct is set.
func GetBastioSecurityConfig() *BastioSecurityConfig {
	// Check for explicit direct mode override
	if os.Getenv("BAST_GATEWAY") == "direct" {
		return nil
	}

	// Load credentials
	creds, err := LoadCredentials()
	if err != nil || creds == nil || !creds.HasProxyCredentials() {
		return nil
	}

	return &BastioSecurityConfig{
		BaseURL: GetBastioBaseURL(),
		ProxyID: creds.ProxyID,
		APIKey:  creds.ProxyAPIKey,
	}
}

// FormatSetupInstructions returns user-friendly setup instructions based on the error
func FormatSetupInstructions(err error) string {
	switch e := err.(type) {
	case *ErrBastioNotConfigured:
		return `Bastio gateway is enabled but not configured.

To set up Bastio:
  1. Run 'bast init' and choose Bastio
  2. Or run 'bast auth login' to authenticate

To use direct mode instead:
  1. Run 'bast init' and choose direct connection
  2. Or set BAST_GATEWAY=direct environment variable`

	case *ErrNoAPIKey:
		if !e.ConfigExists {
			return `No API key configured.

To get started, either:
  1. Run 'bast init' to create a config file
  2. Set the ANTHROPIC_API_KEY environment variable`
		}
		return fmt.Sprintf(`Config file exists but has no API key.

To fix this, either:
  1. Run 'bast init' to reconfigure
  2. Add 'api_key: your-key' to %s
  3. Set the ANTHROPIC_API_KEY environment variable`, e.ConfigPath)

	default:
		return err.Error()
	}
}
