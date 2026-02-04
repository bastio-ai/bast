// Package safety provides security-related utilities for command validation.
package safety

import (
	"regexp"
)

// dangerousPatterns defines regex patterns for potentially dangerous commands.
// These patterns are used to warn users before executing destructive operations.
var dangerousPatterns = []*regexp.Regexp{
	// File system operations
	regexp.MustCompile(`rm\s+(-[rRf]+\s+)*[/~]`),    // rm -rf / or ~
	regexp.MustCompile(`rm\s+-[rRf]+\s+\*`),         // rm -rf *
	regexp.MustCompile(`\bmkfs\b`),                  // filesystem format
	regexp.MustCompile(`\bdd\s+.*of=/dev/`),         // dd to device
	regexp.MustCompile(`>\s*/dev/sd`),               // redirect to device
	regexp.MustCompile(`chmod\s+(-R\s+)?777`),       // overly permissive
	regexp.MustCompile(`:\(\)\{\s*:\|:\s*&\s*\};:`), // fork bomb
	regexp.MustCompile(`>\s*/dev/null\s+2>&1\s*&`),  // backgrounded with no output
	regexp.MustCompile(`curl.*\|\s*(ba)?sh`),        // pipe curl to shell
	regexp.MustCompile(`wget.*\|\s*(ba)?sh`),        // pipe wget to shell

	// Git destructive operations
	regexp.MustCompile(`git\s+push\s+.*(-f|--force)`),             // force push
	regexp.MustCompile(`git\s+push\s+--force-with-lease`),         // force with lease (still destructive)
	regexp.MustCompile(`git\s+reset\s+--hard`),                    // hard reset
	regexp.MustCompile(`git\s+clean\s+-[fd]`),                     // clean untracked files/dirs
	regexp.MustCompile(`git\s+checkout\s+--\s*\.`),                // discard all changes
	regexp.MustCompile(`git\s+branch\s+-[dD]\s+\S`),               // delete branch
	regexp.MustCompile(`git\s+rebase\s`),                          // rebase (history rewriting)
	regexp.MustCompile(`git\s+commit\s+--amend`),                  // amend (history rewriting)
	regexp.MustCompile(`git\s+push\s+.*:.*`),                      // delete remote ref (push :branch)
	regexp.MustCompile(`git\s+stash\s+(drop|clear)`),              // drop stash
	regexp.MustCompile(`git\s+reflog\s+expire`),                   // expire reflog
	regexp.MustCompile(`git\s+gc\s+--prune`),                      // prune garbage collection
	regexp.MustCompile(`git\s+filter-branch`),                     // filter-branch (history rewriting)
	regexp.MustCompile(`git\s+push\s+(origin|upstream)\s+main`),   // push to main
	regexp.MustCompile(`git\s+push\s+(origin|upstream)\s+master`), // push to master
}

// IsDangerousCommand checks if a command matches any dangerous patterns.
// Returns true if the command could be destructive and should require
// additional user confirmation before execution.
func IsDangerousCommand(command string) bool {
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return true
		}
	}
	return false
}

// GetDangerousPatterns returns a copy of the dangerous patterns for testing.
func GetDangerousPatterns() []*regexp.Regexp {
	patterns := make([]*regexp.Regexp, len(dangerousPatterns))
	copy(patterns, dangerousPatterns)
	return patterns
}
