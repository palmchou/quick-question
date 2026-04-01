# qq — quick-question

A minimal CLI tool for asking quick questions to AI assistants from your terminal.

## Usage

```bash
qq "what does git rebase --onto do?"
qq --backend claude "explain CAP theorem in one paragraph"
qq --backend gemini "what is tail recursion?"
```

## Install

```bash
go install github.com/palmchou/quick-question/cmd/qq@latest
```

This installs a `qq` binary into your Go bin directory, which is usually `~/go/bin`.

If `qq` is not found after install, add `~/go/bin` to your shell `PATH`. For `zsh`:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## Prerequisites

- The chosen backend's binary must be installed and available on `PATH`.
- Supported backends are `codex`, `claude`, and `gemini`.
- You must already be authenticated directly with the respective CLI before using `qq`.

## Backends

`qq` forwards a question to your AI CLI of choice and streams the response back. It supports three backends:

| Backend | Command invoked |
|---------|-----------------|
| `codex` | `codex exec --ephemeral --skip-git-repo-check --json --sandbox read-only` |
| `claude`| `claude -p` |
| `gemini`| `gemini -p` |

### Options

| Flag | Description |
|------|-------------|
| `--backend` | AI backend to use: `codex`, `claude`, or `gemini` (default: `codex`) |

## Notes

- `go install github.com/palmchou/quick-question@latest` would produce a `quick-question` binary, not `qq`. To install a `qq` binary directly, the Go entrypoint lives at `cmd/qq`.
