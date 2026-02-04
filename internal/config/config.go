package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
// For direct mode, the API key is stored at the root level (api_key).
// For Bastio mode, credentials are stored separately in credentials.yaml.
type Config struct {
	Mode     string `mapstructure:"mode"`     // "safe" or "yolo"
	Provider string `mapstructure:"provider"` // AI provider (e.g., "anthropic")
	APIKey   string `mapstructure:"api_key"`  // API key for direct mode
	Model    string `mapstructure:"model"`    // Model to use (e.g., "claude-sonnet-4-20250514")
	Gateway  string `mapstructure:"gateway"`  // "bastio" or "direct"

	// Bastio contains settings for Bastio gateway connection
	Bastio BastioConfig `mapstructure:"bastio"`
}

// BastioConfig holds settings for Bastio gateway connection
type BastioConfig struct {
	ProxyID string `mapstructure:"proxy_id"`
}

const (
	DefaultMode     = "safe"
	DefaultProvider = "anthropic"
	DefaultModel    = "claude-sonnet-4-5-20250929"
	DefaultGateway  = "direct" // "bastio" or "direct"

	// Gateway modes
	GatewayBastio = "bastio"
	GatewayDirect = "direct"
)

func DefaultConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "bast"), nil
}

func DefaultConfigPath() (string, error) {
	configDir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

func Load() (*Config, error) {
	configDir, err := DefaultConfigDir()
	if err != nil {
		return nil, err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Set defaults
	viper.SetDefault("mode", DefaultMode)
	viper.SetDefault("provider", DefaultProvider)
	viper.SetDefault("model", DefaultModel)
	viper.SetDefault("gateway", DefaultGateway)

	// Allow environment variable overrides
	viper.SetEnvPrefix("BAST")
	viper.AutomaticEnv()

	// Read config file (if exists)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
		// Config file not found is okay, we use defaults
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	configDir, err := DefaultConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	viper.Set("mode", cfg.Mode)
	viper.Set("provider", cfg.Provider)
	viper.Set("model", cfg.Model)
	viper.Set("gateway", cfg.Gateway)

	// Only save API key for direct mode
	if cfg.Gateway == GatewayDirect && cfg.APIKey != "" {
		viper.Set("api_key", cfg.APIKey)
	}

	// Save bastio config if set
	if cfg.Bastio.ProxyID != "" {
		viper.Set("bastio.proxy_id", cfg.Bastio.ProxyID)
	}

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func ConfigExists() bool {
	configPath, err := DefaultConfigPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(configPath)
	return err == nil
}

// GetEffectiveGateway returns the effective gateway mode, considering environment overrides
func (c *Config) GetEffectiveGateway() string {
	// Environment variable override takes precedence
	if envGateway := os.Getenv("BAST_GATEWAY"); envGateway != "" {
		return envGateway
	}
	if c.Gateway == "" {
		return DefaultGateway
	}
	return c.Gateway
}

// IsBastioEnabled returns true if Bastio gateway is enabled
func (c *Config) IsBastioEnabled() bool {
	return c.GetEffectiveGateway() == GatewayBastio
}

// GetEffectiveAPIKey returns the API key to use for direct mode.
// For Bastio mode, returns empty (caller should use credentials from auth package).
func (c *Config) GetEffectiveAPIKey() string {
	return c.APIKey
}
