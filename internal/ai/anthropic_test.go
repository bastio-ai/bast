package ai

import "testing"

func TestCleanCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Plain commands - no cleaning needed
		{"plain command", "ls -la", "ls -la"},
		{"command with args", "git status", "git status"},
		{"complex command", "find . -name '*.go' -type f", "find . -name '*.go' -type f"},

		// Markdown code blocks
		{"bash code block", "```bash\nls -la\n```", "ls -la"},
		{"sh code block", "```sh\necho hello\n```", "echo hello"},
		{"generic code block", "```\ncat file.txt\n```", "cat file.txt"},
		{"code block with extra whitespace", "```bash\n  ls -la  \n```", "ls -la"},

		// Single backticks
		{"single backticks", "`ls -la`", "ls -la"},
		{"single backticks complex", "`git commit -m \"message\"`", "git commit -m \"message\""},

		// Multiline commands in code blocks
		{"multiline command", "```bash\nls -la && \\\npwd\n```", "ls -la && \\\npwd"},

		// Whitespace handling
		{"leading whitespace", "  ls -la", "ls -la"},
		{"trailing whitespace", "ls -la  ", "ls -la"},
		{"both whitespace", "  ls -la  ", "ls -la"},

		// Edge cases
		{"empty string", "", ""},
		{"only backticks", "```", ""},
		{"only whitespace", "   ", ""},

		// No code block markers (language tag only)
		{"language only no block", "bash\nls -la", "bash\nls -la"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanCommand(tt.input)
			if got != tt.expected {
				t.Errorf("cleanCommand(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
