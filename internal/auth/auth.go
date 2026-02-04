package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Authenticator handles Bastio authentication and proxy management
type Authenticator struct {
	deviceFlow *DeviceFlowClient
	baseURL    string
}

// NewAuthenticator creates a new authenticator instance
func NewAuthenticator() *Authenticator {
	return &Authenticator{
		deviceFlow: NewDeviceFlowClient(),
		baseURL:    GetBastioBaseURL(),
	}
}

// NewAuthenticatorWithURL creates an authenticator with a custom base URL (for testing)
func NewAuthenticatorWithURL(baseURL string) *Authenticator {
	return &Authenticator{
		deviceFlow: NewDeviceFlowClientWithURL(baseURL),
		baseURL:    baseURL,
	}
}

// LoginResult contains the result of a successful login
type LoginResult struct {
	Credentials *Credentials
	UserCode    string
	VerifyURL   string
}

// StartLogin initiates the device flow login process
func (a *Authenticator) StartLogin(ctx context.Context) (*DeviceAuthorizationResponse, error) {
	return a.deviceFlow.StartDeviceFlow(ctx)
}

// CompleteLogin polls for the token and saves credentials
func (a *Authenticator) CompleteLogin(ctx context.Context, deviceCode string, interval int, deviceID string) (*Credentials, error) {
	tokenResp, err := a.deviceFlow.PollForToken(ctx, deviceCode, interval)
	if err != nil {
		return nil, err
	}

	// The device flow returns api_key and proxy_id directly
	// No separate access_token/refresh_token - API keys don't expire
	creds := &Credentials{
		ProxyAPIKey: tokenResp.APIKey,
		ProxyID:     tokenResp.ProxyID,
		DeviceID:    deviceID,
	}

	if err := SaveCredentials(creds); err != nil {
		return nil, fmt.Errorf("failed to save credentials: %w", err)
	}

	return creds, nil
}

// Logout clears stored credentials
func (a *Authenticator) Logout() error {
	return DeleteCredentials()
}

// GetCredentials loads credentials (API keys don't expire, no refresh needed)
func (a *Authenticator) GetCredentials(ctx context.Context) (*Credentials, error) {
	return LoadCredentials()
}

// ProxyCreateRequest is the request to create a CLI proxy
type ProxyCreateRequest struct {
	Name           string `json:"name"`
	MachineID      string `json:"machine_id"`
	Provider       string `json:"provider"`
	ProviderAPIKey string `json:"provider_api_key"`
	DefaultModel   string `json:"default_model"`
}

// ProxyCreateResponse is the response from creating a CLI proxy
type ProxyCreateResponse struct {
	ProxyID     string `json:"proxy_id"`
	ProxyAPIKey string `json:"proxy_api_key"`
	BaseURL     string `json:"base_url"`
}

// CreateProxy creates a new CLI proxy in Bastio
func (a *Authenticator) CreateProxy(ctx context.Context, accessToken, providerAPIKey, model string) (*ProxyCreateResponse, error) {
	url := a.baseURL + "/v1/cli/proxies"

	machineID := generateMachineID()
	hostname, _ := os.Hostname()
	proxyName := fmt.Sprintf("bast-cli-%s", hostname)

	reqBody := ProxyCreateRequest{
		Name:           proxyName,
		MachineID:      machineID,
		Provider:       "anthropic",
		ProviderAPIKey: providerAPIKey,
		DefaultModel:   model,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: DefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("proxy creation failed (status %d): %s", resp.StatusCode, string(body))
	}

	var proxyResp ProxyCreateResponse
	if err := json.Unmarshal(body, &proxyResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &proxyResp, nil
}

// UpdateProxyCredentials updates the stored credentials with proxy information
func (a *Authenticator) UpdateProxyCredentials(ctx context.Context, proxyResp *ProxyCreateResponse) error {
	creds, err := LoadCredentials()
	if err != nil {
		return err
	}
	if creds == nil {
		return fmt.Errorf("no credentials found")
	}

	creds.ProxyID = proxyResp.ProxyID
	creds.ProxyAPIKey = proxyResp.ProxyAPIKey

	return SaveCredentials(creds)
}

// AuthStatus represents the current authentication status
type AuthStatus struct {
	LoggedIn         bool
	HasValidToken    bool
	HasProxySetup    bool
	ProxyID          string
	CredentialsPath  string
	BastioGatewayURL string
}

// GetStatus returns the current authentication status
func (a *Authenticator) GetStatus(ctx context.Context) (*AuthStatus, error) {
	credPath, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	status := &AuthStatus{
		CredentialsPath:  credPath,
		BastioGatewayURL: GetBastioGatewayURL(),
	}

	creds, err := LoadCredentials()
	if err != nil {
		return status, nil
	}
	if creds == nil {
		return status, nil
	}

	// With the simplified auth flow, being logged in means having a proxy API key
	status.LoggedIn = creds.HasProxyCredentials()
	status.HasValidToken = creds.HasProxyCredentials() // API keys don't expire
	status.HasProxySetup = creds.HasProxyCredentials()
	status.ProxyID = creds.ProxyID

	return status, nil
}

// generateMachineID generates a unique identifier for this machine
func generateMachineID() string {
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME") // Windows
	}

	// Create a hash of hostname + username
	data := hostname + ":" + username
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)
}

// ProviderKeyRequest is the request to store a provider API key
type ProviderKeyRequest struct {
	Provider       string `json:"provider"`
	ProviderAPIKey string `json:"provider_api_key"`
}

// StoreProviderKey stores a provider API key (e.g., Anthropic) with Bastio
// This is called after successful device auth to associate the provider key with the CLI proxy
func (a *Authenticator) StoreProviderKey(ctx context.Context, bastioAPIKey, provider, providerAPIKey string) error {
	url := a.baseURL + "/cli/auth/provider-key"

	reqBody := ProviderKeyRequest{
		Provider:       provider,
		ProviderAPIKey: providerAPIKey,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bastioAPIKey)

	client := &http.Client{Timeout: DefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to store provider key: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to store provider key (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetGatewayConfig returns the configuration needed to use the Bastio gateway
func GetGatewayConfig() (baseURL string, apiKey string, err error) {
	// First check environment variable
	if envKey := os.Getenv("BASTIO_API_KEY"); envKey != "" {
		return GetBastioGatewayURL(), envKey, nil
	}

	// Load credentials
	creds, err := LoadCredentials()
	if err != nil {
		return "", "", fmt.Errorf("failed to load credentials: %w", err)
	}
	if creds == nil || !creds.HasProxyCredentials() {
		return "", "", fmt.Errorf("no Bastio proxy configured. Run 'bast auth login' or 'bast init' to set up")
	}

	return GetBastioGatewayURL(), creds.ProxyAPIKey, nil
}
