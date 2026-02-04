#!/bin/zsh
# bast shell integration for zsh
# Usage: eval "$(bast hook zsh)"
# Or source this file: source /path/to/zsh.sh

# Store last command and exit status for context
_bast_preexec() {
    export BAST_LAST_CMD="$1"
}

_bast_precmd() {
    export BAST_EXIT_STATUS="$?"
}

# Register hooks
autoload -Uz add-zsh-hook
add-zsh-hook preexec _bast_preexec
add-zsh-hook precmd _bast_precmd

# Launch bast with Ctrl+A
_bast_widget() {
    # Save current buffer
    local saved_buffer="$BUFFER"
    local saved_cursor="$CURSOR"

    # Clear line for TUI
    BUFFER=""
    zle redisplay

    # Run bast and capture output
    local output
    output=$(bast run 2>&1)

    # Check if a command was selected
    if [[ "$output" == *"BAST_COMMAND:"* ]]; then
        # Extract command after BAST_COMMAND:
        local cmd="${output#*BAST_COMMAND:}"
        BUFFER="$cmd"
        CURSOR=${#BUFFER}
    else
        # Restore original buffer if cancelled
        BUFFER="$saved_buffer"
        CURSOR="$saved_cursor"
    fi

    zle redisplay
}

zle -N _bast_widget
bindkey '^A' _bast_widget
