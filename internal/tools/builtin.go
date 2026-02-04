package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MaxOutputSize is the maximum size of tool output in bytes
const MaxOutputSize = 10000

// RunCommandTool executes shell commands
type RunCommandTool struct {
	// AllowedDir restricts command execution to this directory (optional)
	AllowedDir string
}

func (t *RunCommandTool) Name() string {
	return "run_command"
}

func (t *RunCommandTool) Description() string {
	return "Execute a shell command and return its output. Use this to run commands, check results, or gather information from the system."
}

func (t *RunCommandTool) InputSchema() InputSchema {
	return InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"command": {
				Type:        "string",
				Description: "The shell command to execute",
			},
			"working_dir": {
				Type:        "string",
				Description: "Optional working directory for the command (defaults to current directory)",
			},
		},
		Required: []string{"command"},
	}
}

type runCommandInput struct {
	Command    string `json:"command"`
	WorkingDir string `json:"working_dir,omitempty"`
}

func (t *RunCommandTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params runCommandInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &Result{Output: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if params.Command == "" {
		return &Result{Output: "command is required", IsError: true}, nil
	}

	// Set working directory
	workDir := params.WorkingDir
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return &Result{Output: fmt.Sprintf("failed to get working directory: %v", err), IsError: true}, nil
		}
	}

	// If AllowedDir is set, validate the working directory
	if t.AllowedDir != "" {
		absAllowed, _ := filepath.Abs(t.AllowedDir)
		absWork, _ := filepath.Abs(workDir)
		if !strings.HasPrefix(absWork, absAllowed) {
			return &Result{Output: "working directory outside allowed path", IsError: true}, nil
		}
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(execCtx, "sh", "-c", params.Command)
	cmd.Dir = workDir

	output, err := cmd.CombinedOutput()

	// Truncate output if too large
	outputStr := string(output)
	if len(outputStr) > MaxOutputSize {
		outputStr = outputStr[:MaxOutputSize] + "\n... (output truncated)"
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return &Result{Output: "command timed out after 30 seconds", IsError: true}, nil
		}
		// Include output even on error (often contains useful error messages)
		return &Result{
			Output:  fmt.Sprintf("%s\nExit error: %v", outputStr, err),
			IsError: true,
		}, nil
	}

	return &Result{Output: outputStr}, nil
}

// ReadFileTool reads file contents
type ReadFileTool struct {
	// AllowedDir restricts file access to this directory (optional)
	AllowedDir string
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file. Use this to examine files, configuration, or source code."
}

func (t *ReadFileTool) InputSchema() InputSchema {
	return InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"path": {
				Type:        "string",
				Description: "The path to the file to read (relative or absolute)",
			},
		},
		Required: []string{"path"},
	}
}

type readFileInput struct {
	Path string `json:"path"`
}

func (t *ReadFileTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params readFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &Result{Output: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if params.Path == "" {
		return &Result{Output: "path is required", IsError: true}, nil
	}

	// Resolve path
	path := params.Path
	if !filepath.IsAbs(path) {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, path)
	}

	// If AllowedDir is set, validate the path
	if t.AllowedDir != "" {
		absAllowed, _ := filepath.Abs(t.AllowedDir)
		absPath, _ := filepath.Abs(path)
		if !strings.HasPrefix(absPath, absAllowed) {
			return &Result{Output: "file path outside allowed directory", IsError: true}, nil
		}
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		return &Result{Output: fmt.Sprintf("cannot access file: %v", err), IsError: true}, nil
	}

	if info.IsDir() {
		return &Result{Output: "path is a directory, not a file", IsError: true}, nil
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return &Result{Output: fmt.Sprintf("failed to read file: %v", err), IsError: true}, nil
	}

	// Truncate if too large
	outputStr := string(content)
	if len(outputStr) > MaxOutputSize {
		outputStr = outputStr[:MaxOutputSize] + "\n... (file truncated)"
	}

	return &Result{Output: outputStr}, nil
}

// ListDirectoryTool lists directory contents
type ListDirectoryTool struct {
	// AllowedDir restricts directory access to this directory (optional)
	AllowedDir string
}

func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *ListDirectoryTool) Description() string {
	return "List the contents of a directory. Use this to explore the file system and find files."
}

func (t *ListDirectoryTool) InputSchema() InputSchema {
	return InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"path": {
				Type:        "string",
				Description: "The path to the directory to list (defaults to current directory)",
			},
			"show_hidden": {
				Type:        "boolean",
				Description: "Whether to show hidden files (starting with .)",
			},
		},
		Required: []string{},
	}
}

