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
	"runtime"
	"time"
)

// DeviceAuthorizationResponse is the response from the device authorization endpoint
type DeviceAuthorizationResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	DeviceID        string `json:"device_id"` // Device ID for User-Agent header
}

// TokenResponse is the response from the token endpoint
// Matches Bastio's PollDeviceAuthResponse structure
type TokenResponse struct {
	Status   string `json:"status"`   // authorization_pending, authorized, expired, access_denied, slow_down
	APIKey   string `json:"api_key"`  // The Bastio API key for gateway requests
	ProxyID  string `json:"proxy_id"` // The proxy this key is scoped to
	Error    string `json:"error"`    // Error message if applicable
	Interval int    `json:"interval"` // Polling interval in seconds
}

// DeviceFlowClient handles the OAuth 2.0 Device Authorization Grant flow
type DeviceFlowClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewDeviceFlowClient creates a new device flow client
func NewDeviceFlowClient() *DeviceFlowClient {
	return &DeviceFlowClient{
		BaseURL: GetBastioBaseURL(),
		HTTPClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// NewDeviceFlowClientWithURL creates a client with a custom base URL (for testing)
func NewDeviceFlowClientWithURL(baseURL string) *DeviceFlowClient {
	return &DeviceFlowClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// StartDeviceFlow initiates the device authorization flow
func (c *DeviceFlowClient) StartDeviceFlow(ctx context.Context) (*DeviceAuthorizationResponse, error) {
	url := c.BaseURL + "/cli/auth/device"

	// Generate device ID from hostname + username
	deviceID := generateDeviceID()

	reqBody := map[string]string{
		"device_name": "bast-cli",
		"device_id":   deviceID,
		"os_info":     runtime.GOOS,
		"cli_version": CLIVersion,
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

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to start device flow: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device authorization failed (status %d): %s", resp.StatusCode, string(body))
	}

	var authResp DeviceAuthorizationResponse
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Use the device_id we sent if backend didn't return one
	if authResp.DeviceID == "" {
		authResp.DeviceID = deviceID
	}

	return &authResp, nil
}

// generateDeviceID generates a unique identifier for this device
func generateDeviceID() string {
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

// PollForToken polls the token endpoint until the user completes authorization
// or the device code expires. Returns the token response or an error.
func (c *DeviceFlowClient) PollForToken(ctx context.Context, deviceCode string, interval int) (*TokenResponse, error) {
	// Minimum interval is 5 seconds per RFC 8628
	if interval < 5 {
		interval = 5
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			tokenResp, err := c.requestToken(ctx, deviceCode)
			if err != nil {
				// Check if it's a polling error (authorization_pending, slow_down)
				if pollErr, ok := err.(*PollError); ok {
					if pollErr.ShouldRetry {
						if pollErr.SlowDown {
							// Back off by increasing the interval
							ticker.Reset(time.Duration(interval+5) * time.Second)
						}
						continue
					}
				}
				return nil, err
			}
			return tokenResp, nil
		}
	}
}

// PollError represents an error during token polling
type PollError struct {
	Code        string
	Description string
	ShouldRetry bool
	SlowDown    bool
}

func (e *PollError) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Description)
	}
	return e.Code
}

func (c *DeviceFlowClient) requestToken(ctx context.Context, deviceCode string) (*TokenResponse, error) {
	tokenURL := c.BaseURL + "/cli/auth/token"

	reqBody := map[string]string{
		"device_code": deviceCode,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Check status field for polling states
	switch tokenResp.Status {
	case "authorized":
		return &tokenResp, nil
	case "authorization_pending":
		return nil, &PollError{
			Code:        tokenResp.Status,
			Description: "Waiting for user authorization",
			ShouldRetry: true,
		}
	case "slow_down":
		return nil, &PollError{
			Code:        tokenResp.Status,
			Description: "Polling too fast",
			ShouldRetry: true,
			SlowDown:    true,
		}
	case "expired":
		return nil, &PollError{
			Code:        tokenResp.Status,
			Description: "The device code has expired. Please restart the login process.",
			ShouldRetry: false,
		}
	case "access_denied":
		return nil, &PollError{
			Code:        tokenResp.Status,
			Description: "Authorization was denied by the user.",
			ShouldRetry: false,
		}
	default:
		if tokenResp.Error != "" {
			return nil, fmt.Errorf("authorization failed: %s", tokenResp.Error)
		}
		return nil, fmt.Errorf("unknown status: %s", tokenResp.Status)
	}
}

