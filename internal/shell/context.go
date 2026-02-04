package shell

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/bastio-ai/bast/internal/ai"
	"github.com/bastio-ai/bast/internal/git"
)

// GetContext retrieves the current shell context from environment variables
func GetContext() ai.ShellContext {
	cwd := getCWD()
	ctx := ai.ShellContext{
		CWD:   cwd,
		OS:    runtime.GOOS,
		Shell: getShell(),
		User:  getUser(),
	}

	// Get last command and exit status from environment (set by shell hook)
	if lastCmd := os.Getenv("BAST_LAST_CMD"); lastCmd != "" {
		ctx.LastCommand = lastCmd
	}

	if exitStatus := os.Getenv("BAST_EXIT_STATUS"); exitStatus != "" {
		if status, err := strconv.Atoi(exitStatus); err == nil {
			ctx.ExitStatus = status
		}
	}

	// Get git context if in a repository
	gitCtx := git.GetContext(cwd)
	if gitCtx.IsRepo {
		ctx.Git = &ai.GitContext{
			IsRepo:           gitCtx.IsRepo,
			Branch:           gitCtx.Branch,
			HasUncommitted:   gitCtx.HasUncommitted,
			HasUntracked:     gitCtx.HasUntracked,
			HasStaged:        gitCtx.HasStaged,
			MergeInProgress:  gitCtx.MergeInProgress,
			RebaseInProgress: gitCtx.RebaseInProgress,
			Summary:          gitCtx.Summary(),
		}
	}

	return ctx
}

func getCWD() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return cwd
}

func getShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}
	return filepath.Base(shell)
}

func getUser() string {
	u, err := user.Current()
	if err != nil {
		return os.Getenv("USER")
	}
	return u.Username
}

// GetContextWithHistory returns shell context with history included
func GetContextWithHistory() ai.ShellContext {
	ctx := GetContext()
	ctx.History = GetHistory(ctx.Shell, 20)

	// Read last output/error from env vars (set by shell hook)
	if lastOutput := os.Getenv("BAST_LAST_OUTPUT"); lastOutput != "" {
		ctx.LastOutput = truncate(lastOutput, 2000)
	}
	if lastError := os.Getenv("BAST_LAST_ERROR"); lastError != "" {
		ctx.LastError = truncate(lastError, 2000)
	}

	return ctx
}

// truncate limits a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
