package shell

import "testing"

func TestParseHistoryLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		shell    string
		expected string
	}{
		// Zsh format
		{"zsh simple", "ls -la", "zsh", "ls -la"},
		{"zsh extended format", ": 1699123456:0;git status", "zsh", "git status"},
		{"zsh extended with duration", ": 1699123456:5;npm install", "zsh", "npm install"},
		{"zsh empty", "", "zsh", ""},
		{"zsh whitespace only", "   ", "zsh", ""},

		// Bash format
		{"bash simple", "ls -la", "bash", "ls -la"},
		{"bash timestamp line", "#1699123456", "bash", ""},
		{"bash empty", "", "bash", ""},
		{"bash whitespace only", "   ", "bash", ""},

		// Other shells (default behavior)
		{"unknown shell", "echo hello", "fish", "echo hello"},
		{"empty shell", "pwd", "", "pwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHistoryLine(tt.line, tt.shell)
			if got != tt.expected {
				t.Errorf("parseHistoryLine(%q, %q) = %q, want %q", tt.line, tt.shell, got, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
	}{
		{"empty string", "", 10},
		{"short string", "hello", 10},
		{"exact length", "hello", 5},
		{"truncated", "hello world", 5},
		{"long text", "this is a very long string that needs truncation", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			// Verify result doesn't exceed maxLen + "..." suffix length when truncated
			if len(tt.input) <= tt.maxLen {
				if got != tt.input {
					t.Errorf("truncate(%q, %d) = %q, want %q (no truncation expected)", tt.input, tt.maxLen, got, tt.input)
				}
			} else {
				// When truncated, the result should be maxLen bytes + "..."
				expectedLen := tt.maxLen + 3 // "..." suffix
				if len(got) != expectedLen {
					t.Errorf("truncate(%q, %d) length = %d, want %d", tt.input, tt.maxLen, len(got), expectedLen)
				}
				if got[len(got)-3:] != "..." {
					t.Errorf("truncate(%q, %d) should end with '...', got %q", tt.input, tt.maxLen, got)
				}
			}
		})
	}
}
