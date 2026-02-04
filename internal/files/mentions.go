package files

import (
	"regexp"
	"strings"
)

// mentionRegex matches @file references
// Supports: @filename, @path/to/file, @./relative, @"file with spaces"
var mentionRegex = regexp.MustCompile(`@(?:"([^"]+)"|([^\s@"]+))`)

// ParseMentions extracts @file references from a query.
// e.g., "summarize @readme.md and @src/main.go" → ["readme.md", "src/main.go"]
func ParseMentions(query string) []string {
	matches := mentionRegex.FindAllStringSubmatch(query, -1)
	var mentions []string

	for _, match := range matches {
		// match[1] is quoted, match[2] is unquoted
		if match[1] != "" {
			mentions = append(mentions, match[1])
		} else if match[2] != "" {
			mentions = append(mentions, match[2])
		}
	}

	return mentions
}

// filePatterns are words that commonly refer to specific files
var filePatterns = map[string]bool{
	"readme":       true,
	"license":      true,
	"changelog":    true,
	"contributing": true,
	"makefile":     true,
	"dockerfile":   true,
	"gitignore":    true,
	"package.json": true,
	"go.mod":       true,
	"go.sum":       true,
	"cargo.toml":   true,
	"cargo.lock":   true,
	"pyproject":    true,
	"setup.py":     true,
	"requirements": true,
	"gemfile":      true,
	"composer":     true,
	"config":       true,
	"env":          true,
	"tsconfig":     true,
}

// fileIndicators are phrases that suggest a file reference follows
var fileIndicators = []string{
	"the ",
	"in ",
	"from ",
	"read ",
	"show ",
	"what's in ",
	"whats in ",
	"contents of ",
	"content of ",
	"summarize ",
	"explain ",
	"describe ",
	"analyze ",
	"review ",
	"check ",
	"look at ",
	"open ",
}

// DetectFileReferences finds implicit file references in a query.
// e.g., "summarize the readme" → ["readme"]
// e.g., "what's in the go.mod" → ["go.mod"]
func DetectFileReferences(query string) []string {
	lower := strings.ToLower(query)
	var refs []string
	seen := make(map[string]bool)

	// Check for known file patterns directly in the query
	for pattern := range filePatterns {
		if strings.Contains(lower, pattern) && !seen[pattern] {
			refs = append(refs, pattern)
			seen[pattern] = true
		}
	}

	// Look for patterns after file indicators
	for _, indicator := range fileIndicators {
		idx := strings.Index(lower, indicator)
		if idx == -1 {
			continue
		}

		// Get the word(s) after the indicator
		after := strings.TrimSpace(lower[idx+len(indicator):])
		words := strings.Fields(after)
		if len(words) == 0 {
			continue
		}

		// Take the first word (potential file reference)
		word := words[0]
		// Clean up common suffixes
		word = strings.TrimSuffix(word, "?")
		word = strings.TrimSuffix(word, ".")
		word = strings.TrimSuffix(word, ",")
		word = strings.TrimSuffix(word, "!")

		// Check if it looks like a file reference
		if isLikelyFileReference(word) && !seen[word] {
			refs = append(refs, word)
			seen[word] = true
		}
	}

	return refs
}

// isLikelyFileReference checks if a word looks like a file reference
func isLikelyFileReference(word string) bool {
	// Known file patterns
	if filePatterns[word] {
		return true
	}

	// Has a file extension
	if strings.Contains(word, ".") {
		parts := strings.Split(word, ".")
		ext := parts[len(parts)-1]
		// Common file extensions
		commonExts := map[string]bool{
			"md": true, "txt": true, "go": true, "js": true, "ts": true,
			"py": true, "rb": true, "rs": true, "java": true, "c": true,
			"cpp": true, "h": true, "hpp": true, "json": true, "yaml": true,
			"yml": true, "toml": true, "xml": true, "html": true, "css": true,
			"sh": true, "bash": true, "zsh": true, "fish": true,
			"mod": true, "sum": true, "lock": true,
		}
		if commonExts[ext] {
			return true
		}
	}

	// Contains path separator
	if strings.Contains(word, "/") {
		return true
	}

	return false
}

// StripMentions removes @mentions from a query for cleaner AI prompts.
// e.g., "summarize @readme.md" → "summarize readme.md"
func StripMentions(query string) string {
	// Replace @mentions with just the filename
	result := mentionRegex.ReplaceAllStringFunc(query, func(match string) string {
		// Remove the @ prefix
		return strings.TrimPrefix(match, "@")
	})
	return result
}
