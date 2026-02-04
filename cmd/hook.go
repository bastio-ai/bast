package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook [shell]",
	Short: "Output shell hook script",
	Long:  `Output the shell integration script for the specified shell (zsh or bash).`,
	Args:  cobra.ExactArgs(1),
	RunE:  runHook,
}

func init() {
	rootCmd.AddCommand(hookCmd)
}

func runHook(cmd *cobra.Command, args []string) error {
	// Get absolute path to this executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	shell := args[0]

	switch shell {
	case "zsh":
		fmt.Printf(zshHookTemplate, exePath, exePath)
	case "bash":
		fmt.Printf(bashHookTemplate, exePath, exePath)
	default:
		return fmt.Errorf("unsupported shell: %s (supported: zsh, bash)", shell)
	}

	return nil
}

const zshHookTemplate = `# bast shell integration for zsh
# Add to your .zshrc: eval "$(bast hook zsh)"

# Temp files for capturing output
_bast_stdout_file="${TMPDIR:-/tmp}/bast_stdout.$$"
_bast_stderr_file="${TMPDIR:-/tmp}/bast_stderr.$$"

# Store last command and exit status for context
_bast_preexec() {
    export BAST_LAST_CMD="$1"
    # Clear previous output files
    : > "$_bast_stdout_file" 2>/dev/null
    : > "$_bast_stderr_file" 2>/dev/null
}

_bast_precmd() {
    export BAST_EXIT_STATUS="$?"
    # Read captured output if available (truncated to 2KB)
    if [[ -f "$_bast_stdout_file" ]]; then
        export BAST_LAST_OUTPUT="$(head -c 2048 "$_bast_stdout_file" 2>/dev/null)"
    fi
    if [[ -f "$_bast_stderr_file" ]]; then
        export BAST_LAST_ERROR="$(head -c 2048 "$_bast_stderr_file" 2>/dev/null)"
    fi
}

# Register hooks
autoload -Uz add-zsh-hook
add-zsh-hook preexec _bast_preexec
add-zsh-hook precmd _bast_precmd

# Wrapper function to capture command output (optional, use: bast_capture <command>)
bast_capture() {
    "$@" > >(tee "$_bast_stdout_file") 2> >(tee "$_bast_stderr_file" >&2)
}

# Launch bast with Ctrl+A
_bast_widget() {
    local saved_buffer="$BUFFER"
    local saved_cursor="$CURSOR"

    # Create temp file for output with secure permissions
    local tmpfile=$(mktemp "${TMPDIR:-/tmp}/bast.XXXXXX")
    chmod 600 "$tmpfile"

    # Clear line for TUI
    BUFFER=""
    zle redisplay

    # Sync history to file before launching bast
    fc -AI 2>/dev/null

    # Run bast directly (not in subshell) - TUI gets proper terminal I/O
    "%s" run --output-file "$tmpfile"

    # Read result from temp file
    if [[ -f "$tmpfile" ]]; then
        local output=$(cat "$tmpfile")
        rm -f "$tmpfile"

        if [[ "$output" == BAST_COMMAND:* ]]; then
            BUFFER="${output#BAST_COMMAND:}"
            CURSOR=${#BUFFER}
        else
            BUFFER="$saved_buffer"
            CURSOR="$saved_cursor"
        fi
    else
        BUFFER="$saved_buffer"
        CURSOR="$saved_cursor"
    fi

    zle redisplay
}

zle -N _bast_widget
bindkey '^A' _bast_widget

# Explain command with Ctrl+E (without executing)
_bast_explain_widget() {
    local cmd="$BUFFER"
    if [[ -n "$cmd" ]]; then
        # Invalidate display to allow external command output
        zle -I
        printf '\n'
        "%s" explain "$cmd"
        printf '\n'
    fi
    zle reset-prompt
}
zle -N _bast_explain_widget
bindkey '^E' _bast_explain_widget
`

const bashHookTemplate = `# bast shell integration for bash
# Add to your .bashrc: eval "$(bast hook bash)"

# Temp files for capturing output
_bast_stdout_file="${TMPDIR:-/tmp}/bast_stdout.$$"
_bast_stderr_file="${TMPDIR:-/tmp}/bast_stderr.$$"

# Store last command for context
_bast_preexec() {
    export BAST_LAST_CMD="$BASH_COMMAND"
    # Clear previous output files
    : > "$_bast_stdout_file" 2>/dev/null
    : > "$_bast_stderr_file" 2>/dev/null
}

trap '_bast_preexec' DEBUG

# Store exit status
PROMPT_COMMAND="_bast_precmd${PROMPT_COMMAND:+; $PROMPT_COMMAND}"

_bast_precmd() {
    export BAST_EXIT_STATUS="$?"
    # Read captured output if available (truncated to 2KB)
    if [[ -f "$_bast_stdout_file" ]]; then
        export BAST_LAST_OUTPUT="$(head -c 2048 "$_bast_stdout_file" 2>/dev/null)"
    fi
    if [[ -f "$_bast_stderr_file" ]]; then
        export BAST_LAST_ERROR="$(head -c 2048 "$_bast_stderr_file" 2>/dev/null)"
    fi
}

# Wrapper function to capture command output (optional, use: bast_capture <command>)
bast_capture() {
    "$@" > >(tee "$_bast_stdout_file") 2> >(tee "$_bast_stderr_file" >&2)
}

# Launch bast with Ctrl+A
_bast_readline() {
    local saved_line="$READLINE_LINE"
    local saved_point="$READLINE_POINT"

    # Create temp file for output with secure permissions
    local tmpfile=$(mktemp "${TMPDIR:-/tmp}/bast.XXXXXX")
    chmod 600 "$tmpfile"

    # Sync history to file before launching bast
    history -a 2>/dev/null

    # Run bast directly (not in subshell) - TUI gets proper terminal I/O
    "%s" run --output-file "$tmpfile"

    # Read result from temp file
    if [[ -f "$tmpfile" ]]; then
        local output=$(cat "$tmpfile")
        rm -f "$tmpfile"

        if [[ "$output" == BAST_COMMAND:* ]]; then
            READLINE_LINE="${output#BAST_COMMAND:}"
            READLINE_POINT=${#READLINE_LINE}
        else
            READLINE_LINE="$saved_line"
            READLINE_POINT="$saved_point"
        fi
    else
        READLINE_LINE="$saved_line"
        READLINE_POINT="$saved_point"
    fi
}

bind -x '"\C-a": _bast_readline'

# Explain command with Ctrl+E (without executing)
_bast_explain_readline() {
    local cmd="$READLINE_LINE"
    if [[ -n "$cmd" ]]; then
        printf '\n'
        "%s" explain "$cmd"
        printf '\n'
    fi
}
bind -x '"\C-e": _bast_explain_readline'
`
