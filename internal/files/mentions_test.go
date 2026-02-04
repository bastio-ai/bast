package files

import (
	"reflect"
	"testing"
)

func TestParseMentions(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{"no mentions", "list all files", nil},
		{"single mention", "summarize @readme.md", []string{"readme.md"}},
		{"multiple mentions", "compare @file1.go and @file2.go", []string{"file1.go", "file2.go"}},
		{"path mention", "explain @src/main.go", []string{"src/main.go"}},
		{"relative path", "read @./config.yaml", []string{"./config.yaml"}},
		{"quoted with spaces", `read @"file with spaces.txt"`, []string{"file with spaces.txt"}},
		{"mixed quoted and unquoted", `compare @file.go and @"my doc.md"`, []string{"file.go", "my doc.md"}},
		{"mention at start", "@readme.md summarize this", []string{"readme.md"}},
		{"mention at end", "what's in @package.json", []string{"package.json"}},
		{"deep path", "@internal/files/reader.go", []string{"internal/files/reader.go"}},
		// Note: emails are currently matched as mentions - this is a known limitation
		{"email matched as mention", "contact user@example.com", []string{"example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseMentions(tt.query)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseMentions(%q) = %v, want %v", tt.query, got, tt.expected)
			}
		})
	}
}

func TestDetectFileReferences(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{"no references", "how does go work", nil},
		{"readme reference", "summarize the readme", []string{"readme"}},
		{"license reference", "show me the license", []string{"license"}},
		{"makefile reference", "explain the makefile", []string{"makefile"}},
		{"dockerfile reference", "what's in the dockerfile", []string{"dockerfile"}},
		{"go.mod reference", "check go.mod dependencies", []string{"go.mod"}},
		{"package.json reference", "update package.json", []string{"package.json"}},
		{"multiple references", "compare readme and license", []string{"readme", "license"}},
		{"with indicator", "what's in the config", []string{"config"}},
		{"file extension", "read main.go", []string{"main.go"}},
		{"path reference", "look at src/utils.ts", []string{"src/utils.ts"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFileReferences(tt.query)
			// Check that all expected references are found (order may vary)
			if len(got) != len(tt.expected) {
				t.Errorf("DetectFileReferences(%q) = %v (len %d), want %v (len %d)",
					tt.query, got, len(got), tt.expected, len(tt.expected))
				return
			}
			gotMap := make(map[string]bool)
			for _, g := range got {
				gotMap[g] = true
			}
			for _, e := range tt.expected {
				if !gotMap[e] {
					t.Errorf("DetectFileReferences(%q) missing expected %q, got %v", tt.query, e, got)
				}
			}
		})
	}
}

func TestIsLikelyFileReference(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		// Known file patterns
		{"readme", "readme", true},
		{"license", "license", true},
		{"makefile", "makefile", true},
		{"dockerfile", "dockerfile", true},
		{"package.json", "package.json", true},
		{"go.mod", "go.mod", true},

		// File extensions
		{"go file", "main.go", true},
		{"js file", "app.js", true},
		{"ts file", "index.ts", true},
		{"py file", "script.py", true},
		{"md file", "README.md", true},
		{"yaml file", "config.yaml", true},
		{"json file", "data.json", true},

		// Paths
		{"path with slash", "src/main.go", true},
		{"nested path", "internal/files/reader.go", true},

		// Not file references
		{"plain word", "hello", false},
		{"number", "123", false},
		{"the", "the", false},
		{"unknown ext", "file.xyz", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLikelyFileReference(tt.word)
			if got != tt.expected {
				t.Errorf("isLikelyFileReference(%q) = %v, want %v", tt.word, got, tt.expected)
			}
		})
	}
}

func TestStripMentions(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{"no mentions", "list all files", "list all files"},
		{"single mention", "summarize @readme.md", "summarize readme.md"},
		{"multiple mentions", "compare @a.go and @b.go", "compare a.go and b.go"},
		{"quoted mention", `read @"my file.txt"`, `read "my file.txt"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripMentions(tt.query)
			if got != tt.expected {
				t.Errorf("StripMentions(%q) = %q, want %q", tt.query, got, tt.expected)
			}
		})
	}
}
