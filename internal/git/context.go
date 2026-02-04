// Package git provides utilities for git repository context detection
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Context contains information about the current git repository state
type Context struct {
	IsRepo           bool     // True if current directory is in a git repo
	Branch           string   // Current branch name
	HasUncommitted   bool     // True if there are uncommitted changes
	HasUntracked     bool     // True if there are untracked files
	HasStaged        bool     // True if there are staged changes
	MergeInProgress  bool     // True if a merge is in progress
	RebaseInProgress bool     // True if a rebase is in progress
	RecentCommits    []Commit // Recent commits (up to 5)
	RemoteURL        string   // Origin remote URL (if available)
	Ahead            int      // Commits ahead of remote
	Behind           int      // Commits behind remote
}

// Commit represents a git commit
type Commit struct {
	Hash    string // Short hash (7 chars)
	Subject string // Commit message first line
	Author  string // Author name
}

// GetContext gathers git repository context from the current directory
func GetContext(cwd string) *Context {
	ctx := &Context{}

	// Check if we're in a git repository
	gitDir := findGitDir(cwd)
	if gitDir == "" {
		return ctx
	}
	ctx.IsRepo = true

	// Get current branch
	ctx.Branch = getCurrentBranch(cwd)

	// Check for uncommitted changes
	ctx.HasUncommitted, ctx.HasStaged, ctx.HasUntracked = getWorkingTreeStatus(cwd)

	// Check for merge/rebase in progress
	ctx.MergeInProgress = fileExists(filepath.Join(gitDir, "MERGE_HEAD"))
	ctx.RebaseInProgress = fileExists(filepath.Join(gitDir, "rebase-merge")) ||
		fileExists(filepath.Join(gitDir, "rebase-apply"))

	// Get recent commits
	ctx.RecentCommits = getRecentCommits(cwd, 5)

	// Get remote URL
	ctx.RemoteURL = getRemoteURL(cwd)

	// Get ahead/behind counts
	ctx.Ahead, ctx.Behind = getAheadBehind(cwd)

	return ctx
}

// findGitDir locates the .git directory for the repository
func findGitDir(cwd string) string {
	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitPath); err == nil {
			if info.IsDir() {
				return gitPath
			}
			// Handle worktree case where .git is a file
			content, err := os.ReadFile(gitPath)
			if err == nil {
				line := strings.TrimSpace(string(content))
				if strings.HasPrefix(line, "gitdir: ") {
					return strings.TrimPrefix(line, "gitdir: ")
				}
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// getCurrentBranch returns the current branch name
func getCurrentBranch(cwd string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getWorkingTreeStatus checks for uncommitted, staged, and untracked files
func getWorkingTreeStatus(cwd string) (uncommitted, staged, untracked bool) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return false, false, false
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		indexStatus := line[0]
		workTreeStatus := line[1]

		// Staged changes (added to index)
		if indexStatus != ' ' && indexStatus != '?' {
			staged = true
		}

		// Uncommitted changes in working tree
		if workTreeStatus != ' ' && workTreeStatus != '?' {
			uncommitted = true
		}

		// Untracked files
		if indexStatus == '?' && workTreeStatus == '?' {
			untracked = true
		}
	}

	return uncommitted, staged, untracked
}

// getRecentCommits returns the most recent commits
func getRecentCommits(cwd string, count int) []Commit {
	cmd := exec.Command("git", "log", "-n", strconv.Itoa(count), "--pretty=format:%h|%s|%an")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var commits []Commit
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) == 3 {
			commits = append(commits, Commit{
				Hash:    parts[0],
				Subject: parts[1],
				Author:  parts[2],
			})
		}
	}

	return commits
}

// getRemoteURL returns the origin remote URL
func getRemoteURL(cwd string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// getAheadBehind returns the number of commits ahead/behind the remote
func getAheadBehind(cwd string) (ahead, behind int) {
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(string(out))
	if len(parts) == 2 {
		// Note: left-right gives us behind first, then ahead
		behind, _ = strconv.Atoi(parts[0])
		ahead, _ = strconv.Atoi(parts[1])
	}

	return ahead, behind
}

// fileExists checks if a file or directory exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Summary returns a brief summary of the git state for prompts
func (c *Context) Summary() string {
	if !c.IsRepo {
		return ""
	}

	var parts []string

	// Branch
	if c.Branch != "" {
		parts = append(parts, "branch: "+c.Branch)
	}

	// Status
	var status []string
	if c.HasStaged {
		status = append(status, "staged changes")
	}
	if c.HasUncommitted {
		status = append(status, "uncommitted changes")
	}
	if c.HasUntracked {
		status = append(status, "untracked files")
	}
	if len(status) > 0 {
		parts = append(parts, strings.Join(status, ", "))
	} else {
		parts = append(parts, "clean")
	}

	// Special states
	if c.MergeInProgress {
		parts = append(parts, "MERGE IN PROGRESS")
	}
	if c.RebaseInProgress {
		parts = append(parts, "REBASE IN PROGRESS")
	}

	// Ahead/behind
	if c.Ahead > 0 || c.Behind > 0 {
		var syncStatus []string
		if c.Ahead > 0 {
			syncStatus = append(syncStatus, fmt.Sprintf("%d ahead", c.Ahead))
		}
		if c.Behind > 0 {
			syncStatus = append(syncStatus, fmt.Sprintf("%d behind", c.Behind))
		}
		parts = append(parts, strings.Join(syncStatus, ", "))
	}

	return strings.Join(parts, " | ")
}
