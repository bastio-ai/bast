package auth

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

const (
	// CredentialsFileName is the name of the credentials file
	CredentialsFileName = "credentials.yaml"

	// CredentialsFileMode is the file permission for the credentials file (owner read/write only)
	CredentialsFileMode = 0600
)

// Credentials holds the Bastio authentication credentials
type Credentials struct {
	AccessToken  string    `mapstructure:"access_token"`
	RefreshToken string    `mapstructure:"refresh_token"`
	ExpiresAt    time.Time `mapstructure:"expires_at"`
	ProxyAPIKey  string    `mapstructure:"proxy_api_key"`
	ProxyID      string    `mapstructure:"proxy_id"`
	DeviceID     string    `mapstructure:"device_id"`
}

// CredentialsFile wraps the credentials with the bastio section
type CredentialsFile struct {
	Bastio Credentials `mapstructure:"bastio"`
}

// CredentialsPath returns the path to the credentials file
func CredentialsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "bast", CredentialsFileName), nil
}

// LoadCredentials loads the Bastio credentials from disk
func LoadCredentials() (*Credentials, error) {
	credPath, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		return nil, nil // No credentials file yet
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(credPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read credentials: %w", err)
	}

	var credFile CredentialsFile
	if err := v.Unmarshal(&credFile); err != nil {
		return nil, fmt.Errorf("failed to parse credentials: %w", err)
	}

	return &credFile.Bastio, nil
}

// SaveCredentials saves the Bastio credentials to disk with secure permissions
func SaveCredentials(creds *Credentials) error {
	credPath, err := CredentialsPath()
	if err != nil {
		return err
	}

	// Ensure the config directory exists
	configDir := filepath.Dir(credPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	v := viper.New()
	v.SetConfigType("yaml")
	v.SetConfigFile(credPath)

	// Set the credentials under the bastio section
	v.Set("bastio.access_token", creds.AccessToken)
	v.Set("bastio.refresh_token", creds.RefreshToken)
	v.Set("bastio.expires_at", creds.ExpiresAt)
	v.Set("bastio.proxy_api_key", creds.ProxyAPIKey)
	v.Set("bastio.proxy_id", creds.ProxyID)
	v.Set("bastio.device_id", creds.DeviceID)

	// Write the config file
	if err := v.WriteConfigAs(credPath); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	// Set secure permissions (owner read/write only)
	if err := os.Chmod(credPath, CredentialsFileMode); err != nil {
		return fmt.Errorf("failed to set credentials file permissions: %w", err)
	}

	return nil
}

// DeleteCredentials removes the credentials file
func DeleteCredentials() error {
	credPath, err := CredentialsPath()
	if err != nil {
		return err
	}

	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	return nil
}

// CredentialsExist checks if credentials file exists
func CredentialsExist() bool {
	credPath, err := CredentialsPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(credPath)
	return err == nil
}

// IsExpired checks if the access token has expired
func (c *Credentials) IsExpired() bool {
	if c == nil || c.ExpiresAt.IsZero() {
		return true
	}
	// Consider expired if less than 5 minutes remaining
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

// HasValidToken checks if we have a valid (non-expired) access token
func (c *Credentials) HasValidToken() bool {
	return c != nil && c.AccessToken != "" && !c.IsExpired()
}

// HasProxyCredentials checks if we have valid proxy credentials
func (c *Credentials) HasProxyCredentials() bool {
	return c != nil && c.ProxyAPIKey != "" && c.ProxyID != ""
}
