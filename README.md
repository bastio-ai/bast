```
  ____    _    ____ _____ ___ ___       _    ___ 
 | __ )  / \  / ___|_   _|_ _/ _ \     / \  |_ _|
 |  _ \ / _ \ \___ \ | |  | | | | |   / _ \  | | 
 | |_) / ___ \ ___) || |  | | |_| |  / ___ \ | | 
 |____/_/   \_\____/ |_| |___\___/  /_/   \_\___|
                                                 
```

# BAST CLI

**Your AI-powered terminal assistant.** Describe tasks in plain English, get shell commands instantly.

[![Go Version](https://img.shields.io/badge/go-1.24-blue)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Release](https://img.shields.io/github/v/release/bastio-ai/bast)](https://github.com/bastio-ai/bast/releases)

<!-- TODO: Add demo GIF here -->
<!-- ![bast demo](docs/demo.gif) -->

## Quick Install

```bash
curl -fsSL https://raw.githubusercontent.com/bastio-ai/bast/main/scripts/install.sh | sh
```

## Why bast?

**bast** is a free, open-source CLI built to bring security to AI-powered terminal operations. It integrates with [Bastio AI Security Gateway](https://bastio.com) to protect your data before it reaches the LLM.

**Every developer gets:**
- **100,000 FREE requests/month** — No credit card required
- **Automatic PII redaction** — Sensitive data filtered before reaching Claude
- **Jailbreak & injection protection** — Blocks prompt manipulation attempts
- **Full observability** — Track usage, costs, and request patterns

### Getting Started

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/bastio-ai/bast/main/scripts/install.sh | sh

# Run the setup wizard
bast init
```

The setup wizard will:
1. Ask if you want to use **Bastio** (recommended) or connect directly to Anthropic
2. If Bastio: Open your browser to create a free account and authenticate
3. Prompt for your Anthropic API key (stored securely with Bastio, never locally)
4. Configure your preferences (model, safe mode)

That's it — you're ready to use `bast run`.

### Direct Anthropic API (Optional)

Prefer to skip Bastio? You can connect directly to the Anthropic API:

```bash
bast init
# Select "Direct connection to Anthropic" when prompted
# Enter your Anthropic API key
```

Or set via environment variable:
```bash
export ANTHROPIC_API_KEY=sk-ant-...
export BAST_GATEWAY=direct
```

## Features

- **Natural Language to Commands** - Describe what you want, get the shell command
- **Smart Intent Detection** - Automatically knows when to generate commands vs answer questions
- **Context-Aware** - Uses your shell, OS, current directory, and command history
- **File Context with @syntax** - Reference files like `@README.md` for AI analysis
- **Dangerous Command Protection** - Warns before `rm -rf`, `dd`, and other destructive operations
- **Multi-turn Chat** - Follow-up questions with conversation history
- **Beautiful TUI** - Full terminal interface built with Bubble Tea
- **Shell Integration** - Press **Ctrl+A** to launch, **Ctrl+E** to explain commands
- **Agentic Mode** - Use `/agent` for multi-step tasks with tool execution
- **Custom Plugins** - Extend with your own tools via `~/.config/bast/tools/`
- **Error Recovery** (`bast fix`) - Analyze failed commands and get suggested fixes
- **Output Piping** (`bast explain`) - Pipe command output to AI for analysis
- **Git Integration** - Context-aware git commands with destructive operation warnings

## Quick Example

```bash
# Generate commands from natural language
$ bast run
> find all go files modified in the last week

find . -name "*.go" -mtime -7

[⏎ Run] [e Edit] [c Copy] [? Explain] [Esc Exit]
```

```bash
# Understand commands before running (Ctrl+E with shell integration)
$ git rebase -i HEAD~3           # ← Press Ctrl+E instead of Enter

Starts an interactive rebase for the last 3 commits, allowing you to
reorder, squash, edit, or drop commits. Opens your editor with a list
of commits where you can change 'pick' to 'squash', 'edit', etc.

# Or explain any command directly
$ bast explain "tar -xzvf archive.tar.gz"
```

```bash
# Ask questions and get answers
$ bast run
> what's using port 8080?

lsof -i :8080
```

## Command Explanation

Understand any command before running it:

```bash
# Type a command, hit Ctrl+E to understand it before executing
$ find . -name '*.go' -exec wc -l {} +    # ← Ctrl+E

Recursively finds all .go files and counts lines in each. The '+' batches
files into fewer wc calls for better performance than using '\;'.

# Works with complex pipelines too
$ kubectl get pods | grep -v Running | awk '{print $1}' | xargs kubectl delete pod
                                                         # ← Ctrl+E

# Or call directly without shell integration
$ bast explain "docker run -it --rm -v $(pwd):/app -w /app node:18 npm test"
```

Breaks down commands, flags, and pipelines into plain English. Especially useful for commands you found on Stack Overflow.

## Agentic Mode

For complex multi-step tasks, use `/agent` to let bast execute commands and iterate:

```bash
$ bast run
> /agent find all TODO comments in go files and summarize them

Tool Calls:
  run_command {"command": "grep -r 'TODO' --include='*.go' ."}
    internal/ai/anthropic.go:// TODO: add streaming support
    internal/tools/loader.go:// TODO: validate script permissions
    ...

Response:
Found 2 TODO comments in the codebase:
1. `internal/ai/anthropic.go` - Add streaming support for responses
2. `internal/tools/loader.go` - Validate script permissions before execution
```

Built-in tools: `run_command`, `read_file`, `list_directory`, `write_file`

## Error Recovery

Fix failed commands with AI-powered analysis:

```bash
# After a command fails
$ ls /nonexistent
ls: /nonexistent: No such file or directory

$ bast fix
Analyzing: ls /nonexistent
Suggested fix:
  ls /

The directory /nonexistent doesn't exist. Did you mean the root directory?
```

## Output Piping

Pipe any command output to AI for explanation:

```bash
# Explain command output
$ kubectl get pods | bast explain
$ kubectl get pods | bast explain "any failing?"
$ cat error.log | bast explain "why is it crashing"
$ docker ps | bast explain
```

## Git Integration

bast automatically detects when you're in a git repository and uses your repo state to give better suggestions, smarter commands, and safety warnings.

### Git Context Awareness

Every time you run bast inside a git repo, the AI sees:

- **Current branch** — e.g., `feature/auth`, `main`, `HEAD` (detached)
- **Working tree status** — staged changes, uncommitted modifications, untracked files
- **Merge/rebase state** — detects in-progress merges and rebases
- **Recent commits** — last 5 commits with hash, subject, and author
- **Ahead/behind tracking** — how many commits ahead or behind the remote

This context improves every interaction — not just git commands. For example, asking "what did I change?" or "summarize my work" uses your repo state to give accurate answers.

### Context-Aware Command Generation

```bash
# bast uses your repo state to generate accurate commands
$ bast run
> commit my changes with a good message

# bast sees: branch 'feature/auth', 3 staged files, 2 ahead of origin
git commit -m "Add JWT authentication middleware and refresh token handling"
```

```bash
# Merge conflict? bast knows and helps resolve it
$ bast run
> help me finish this merge

# bast sees: MERGE IN PROGRESS, 2 conflicted files
# Suggests steps to resolve conflicts and complete the merge
```

### Protected Git Operations

Dangerous operations require explicit confirmation before execution:

```bash
$ bast run
> force push to origin

git push --force origin feature/auth

⚠️  WARNING: This command may be destructive!
Type 'yes' to execute, or ask a follow-up question:
```

**Full list of protected git operations:**
- `git push --force` / `-f` — Force push
- `git push --force-with-lease` — Force push (still destructive)
- `git reset --hard` — Discard all changes
- `git clean -fd` — Remove untracked files/directories
- `git checkout -- .` — Discard all working tree changes
- `git branch -d` / `-D` — Delete branch
- `git rebase` — History rewriting
- `git commit --amend` — Rewrite last commit
- `git push ...:...` — Delete remote ref
- `git stash drop` / `clear` — Permanently discard stashed changes
- `git reflog expire` — Expire reflog entries
- `git gc --prune` — Prune unreachable objects
- `git filter-branch` — Rewrite repository history
- `git push origin main/master` — Push to protected branches

### Git Error Recovery

When a git command fails, `bast fix` analyzes the error and suggests a solution:

```bash
$ git push origin feature/auth
! [rejected]        feature/auth -> feature/auth (non-fast-forward)

$ bast fix
Analyzing: git push origin feature/auth

The remote branch has commits you don't have locally.
Suggested fix:
  git pull --rebase origin feature/auth

Pull remote changes and replay your commits on top.
```

### Git Command Explanation

Understand git commands before running them with **Ctrl+E** or `bast explain`:

```bash
$ git rebase -i HEAD~3           # ← Press Ctrl+E instead of Enter

Starts an interactive rebase for the last 3 commits, allowing you to
reorder, squash, edit, or drop commits. Opens your editor with a list
of commits where you can change 'pick' to 'squash', 'edit', etc.
```

### Git Summary in Agentic Mode

In agentic mode (`/agent`), bast has a built-in `git_summary` tool that provides a quick overview of branch, status, recent commits, and uncommitted changes — useful for multi-step workflows that need to inspect repo state.

## Custom Plugins

**Turn any script into an AI-powered tool.** Plugins let you extend bast with your own commands, workflows, and integrations—making the AI aware of your specific toolchain.

Use cases:
- **Deployment pipelines** - Deploy to staging/production with natural language
- **Database operations** - Run migrations, backups, or queries safely
- **CI/CD integration** - Trigger builds, check status, review logs
- **Custom workflows** - Wrap complex multi-step processes into simple commands

Create plugins in `~/.config/bast/tools/` with simple YAML manifests:

```yaml
# ~/.config/bast/tools/git-status.yaml
name: git_status
description: Get git repository status
command: git status --short
parameters: []
```

```yaml
# ~/.config/bast/tools/deploy/manifest.yaml
name: deploy
description: Deploy to staging environment
command: ./deploy.sh $ENVIRONMENT
parameters:
  - name: environment
    type: string
    description: Target environment (staging/production)
    required: true
```

Plugins are automatically discovered and available in agentic mode. The AI understands your tools' descriptions and parameters, choosing the right ones for each task.

## Quick Start

```bash
# Interactive setup (configure API key)
bast init

# Launch TUI
bast run

# With initial query
bast run --query "find all go files modified today"
```

## Shell Integration

Add to your shell config for keyboard shortcuts:

```bash
# ~/.zshrc
eval "$(bast hook zsh)"

# ~/.bashrc
eval "$(bast hook bash)"
```

Then restart your terminal.

**Keyboard Shortcuts:**
- **Ctrl+A** - Launch bast TUI from any prompt
- **Ctrl+E** - Explain the command currently typed (without executing)

## Configuration

Config file: `~/.config/bast/config.yaml`

```yaml
mode: safe              # safe (confirm before execute) or yolo
provider: anthropic
api_key: sk-ant-...
model: claude-sonnet-4-20250514
```

Environment variables:
- `ANTHROPIC_API_KEY` or `BAST_API_KEY` - API key override
- `BAST_*` prefix overrides config file settings

## Security

- Sensitive files blocked from reading (.env, credentials, keys)
- Dangerous command patterns trigger confirmation before execution
- File access restricted to current working directory

## Development

### Build from Source

```bash
git clone https://github.com/bastio-ai/bast.git
cd bast
go build .

# Build with version info
go build -ldflags="-X github.com/bastio-ai/bast/cmd.Version=0.1.0" .

# Run tests
go test ./...
```

## Releasing

This project uses [GoReleaser](https://goreleaser.com/) for automated multi-platform builds.

### Prerequisites
- [GoReleaser](https://goreleaser.com/install/) installed

### Creating a Release

1. Tag the release:
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   git push origin v0.1.0
   ```

2. GitHub Actions automatically:
   - Builds binaries for darwin/linux (amd64/arm64)
   - Creates GitHub release with artifacts and checksums

### Local Testing
```bash
# Test build without publishing
goreleaser build --snapshot --clean

# Test full release process
goreleaser release --snapshot --clean
```

### Configuration
Release configuration is in `.goreleaser.yaml`. Key settings:
- Builds for `darwin` and `linux` on `amd64` and `arm64`
- Creates `.tar.gz` archives with SHA256 checksums

---

## License

MIT