type listDirectoryInput struct {
	Path       string `json:"path,omitempty"`
	ShowHidden bool   `json:"show_hidden,omitempty"`
}

func (t *ListDirectoryTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params listDirectoryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &Result{Output: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	// Default to current directory
	path := params.Path
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return &Result{Output: fmt.Sprintf("failed to get working directory: %v", err), IsError: true}, nil
		}
	}

	// Resolve path
	if !filepath.IsAbs(path) {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, path)
	}

	// If AllowedDir is set, validate the path
	if t.AllowedDir != "" {
		absAllowed, _ := filepath.Abs(t.AllowedDir)
		absPath, _ := filepath.Abs(path)
		if !strings.HasPrefix(absPath, absAllowed) {
			return &Result{Output: "directory path outside allowed directory", IsError: true}, nil
		}
	}

	// Read directory
	entries, err := os.ReadDir(path)
	if err != nil {
		return &Result{Output: fmt.Sprintf("failed to read directory: %v", err), IsError: true}, nil
	}

	var lines []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files if not requested
		if !params.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}

		// Format entry
		info, err := entry.Info()
		if err != nil {
			lines = append(lines, fmt.Sprintf("%s (error getting info)", name))
			continue
		}

		if entry.IsDir() {
			lines = append(lines, fmt.Sprintf("%s/", name))
		} else {
			lines = append(lines, fmt.Sprintf("%s (%d bytes)", name, info.Size()))
		}
	}

	if len(lines) == 0 {
		return &Result{Output: "(empty directory)"}, nil
	}

	return &Result{Output: strings.Join(lines, "\n")}, nil
}

// WriteFileTool writes content to a file
type WriteFileTool struct {
	// AllowedDir restricts file access to this directory (optional)
	AllowedDir string
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file if it doesn't exist, or overwrites if it does. Use this to create or modify files."
}

func (t *WriteFileTool) InputSchema() InputSchema {
	return InputSchema{
		Type: "object",
		Properties: map[string]Property{
			"path": {
				Type:        "string",
				Description: "The path to the file to write (relative or absolute)",
			},
			"content": {
				Type:        "string",
				Description: "The content to write to the file",
			},
		},
		Required: []string{"path", "content"},
	}
}

type writeFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	var params writeFileInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &Result{Output: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if params.Path == "" {
		return &Result{Output: "path is required", IsError: true}, nil
	}

	// Resolve path
	path := params.Path
	if !filepath.IsAbs(path) {
		cwd, _ := os.Getwd()
		path = filepath.Join(cwd, path)
	}

	// If AllowedDir is set, validate the path
	if t.AllowedDir != "" {
		absAllowed, _ := filepath.Abs(t.AllowedDir)
		absPath, _ := filepath.Abs(path)
		if !strings.HasPrefix(absPath, absAllowed) {
			return &Result{Output: "file path outside allowed directory", IsError: true}, nil
		}
	}

	// Create parent directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &Result{Output: fmt.Sprintf("failed to create directory: %v", err), IsError: true}, nil
	}

	// Write file
	if err := os.WriteFile(path, []byte(params.Content), 0644); err != nil {
		return &Result{Output: fmt.Sprintf("failed to write file: %v", err), IsError: true}, nil
	}

	return &Result{Output: fmt.Sprintf("Successfully wrote %d bytes to %s", len(params.Content), path)}, nil
}

// DoctorTool provides friendly assistance when users ask for help
type DoctorTool struct{}

func (t *DoctorTool) Name() string {
	return "doctor"
}

func (t *DoctorTool) Description() string {
	return "A friendly helper that comes to the rescue when users need assistance. Use this tool when the user says things like \"help me out\", \"I need help\", or asks for assistance."
}

func (t *DoctorTool) InputSchema() InputSchema {
	return InputSchema{
		Type:       "object",
		Properties: map[string]Property{},
		Required:   []string{},
	}
}

func (t *DoctorTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	return &Result{Output: "🩺 Doctor to the rescue!"}, nil
}

// RegisterBuiltins registers all built-in tools with the given registry
func RegisterBuiltins(registry *Registry, allowedDir string) {
	registry.Register(&RunCommandTool{AllowedDir: allowedDir})
	registry.Register(&ReadFileTool{AllowedDir: allowedDir})
	registry.Register(&ListDirectoryTool{AllowedDir: allowedDir})
	registry.Register(&WriteFileTool{AllowedDir: allowedDir})
	registry.Register(&DoctorTool{})
}
