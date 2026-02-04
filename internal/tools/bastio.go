package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// BastioSecurityClient handles tool call validation and content scanning
// through Bastio's Agent Security API.
type BastioSecurityClient struct {
	baseURL   string
	proxyID   string
	apiKey    string
	sessionID string
	client    *http.Client
}

// NewBastioSecurityClient creates a new Bastio security client.
func NewBastioSecurityClient(baseURL, proxyID, apiKey, sessionID string) *BastioSecurityClient {
	return &BastioSecurityClient{
		baseURL:   baseURL,
		proxyID:   proxyID,
		apiKey:    apiKey,
		sessionID: sessionID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ValidationAction represents the action Bastio wants us to take
type ValidationAction string

const (
	ActionAllow           ValidationAction = "allow"
	ActionBlock           ValidationAction = "block"
	ActionRequireApproval ValidationAction = "require_approval"
	ActionWarn            ValidationAction = "warn"
)

// ValidationResult contains Bastio's response for tool call validation
type ValidationResult struct {
	Action          ValidationAction `json:"action"`
	RiskScore       float64          `json:"risk_score"`
	ThreatsDetected []string         `json:"threats_detected"`
	Message         string           `json:"message"`
	ApprovalID      string           `json:"approval_id,omitempty"`
}

// toolCallRequest is the request body for tool validation
// Note: proxy_id is passed in the URL path, not the body
type toolCallRequest struct {
	SessionID string         `json:"session_id"`
	ToolCalls []toolCallData `json:"tool_calls"`
}

type toolCallData struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ValidateToolCall sends a tool call to Bastio for validation before execution.
// Returns the validation result indicating whether the call should proceed.
func (c *BastioSecurityClient) ValidateToolCall(ctx context.Context, call Call) (*ValidationResult, error) {
	// Ensure arguments is valid JSON
	arguments := call.Input
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}

	reqBody := toolCallRequest{
		SessionID: c.sessionID,
		ToolCalls: []toolCallData{
			{
				ID:        call.ID,
				Type:      "tool_use",
				Name:      call.Name,
				Arguments: arguments,
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/guard/%s/agent/validate", c.baseURL, c.proxyID)

	// Debug output
	if os.Getenv("BAST_DEBUG_HTTP") == "1" {
		fmt.Fprintf(os.Stderr, "DEBUG SECURITY: ValidateToolCall URL=%s\n", url)
		fmt.Fprintf(os.Stderr, "DEBUG SECURITY: ValidateToolCall Body=%s\n", string(body))
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result ValidationResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// ScanAction represents the action for content scanning
type ScanAction string

const (
	ScanActionAllow    ScanAction = "allow"
	ScanActionSanitize ScanAction = "sanitize"
	ScanActionBlock    ScanAction = "block"
	ScanActionWarn     ScanAction = "warn"
)

// ScanResult contains Bastio's response for content scanning
type ScanResult struct {
	Action           ScanAction `json:"action"`
	ProcessedContent string     `json:"processed_content,omitempty"`
	ThreatsDetected  []string   `json:"threats_detected"`
	RiskScore        float64    `json:"risk_score"`
	Message          string     `json:"message"`
}

// contentScanRequest is the request body for output scanning
// Note: proxy_id is passed in the URL path, not the body
type contentScanRequest struct {
	SessionID string `json:"session_id"`
	ToolName  string `json:"tool_name"`
	Output    string `json:"output"`
}

// ScanContent sends tool output to Bastio for threat detection and sanitization.
// Returns the scan result with potential sanitized content.
func (c *BastioSecurityClient) ScanContent(ctx context.Context, toolName string, content string) (*ScanResult, error) {
	reqBody := contentScanRequest{
		SessionID: c.sessionID,
		ToolName:  toolName,
		Output:    content,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/guard/%s/agent/scan-output", c.baseURL, c.proxyID)

	// Debug output
	if os.Getenv("BAST_DEBUG_HTTP") == "1" {
		fmt.Fprintf(os.Stderr, "DEBUG SECURITY: ScanContent URL=%s\n", url)
		fmt.Fprintf(os.Stderr, "DEBUG SECURITY: ScanContent Body=%s\n", string(body))
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result ScanResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// LogWarning logs a security warning (used for warn actions)
func LogWarning(toolName string, message string, threats []string) {
	if len(threats) > 0 {
		log.Printf("Security warning for %s: %s (threats: %v)", toolName, message, threats)
	} else {
		log.Printf("Security warning for %s: %s", toolName, message)
	}
}
