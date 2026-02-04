package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBastioSecurityClient_ValidateToolCall(t *testing.T) {
	t.Run("allow action permits execution", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/v1/guard/test-proxy/agent/validate" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
			}

			resp := ValidationResult{
				Action:    ActionAllow,
				RiskScore: 0.1,
				Message:   "Tool call permitted",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "ls -la"}`),
		}

		result, err := client.ValidateToolCall(context.Background(), call)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ActionAllow {
			t.Errorf("expected allow, got %s", result.Action)
		}
	})

	t.Run("block action prevents execution", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ValidationResult{
				Action:          ActionBlock,
				RiskScore:       0.95,
				ThreatsDetected: []string{"shell_injection", "destructive_command"},
				Message:         "Potentially dangerous command detected: rm -rf /",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "rm -rf /"}`),
		}

		result, err := client.ValidateToolCall(context.Background(), call)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ActionBlock {
			t.Errorf("expected block, got %s", result.Action)
		}
		if len(result.ThreatsDetected) != 2 {
			t.Errorf("expected 2 threats, got %d", len(result.ThreatsDetected))
		}
	})

	t.Run("require_approval action returns approval needed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ValidationResult{
				Action:     ActionRequireApproval,
				RiskScore:  0.7,
				Message:    "This action requires human approval",
				ApprovalID: "approval-456",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		call := Call{
			ID:    "call-1",
			Name:  "write_file",
			Input: json.RawMessage(`{"path": "/etc/passwd", "content": "test"}`),
		}

		result, err := client.ValidateToolCall(context.Background(), call)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ActionRequireApproval {
			t.Errorf("expected require_approval, got %s", result.Action)
		}
		if result.ApprovalID != "approval-456" {
			t.Errorf("expected approval ID approval-456, got %s", result.ApprovalID)
		}
	})

	t.Run("warn action logs but continues", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ValidationResult{
				Action:          ActionWarn,
				RiskScore:       0.5,
				ThreatsDetected: []string{"elevated_privilege"},
				Message:         "Command requires elevated privileges",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "sudo apt update"}`),
		}

		result, err := client.ValidateToolCall(context.Background(), call)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ActionWarn {
			t.Errorf("expected warn, got %s", result.Action)
		}
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal server error"))
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "ls"}`),
		}

		_, err := client.ValidateToolCall(context.Background(), call)
		if err == nil {
			t.Error("expected error for 500 response")
		}
	})

	t.Run("sends correct request body", func(t *testing.T) {
		var receivedBody toolCallRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			resp := ValidationResult{Action: ActionAllow}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-abc")
		call := Call{
			ID:    "call-xyz",
			Name:  "read_file",
			Input: json.RawMessage(`{"path": "/tmp/test.txt"}`),
		}

		client.ValidateToolCall(context.Background(), call)

		if receivedBody.SessionID != "session-abc" {
			t.Errorf("expected session_id 'session-abc', got %s", receivedBody.SessionID)
		}
		if len(receivedBody.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(receivedBody.ToolCalls))
		}
		if receivedBody.ToolCalls[0].ID != "call-xyz" {
			t.Errorf("expected tool call id 'call-xyz', got %s", receivedBody.ToolCalls[0].ID)
		}
		if receivedBody.ToolCalls[0].Name != "read_file" {
			t.Errorf("expected tool name 'read_file', got %s", receivedBody.ToolCalls[0].Name)
		}
	})
}

func TestBastioSecurityClient_ScanContent(t *testing.T) {
	t.Run("allow action passes content through", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/v1/guard/test-proxy/agent/scan-output" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			resp := ScanResult{
				Action:    ScanActionAllow,
				RiskScore: 0.1,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		result, err := client.ScanContent(context.Background(), "run_command", "file1.txt\nfile2.txt")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ScanActionAllow {
			t.Errorf("expected allow, got %s", result.Action)
		}
	})

	t.Run("sanitize action returns cleaned content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ScanResult{
				Action:           ScanActionSanitize,
				ProcessedContent: "API_KEY=***REDACTED***",
				ThreatsDetected:  []string{"credential_exposure"},
				RiskScore:        0.8,
				Message:          "Sensitive data detected and redacted",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		result, err := client.ScanContent(context.Background(), "read_file", "API_KEY=sk-12345")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ScanActionSanitize {
			t.Errorf("expected sanitize, got %s", result.Action)
		}
		if result.ProcessedContent != "API_KEY=***REDACTED***" {
			t.Errorf("unexpected processed content: %s", result.ProcessedContent)
		}
	})

	t.Run("block action prevents output", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ScanResult{
				Action:          ScanActionBlock,
				ThreatsDetected: []string{"malware_signature"},
				RiskScore:       0.99,
				Message:         "Malicious content detected in output",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-123")
		result, err := client.ScanContent(context.Background(), "run_command", "malicious output")

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Action != ScanActionBlock {
			t.Errorf("expected block, got %s", result.Action)
		}
	})

	t.Run("sends correct request body", func(t *testing.T) {
		var receivedBody contentScanRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewDecoder(r.Body).Decode(&receivedBody)
			resp := ScanResult{Action: ScanActionAllow}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewBastioSecurityClient(server.URL, "test-proxy", "test-key", "session-xyz")
		client.ScanContent(context.Background(), "list_directory", "dir contents here")

		if receivedBody.SessionID != "session-xyz" {
			t.Errorf("expected session_id 'session-xyz', got %s", receivedBody.SessionID)
		}
		if receivedBody.ToolName != "list_directory" {
			t.Errorf("expected tool_name 'list_directory', got %s", receivedBody.ToolName)
		}
		if receivedBody.Output != "dir contents here" {
			t.Errorf("unexpected output: %s", receivedBody.Output)
		}
	})
}

