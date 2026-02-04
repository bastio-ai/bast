package shell

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// GetHistory reads the last N commands from the shell history file.
// For freshest results, configure your shell to write history immediately:
//
//	zsh:  setopt INC_APPEND_HISTORY
//	bash: PROMPT_COMMAND="history -a"
func GetHistory(shell string, count int) []string {
	histFile := getHistoryFile(shell)
	if histFile == "" {
		return nil
	}

	file, err := os.Open(histFile)
	if err != nil {
		return nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	// Handle long commands
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		cmd := parseHistoryLine(line, shell)
		if cmd != "" {
			lines = append(lines, cmd)
		}
	}

	// Return last `count` commands
	if len(lines) > count {
		return lines[len(lines)-count:]
	}
	return lines
}

func getHistoryFile(shell string) string {
	// Check HISTFILE first (user override)
	if histFile := os.Getenv("HISTFILE"); histFile != "" {
		if strings.HasPrefix(histFile, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				histFile = filepath.Join(home, histFile[1:])
			}
		}
		return histFile
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch shell {
	case "zsh":
		return filepath.Join(home, ".zsh_history")
	case "bash":
		return filepath.Join(home, ".bash_history")
	default:
		return ""
	}
}

func parseHistoryLine(line, shell string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	switch shell {
	case "zsh":
		// Handle zsh extended format: ": timestamp:duration;command"
		if strings.HasPrefix(line, ": ") {
			if _, after, found := strings.Cut(line, ";"); found {
				return strings.TrimSpace(after)
			}
		}
		return line
	case "bash":
		// Skip timestamp lines (when HISTTIMEFORMAT is set)
		if strings.HasPrefix(line, "#") {
			return ""
		}
		return line
	default:
		return line
	}
}
