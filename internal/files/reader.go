package files

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// sensitivePatterns defines file patterns that should not be read and sent to AI
var sensitivePatterns = []string{
	".env",
	".env.*",
	"*.key",
	"*.pem",
	"*.p12",
	"*.pfx",
	"*credentials*",
	"*secrets*",
	".aws/credentials",
	".ssh/*",
	"*.secret",
	"id_rsa*",
	"id_ed25519*",
	"id_ecdsa*",
	"id_dsa*",
	".netrc",
	".npmrc",
	".pypirc",
}

// FileContent holds a file's content for context
type FileContent struct {
	Path    string
	Content string
	Error   string // If file couldn't be read
}

// ReadFiles reads multiple files, respecting size limits.
// maxBytes is the maximum total bytes to read across all files.
// Files are read in order until the limit is reached.
func ReadFiles(cwd string, paths []string, maxBytes int) []FileContent {
	var results []FileContent
	totalRead := 0

	for _, p := range paths {
		if totalRead >= maxBytes {
			break
		}

		// Resolve path relative to cwd
		fullPath := p
		if !filepath.IsAbs(p) {
			fullPath = filepath.Join(cwd, p)
		}

		// Security: ensure path is within cwd (no parent traversal)
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			results = append(results, FileContent{
				Path:  p,
				Error: "invalid path",
			})
			continue
		}

		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			results = append(results, FileContent{
				Path:  p,
				Error: "invalid working directory",
			})
			continue
		}

		if !strings.HasPrefix(absPath, absCwd+string(filepath.Separator)) && absPath != absCwd {
			// Allow files directly in cwd
			if filepath.Dir(absPath) != absCwd {
				results = append(results, FileContent{
					Path:  p,
					Error: "path outside working directory",
				})
				continue
			}
		}

		// Security: block sensitive files from being read
		if isSensitiveFile(absPath) {
			results = append(results, FileContent{
				Path:  p,
				Error: "sensitive file (contains credentials or secrets)",
			})
			continue
		}

		// Check if file exists and is regular
		info, err := os.Stat(absPath)
		if err != nil {
			results = append(results, FileContent{
				Path:  p,
				Error: "file not found",
			})
			continue
		}

		if info.IsDir() {
			results = append(results, FileContent{
				Path:  p,
				Error: "is a directory",
			})
			continue
		}

		// Skip large files
		if info.Size() > int64(MaxSingleFileBytes) {
			results = append(results, FileContent{
				Path:  p,
				Error: "file too large (>50KB, see MaxSingleFileBytes)",
			})
			continue
		}

		// Read file with remaining budget
		remaining := maxBytes - totalRead
		if remaining <= 0 {
			break
		}

		content, err := readFileWithLimit(absPath, remaining)
		if err != nil {
			results = append(results, FileContent{
				Path:  p,
				Error: err.Error(),
			})
			continue
		}

		// Skip binary files (check for null bytes or invalid UTF-8)
		if isBinary(content) {
			results = append(results, FileContent{
				Path:  p,
				Error: "binary file",
			})
			continue
		}

		totalRead += len(content)
		truncated := len(content) < int(info.Size())

		fc := FileContent{
			Path:    p,
			Content: content,
		}
		if truncated {
			fc.Content += "\n... (truncated)"
		}

		results = append(results, fc)
	}

	return results
}

// readFileWithLimit reads up to maxBytes from a file
func readFileWithLimit(path string, maxBytes int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	data := make([]byte, maxBytes)
	n, err := io.ReadFull(f, data)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}

	return string(data[:n]), nil
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// isBinary checks if content appears to be binary
func isBinary(content string) bool {
	// Check for null bytes
	if strings.Contains(content, "\x00") {
		return true
	}
	// Check if content is valid UTF-8
	if !utf8.ValidString(content) {
		return true
	}
	return false
}