func TestRegistryWithSecurity(t *testing.T) {
	t.Run("blocks tool execution when validation returns block", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ValidationResult{
				Action:  ActionBlock,
				Message: "Blocked by policy",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		securityClient := NewBastioSecurityClient(server.URL, "proxy", "key", "session")
		registry.SetSecurityClient(securityClient)

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "rm -rf /"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		if !result.IsError {
			t.Error("expected error for blocked call")
		}
		if result.Content != "Blocked by security policy: Blocked by policy" {
			t.Errorf("unexpected error message: %s", result.Content)
		}
	})

	t.Run("allows tool execution when validation returns allow", func(t *testing.T) {
		validationServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/guard/proxy/agent/validate" {
				resp := ValidationResult{Action: ActionAllow}
				json.NewEncoder(w).Encode(resp)
			} else if r.URL.Path == "/v1/guard/proxy/agent/scan-output" {
				resp := ScanResult{Action: ScanActionAllow}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer validationServer.Close()

		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		securityClient := NewBastioSecurityClient(validationServer.URL, "proxy", "key", "session")
		registry.SetSecurityClient(securityClient)

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "echo hello"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		if result.IsError {
			t.Errorf("unexpected error: %s", result.Content)
		}
		if result.Content != "hello\n" {
			t.Errorf("unexpected output: %s", result.Content)
		}
	})

	t.Run("blocks tool when require_approval is returned", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := ValidationResult{
				Action:  ActionRequireApproval,
				Message: "Needs human review",
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		securityClient := NewBastioSecurityClient(server.URL, "proxy", "key", "session")
		registry.SetSecurityClient(securityClient)

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "dangerous"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		if !result.IsError {
			t.Error("expected error for require_approval")
		}
		if result.Content != "Requires human approval: Needs human review" {
			t.Errorf("unexpected message: %s", result.Content)
		}
	})

	t.Run("sanitizes output when content scan returns sanitize", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/guard/proxy/agent/validate" {
				resp := ValidationResult{Action: ActionAllow}
				json.NewEncoder(w).Encode(resp)
			} else if r.URL.Path == "/v1/guard/proxy/agent/scan-output" {
				resp := ScanResult{
					Action:           ScanActionSanitize,
					ProcessedContent: "[REDACTED]",
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		securityClient := NewBastioSecurityClient(server.URL, "proxy", "key", "session")
		registry.SetSecurityClient(securityClient)

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "echo secret"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		if result.IsError {
			t.Errorf("unexpected error: %s", result.Content)
		}
		if result.Content != "[REDACTED]" {
			t.Errorf("expected sanitized output, got: %s", result.Content)
		}
	})

	t.Run("blocks output when content scan returns block", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/guard/proxy/agent/validate" {
				resp := ValidationResult{Action: ActionAllow}
				json.NewEncoder(w).Encode(resp)
			} else if r.URL.Path == "/v1/guard/proxy/agent/scan-output" {
				resp := ScanResult{
					Action:  ScanActionBlock,
					Message: "Malicious output detected",
				}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		securityClient := NewBastioSecurityClient(server.URL, "proxy", "key", "session")
		registry.SetSecurityClient(securityClient)

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "echo something"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		if !result.IsError {
			t.Error("expected error for blocked output")
		}
		if result.Content != "Output blocked by security policy: Malicious output detected" {
			t.Errorf("unexpected message: %s", result.Content)
		}
	})

	t.Run("executes normally without security client", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(&RunCommandTool{})
		// No security client set

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "echo test"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		if result.IsError {
			t.Errorf("unexpected error: %s", result.Content)
		}
		if result.Content != "test\n" {
			t.Errorf("unexpected output: %s", result.Content)
		}
	})

	t.Run("continues on validation error (logs warning)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/guard/proxy/agent/validate" {
				// Return invalid JSON to cause error
				w.Write([]byte("invalid json"))
			} else if r.URL.Path == "/v1/guard/proxy/agent/scan-output" {
				resp := ScanResult{Action: ScanActionAllow}
				json.NewEncoder(w).Encode(resp)
			}
		}))
		defer server.Close()

		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		securityClient := NewBastioSecurityClient(server.URL, "proxy", "key", "session")
		registry.SetSecurityClient(securityClient)

		call := Call{
			ID:    "call-1",
			Name:  "run_command",
			Input: json.RawMessage(`{"command": "echo fallback"}`),
		}

		result := registry.ExecuteCall(context.Background(), call)

		// Should continue execution despite validation error
		if result.IsError {
			t.Errorf("should continue on validation error, got: %s", result.Content)
		}
	})
}
