# qq — quick-question

A minimal CLI tool for asking quick questions to AI assistants from your terminal.

## Usage

```bash
qq "what does git rebase --onto do?"
qq --backend claude "explain CAP theorem in one paragraph"
qq --backend gemini "what is tail recursion?"
```

By default, `qq` uses `codex`, unless you override that in config.

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

- The chosen backend's binary must be installed and either available on `PATH` or configured explicitly in `qq`'s config file.
- Supported backends are `codex`, `claude`, and `gemini`.
- You must already be authenticated directly with the respective CLI before using `qq`.

## Configuration

`qq` reads optional configuration from:

```bash
~/.config/qq/config.json
```

The config file can set a default backend and explicit paths to backend CLIs:

```json
{
  "default_backend": "claude",
  "backend_paths": {
    "codex": "/opt/homebrew/bin/codex",
    "claude": "/Users/you/bin/claude",
    "gemini": "/Users/you/bin/gemini"
  }
}
```

Fields:

- `default_backend`: optional default backend when `--backend` is not provided
- `backend_paths`: optional per-backend binary paths to use instead of looking on `PATH`

The `--backend` flag still takes precedence over `default_backend`.

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
