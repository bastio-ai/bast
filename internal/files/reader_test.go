package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSensitiveFile(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		sensitive bool
	}{
		// Environment files
		{".env", ".env", true},
		{".env.local", ".env.local", true},
		{".env.production", ".env.production", true},
		{".env.development", ".env.development", true},

		// Key files
		{"private key", "server.key", true},
		{"ssl key", "ssl.key", true},
		{"pem file", "certificate.pem", true},
		{"p12 file", "keystore.p12", true},
		{"pfx file", "certificate.pfx", true},

		// Credentials files
		{"credentials json", "credentials.json", true},
		{"aws credentials", "aws_credentials.yaml", true},
		{"db credentials", "db-credentials.txt", true},

		// Secrets files
		{"secrets yaml", "secrets.yaml", true},
		{"app secrets", "app-secrets.json", true},
		{"secret file", "config.secret", true},

		// SSH files
		{"ssh dir file", "/home/user/.ssh/id_rsa", true},
		{"id_rsa", "id_rsa", true},
		{"id_rsa.pub", "id_rsa.pub", true},
		{"id_ed25519", "id_ed25519", true},
		{"id_ecdsa", "id_ecdsa", true},
		{"id_dsa", "id_dsa", true},

		// Auth files
		{".netrc", ".netrc", true},
		{".npmrc", ".npmrc", true},
		{".pypirc", ".pypirc", true},

		// AWS credentials path
		{"aws credentials path", "/home/user/.aws/credentials", true},

		// Safe files
		{"readme", "README.md", false},
		{"go file", "main.go", false},
		{"json config", "config.json", false},
		{"yaml config", "config.yaml", false},
		{"package.json", "package.json", false},
		{"go.mod", "go.mod", false},
		{"dockerfile", "Dockerfile", false},
		{"makefile", "Makefile", false},
		{"gitignore", ".gitignore", false},

		// Files that contain "key" but are not key files
		{"keyboard.go", "keyboard.go", false},
		{"keymap.json", "keymap.json", false},

		// Files with "secrets" (plural) in name are blocked, but "secret" (singular) is not
		{"secrets in name", "app-secrets.json", true},
		{"secret singular not blocked", "my_secret_config.txt", false},

		// Files that contain "credentials" in name should be blocked
		{"credentials in name", "user_credentials.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitiveFile(tt.filename)
			if got != tt.sensitive {
				t.Errorf("isSensitiveFile(%q) = %v, want %v", tt.filename, got, tt.sensitive)
			}
		})
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name    string
		content string
		binary  bool
	}{
		{"empty", "", false},
		{"plain text", "Hello, World!", false},
		{"multiline text", "line1\nline2\nline3", false},
		{"unicode text", "Hello, 世界! 🎉", false},
		{"null byte", "Hello\x00World", true},
		{"binary data", "\x00\x01\x02\x03", true},
		{"code", "func main() {\n\tfmt.Println(\"hello\")\n}", false},
		{"json", `{"key": "value", "number": 42}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBinary(tt.content)
			if got != tt.binary {
				t.Errorf("isBinary(%q) = %v, want %v", tt.content, got, tt.binary)
			}
		})
	}
}

func TestReadFiles(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "bast-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"readme.md":    "# Test README\n\nThis is a test file.",
		"main.go":      "package main\n\nfunc main() {}\n",
		"config.json":  `{"key": "value"}`,
		".env":         "SECRET_KEY=abc123",
		"data.bin":     "text\x00binary",
		"subdir/a.txt": "file in subdirectory",
	}

	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
	}

	t.Run("read single file", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{"readme.md"}, MaxTotalFileBytes)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Error != "" {
			t.Errorf("Unexpected error: %s", results[0].Error)
		}
		if results[0].Content != testFiles["readme.md"] {
			t.Errorf("Content mismatch: got %q, want %q", results[0].Content, testFiles["readme.md"])
		}
	})

	t.Run("read multiple files", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{"readme.md", "main.go", "config.json"}, MaxTotalFileBytes)
		if len(results) != 3 {
			t.Fatalf("Expected 3 results, got %d", len(results))
		}
		for _, r := range results {
			if r.Error != "" {
				t.Errorf("Unexpected error for %s: %s", r.Path, r.Error)
			}
		}
	})

	t.Run("block sensitive file", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{".env"}, MaxTotalFileBytes)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Error == "" {
			t.Error("Expected error for sensitive file, got none")
		}
		if results[0].Content != "" {
			t.Error("Sensitive file content should not be returned")
		}
	})

	t.Run("block binary file", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{"data.bin"}, MaxTotalFileBytes)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Error == "" {
			t.Error("Expected error for binary file, got none")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{"nonexistent.txt"}, MaxTotalFileBytes)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Error == "" {
			t.Error("Expected error for nonexistent file, got none")
		}
	})

	t.Run("subdirectory file", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{"subdir/a.txt"}, MaxTotalFileBytes)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Error != "" {
			t.Errorf("Unexpected error: %s", results[0].Error)
		}
	})

	t.Run("path traversal blocked", func(t *testing.T) {
		results := ReadFiles(tmpDir, []string{"../../../etc/passwd"}, MaxTotalFileBytes)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Error == "" {
			t.Error("Expected error for path traversal, got none")
		}
	})

	t.Run("respect byte limit", func(t *testing.T) {
		// Create a file larger than the byte limit
		largePath := filepath.Join(tmpDir, "large.txt")
		largeContent := make([]byte, 500)
		for i := range largeContent {
			largeContent[i] = 'a'
		}
		if err := os.WriteFile(largePath, largeContent, 0644); err != nil {
			t.Fatalf("Failed to write large file: %v", err)
		}

		// Use a limit that will cause truncation
		// The function reads up to maxBytes and may append "... (truncated)"
		results := ReadFiles(tmpDir, []string{"large.txt"}, 200)
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		// Content should be truncated (less than original 500 bytes)
		if len(results[0].Content) >= 500 {
			t.Errorf("Content should be truncated, got %d bytes", len(results[0].Content))
		}
	})
}

func TestFindFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "bast-findfile-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []string{
		"README.md",
		"LICENSE",
		"Makefile",
		"main.go",
		"config.yaml",
	}

	for _, name := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", name, err)
		}
	}

	tests := []struct {
		name     string
		query    string
		wantErr  bool
	}{
		{"readme lowercase", "readme", false},
		{"license lowercase", "license", false},
		{"makefile lowercase", "makefile", false},
		{"exact match", "main.go", false},
		{"partial config", "config", false},
		{"nonexistent", "foobar", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindFile(tmpDir, tt.query)
			if tt.wantErr {
				if err == nil {
					t.Errorf("FindFile(%q) expected error, got %q", tt.query, got)
				}
				return
			}
			if err != nil {
				t.Errorf("FindFile(%q) unexpected error: %v", tt.query, err)
				return
			}
			// Just verify we got a result (case may vary based on filesystem)
			if got == "" {
				t.Errorf("FindFile(%q) returned empty string", tt.query)
			}
		})
	}
}
