package files

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// skippedDirs are directories that should be skipped during file listing
var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".svn":         true,
	".hg":          true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".cache":       true,
	".idea":        true,
	".vscode":      true,
	"dist":         true,
	"build":        true,
	"target":       true,
}

// ListFiles returns files matching a prefix, for autocomplete suggestions.
// Searches cwd and subdirectories recursively (limited depth).
// Returns relative paths sorted alphabetically.
func ListFiles(cwd string, prefix string, maxResults int) []string {
	maxDepth := MaxSearchDepth
	var matches []string

	prefix = strings.ToLower(prefix)

	filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Get relative path
		relPath, err := filepath.Rel(cwd, path)
		if err != nil {
			return nil
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Check depth
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Skip hidden files/directories (starting with .)
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Skip known directories to ignore
		if d.IsDir() {
			if skippedDirs[name] {
				return fs.SkipDir
			}
			return nil // Don't include directories in results
		}

		// Check if file matches prefix (case-insensitive)
		lowerPath := strings.ToLower(relPath)
		if prefix == "" || strings.Contains(lowerPath, prefix) {
			matches = append(matches, relPath)
		}

		return nil
	})

	// Sort alphabetically
	sort.Strings(matches)

	// Limit results
	if len(matches) > maxResults {
		matches = matches[:maxResults]
	}

	return matches
}
