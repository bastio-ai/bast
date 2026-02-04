package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommandTool(t *testing.T) {
	tool := &RunCommandTool{}

	t.Run("executes simple command", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"command": "echo hello"})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Output)
		}
		if !strings.Contains(result.Output, "hello") {
			t.Errorf("expected output to contain 'hello', got: %s", result.Output)
		}
	})

	t.Run("returns error for empty command", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"command": ""})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for empty command")
		}
	})

	t.Run("returns error for failed command", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"command": "exit 1"})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for failed command")
		}
	})
}

func TestReadFileTool(t *testing.T) {
	tool := &ReadFileTool{}

	// Create a temp file for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0644)

	t.Run("reads existing file", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"path": testFile})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Output)
		}
		if result.Output != "test content" {
			t.Errorf("expected 'test content', got: %s", result.Output)
		}
	})

	t.Run("returns error for nonexistent file", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"path": "/nonexistent/file.txt"})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("returns error for empty path", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"path": ""})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for empty path")
		}
	})
}

func TestListDirectoryTool(t *testing.T) {
	tool := &ListDirectoryTool{}

	// Create a temp directory with files
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)

	t.Run("lists directory contents", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"path": tmpDir})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Output)
		}
		if !strings.Contains(result.Output, "file1.txt") {
			t.Errorf("expected output to contain 'file1.txt', got: %s", result.Output)
		}
		if !strings.Contains(result.Output, "subdir/") {
			t.Errorf("expected output to contain 'subdir/', got: %s", result.Output)
		}
	})

	t.Run("returns error for nonexistent directory", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"path": "/nonexistent/dir"})
		result, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.IsError {
			t.Error("expected error for nonexistent directory")
		}
	})
}

func TestRegistry(t *testing.T) {
	t.Run("registers and retrieves tools", func(t *testing.T) {
		registry := NewRegistry()
		tool := &RunCommandTool{}

		err := registry.Register(tool)
		if err != nil {
			t.Fatalf("unexpected error registering tool: %v", err)
		}

		retrieved, ok := registry.Get("run_command")
		if !ok {
			t.Fatal("expected to find registered tool")
		}
		if retrieved.Name() != "run_command" {
			t.Errorf("expected name 'run_command', got: %s", retrieved.Name())
		}
	})

	t.Run("returns error for duplicate registration", func(t *testing.T) {
		registry := NewRegistry()
		tool := &RunCommandTool{}

		registry.Register(tool)
		err := registry.Register(tool)
		if err == nil {
			t.Error("expected error for duplicate registration")
		}
	})

	t.Run("lists all tools", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(&RunCommandTool{})
		registry.Register(&ReadFileTool{})

		tools := registry.List()
		if len(tools) != 2 {
			t.Errorf("expected 2 tools, got: %d", len(tools))
		}
	})

	t.Run("executes tool by name", func(t *testing.T) {
		registry := NewRegistry()
		registry.Register(&RunCommandTool{})

		input, _ := json.Marshal(map[string]string{"command": "echo test"})
		result, err := registry.Execute(context.Background(), "run_command", input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError {
			t.Fatalf("expected success, got error: %s", result.Output)
		}
	})

	t.Run("returns error for unknown tool", func(t *testing.T) {
		registry := NewRegistry()
		result, _ := registry.Execute(context.Background(), "unknown", nil)
		if !result.IsError {
			t.Error("expected error for unknown tool")
		}
	})
}

func TestRegisterDefaultPlugins(t *testing.T) {
	t.Run("registers default plugins", func(t *testing.T) {
		registry := NewRegistry()
		cwd := t.TempDir()

		err := RegisterDefaultPlugins(registry, cwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that git_summary is registered
		gitSummary, ok := registry.Get("git_summary")
		if !ok {
			t.Error("expected git_summary to be registered")
		} else {
			if gitSummary.Description() == "" {
				t.Error("expected git_summary to have a description")
			}
		}

		// Check that grep_code is registered
		grepCode, ok := registry.Get("grep_code")
		if !ok {
			t.Error("expected grep_code to be registered")
		} else {
			if grepCode.Description() == "" {
				t.Error("expected grep_code to have a description")
			}
			// Check that it has the pattern parameter
			schema := grepCode.InputSchema()
			if _, hasPattern := schema.Properties["pattern"]; !hasPattern {
				t.Error("expected grep_code to have a 'pattern' parameter")
			}
		}
	})

	t.Run("default plugins have correct tools count", func(t *testing.T) {
		registry := NewRegistry()
		cwd := t.TempDir()

		err := RegisterDefaultPlugins(registry, cwd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tools := registry.List()
		if len(tools) != 3 {
			t.Errorf("expected 3 default plugins, got: %d", len(tools))
		}
	})
}