// isSensitiveFile checks if a filename matches sensitive patterns
func isSensitiveFile(filename string) bool {
	name := filepath.Base(filename)
	nameLower := strings.ToLower(name)

	for _, pattern := range sensitivePatterns {
		patternLower := strings.ToLower(pattern)

		// Handle wildcard patterns
		if strings.HasPrefix(patternLower, "*") && strings.HasSuffix(patternLower, "*") {
			// *contains*
			substr := patternLower[1 : len(patternLower)-1]
			if strings.Contains(nameLower, substr) {
				return true
			}
		} else if strings.HasPrefix(patternLower, "*") {
			// *suffix
			suffix := patternLower[1:]
			if strings.HasSuffix(nameLower, suffix) {
				return true
			}
		} else if strings.HasSuffix(patternLower, "*") {
			// prefix*
			prefix := patternLower[:len(patternLower)-1]
			if strings.HasPrefix(nameLower, prefix) {
				return true
			}
		} else if strings.Contains(patternLower, "/") {
			// Path pattern like .ssh/* or .aws/credentials
			if strings.HasSuffix(patternLower, "/*") {
				// Directory wildcard
				dir := patternLower[:len(patternLower)-2]
				if strings.Contains(strings.ToLower(filename), dir+"/") {
					return true
				}
			} else {
				// Exact path match
				if strings.HasSuffix(strings.ToLower(filename), patternLower) {
					return true
				}
			}
		} else if strings.Contains(patternLower, ".") && strings.HasPrefix(patternLower, ".env") {
			// .env.* pattern
			if nameLower == ".env" || strings.HasPrefix(nameLower, ".env.") {
				return true
			}
		} else {
			// Exact match
			if nameLower == patternLower {
				return true
			}
		}
	}
	return false
}

// FindFile finds a file by partial name (case-insensitive).
// It searches for common variations of the given name.
func FindFile(cwd string, name string) (string, error) {
	name = strings.ToLower(name)

	// Build list of candidates to try
	var candidates []string

	// Exact match first
	candidates = append(candidates, name)

	// Common file patterns based on the name
	switch name {
	case "readme":
		candidates = append(candidates,
			"README.md", "README", "readme.md", "readme",
			"README.txt", "readme.txt", "README.rst", "readme.rst",
		)
	case "license":
		candidates = append(candidates,
			"LICENSE", "LICENSE.md", "LICENSE.txt",
			"license", "license.md", "license.txt",
			"LICENCE", "LICENCE.md", "licence", "licence.md",
		)
	case "changelog":
		candidates = append(candidates,
			"CHANGELOG.md", "CHANGELOG", "changelog.md", "changelog",
			"HISTORY.md", "HISTORY", "history.md",
		)
	case "contributing":
		candidates = append(candidates,
			"CONTRIBUTING.md", "CONTRIBUTING", "contributing.md",
		)
	case "makefile":
		candidates = append(candidates,
			"Makefile", "makefile", "GNUmakefile",
		)
	case "dockerfile":
		candidates = append(candidates,
			"Dockerfile", "dockerfile",
		)
	case "config":
		candidates = append(candidates,
			"config.json", "config.yaml", "config.yml", "config.toml",
			".config", "config.js", "config.ts",
		)
	default:
		// Try with common extensions
		candidates = append(candidates,
			name+".md", name+".txt", name+".go", name+".js", name+".ts",
			name+".py", name+".json", name+".yaml", name+".yml",
			strings.ToUpper(name), capitalizeFirst(name),
		)
	}

	// Try each candidate
	for _, candidate := range candidates {
		path := filepath.Join(cwd, candidate)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	// If nothing found, try case-insensitive directory scan
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryLower := strings.ToLower(entry.Name())
		// Check if name is contained in the filename
		if strings.Contains(entryLower, name) {
			return entry.Name(), nil
		}
		// Check without extension
		ext := filepath.Ext(entryLower)
		nameWithoutExt := strings.TrimSuffix(entryLower, ext)
		if nameWithoutExt == name {
			return entry.Name(), nil
		}
	}

	return "", os.ErrNotExist
}
