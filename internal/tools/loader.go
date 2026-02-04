package tools

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/*.yaml
var defaultPlugins embed.FS

// PluginManifest defines the YAML structure for a user-defined tool
type PluginManifest struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Command     string              `yaml:"command"`      // Shell command to execute
	Script      string              `yaml:"script"`       // Or path to script file
	Parameters  []PluginParameter   `yaml:"parameters"`
	Timeout     int                 `yaml:"timeout"`      // Timeout in seconds (default 30)
}

// PluginParameter defines a parameter for a user-defined tool
type PluginParameter struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`        // string, number, boolean
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Enum        []string `yaml:"enum,omitempty"`
}

// PluginTool wraps a user-defined tool from a YAML manifest
type PluginTool struct {
	manifest PluginManifest
	basePath string // Directory containing the manifest
}

func (t *PluginTool) Name() string {
	return t.manifest.Name
}

func (t *PluginTool) Description() string {
	return t.manifest.Description
}

func (t *PluginTool) InputSchema() InputSchema {
	props := make(map[string]Property)
	var required []string

	for _, param := range t.manifest.Parameters {
		props[param.Name] = Property{
			Type:        param.Type,
			Description: param.Description,
			Enum:        param.Enum,
		}
		if param.Required {
			required = append(required, param.Name)
		}
	}

	return InputSchema{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}

func (t *PluginTool) Execute(ctx context.Context, input json.RawMessage) (*Result, error) {
	// Parse input parameters
	var params map[string]interface{}
	if err := json.Unmarshal(input, &params); err != nil {
		return &Result{Output: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	// Determine command to run
	var command string
	if t.manifest.Command != "" {
		command = t.manifest.Command
	} else if t.manifest.Script != "" {
		// Resolve script path relative to manifest
		scriptPath := t.manifest.Script
		if !filepath.IsAbs(scriptPath) {
			scriptPath = filepath.Join(t.basePath, scriptPath)
		}
		command = scriptPath
	} else {
		return &Result{Output: "tool has no command or script defined", IsError: true}, nil
	}

	// Substitute parameters in command using $PARAM_NAME format
	for name, value := range params {
		envKey := strings.ToUpper(name)
		placeholder := "$" + envKey
		command = strings.ReplaceAll(command, placeholder, fmt.Sprintf("%v", value))
	}

	// Set timeout
	timeout := time.Duration(t.manifest.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute command
	cmd := exec.CommandContext(execCtx, "sh", "-c", command)
	cmd.Dir = t.basePath

	// Set parameters as environment variables
	cmd.Env = os.Environ()
	for name, value := range params {
		envKey := "BAST_PARAM_" + strings.ToUpper(name)
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", envKey, value))
	}

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if len(outputStr) > MaxOutputSize {
		outputStr = outputStr[:MaxOutputSize] + "\n... (output truncated)"
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return &Result{Output: "command timed out", IsError: true}, nil
		}
		return &Result{
			Output:  fmt.Sprintf("%s\nExit error: %v", outputStr, err),
			IsError: true,
		}, nil
	}

	return &Result{Output: outputStr}, nil
}

// LoadPlugins loads all user-defined tools from a directory
func LoadPlugins(dir string) ([]*PluginTool, error) {
	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, nil // No plugins directory, not an error
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	var plugins []*PluginTool

	for _, entry := range entries {
		// Look for YAML files or directories with manifest.yaml
		var manifestPath string
		var basePath string

		if entry.IsDir() {
			// Check for manifest.yaml in subdirectory
			manifestPath = filepath.Join(dir, entry.Name(), "manifest.yaml")
			basePath = filepath.Join(dir, entry.Name())
			if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
				// Try manifest.yml
				manifestPath = filepath.Join(dir, entry.Name(), "manifest.yml")
				if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
					continue
				}
			}
		} else if strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml") {
			manifestPath = filepath.Join(dir, entry.Name())
			basePath = dir
		} else {
			continue
		}

		// Load manifest
		plugin, err := loadPlugin(manifestPath, basePath)
		if err != nil {
			// Log warning but continue loading other plugins
			fmt.Fprintf(os.Stderr, "Warning: failed to load plugin %s: %v\n", manifestPath, err)
			continue
		}

		plugins = append(plugins, plugin)
	}

	return plugins, nil
}

func loadPlugin(manifestPath, basePath string) (*PluginTool, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest PluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Validate manifest
	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest missing required field: name")
	}
	if manifest.Description == "" {
		return nil, fmt.Errorf("manifest missing required field: description")
	}
	if manifest.Command == "" && manifest.Script == "" {
		return nil, fmt.Errorf("manifest must have either command or script")
	}

	return &PluginTool{
		manifest: manifest,
		basePath: basePath,
	}, nil
}

// DefaultPluginsDir returns the default plugins directory path
func DefaultPluginsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "bast", "tools"), nil
}

// LoadUserPlugins loads plugins from the default user directory
func LoadUserPlugins() ([]*PluginTool, error) {
	dir, err := DefaultPluginsDir()
	if err != nil {
		return nil, err
	}
	return LoadPlugins(dir)
}

// RegisterUserPlugins loads and registers user plugins with a registry
func RegisterUserPlugins(registry *Registry) error {
	plugins, err := LoadUserPlugins()
	if err != nil {
		return err
	}

	for _, plugin := range plugins {
		if err := registry.Register(plugin); err != nil {
			// Log warning but continue registering other plugins
			fmt.Fprintf(os.Stderr, "Warning: failed to register plugin %s: %v\n", plugin.Name(), err)
		}
	}

	return nil
}

// RegisterDefaultPlugins loads and registers the built-in default plugins from embedded YAML files
func RegisterDefaultPlugins(registry *Registry, cwd string) error {
	entries, err := defaultPlugins.ReadDir("defaults")
	if err != nil {
		return fmt.Errorf("failed to read embedded defaults: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		data, err := defaultPlugins.ReadFile("defaults/" + entry.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read default plugin %s: %v\n", entry.Name(), err)
			continue
		}

		var manifest PluginManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse default plugin %s: %v\n", entry.Name(), err)
			continue
		}

		// Validate manifest
		if manifest.Name == "" || manifest.Description == "" {
			fmt.Fprintf(os.Stderr, "Warning: default plugin %s missing required fields\n", entry.Name())
			continue
		}

		if manifest.Command == "" && manifest.Script == "" {
			fmt.Fprintf(os.Stderr, "Warning: default plugin %s has no command or script\n", entry.Name())
			continue
		}

		plugin := &PluginTool{
			manifest: manifest,
			basePath: cwd, // Use current working directory for default plugins
		}

		if err := registry.Register(plugin); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to register default plugin %s: %v\n", plugin.Name(), err)
		}
	}

	return nil
}
