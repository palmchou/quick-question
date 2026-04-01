# qq — quick-question

A minimal CLI tool for asking quick questions to AI assistants from your terminal.

## Usage

```bash
qq "what does git rebase --onto do?"
qq --backend claude "explain CAP theorem in one paragraph"
qq --backend gemini "what is tail recursion?"
```

## Install

*TODO*

## Build

```bash
go build -o qq .
```

## Backends

`qq` forwards a question to your AI CLI of choice and streams the response back. It supports three backends:

| Backend | Command invoked |
|---------|-----------------|
| `codex` | `codex exec --ephemeral --skip-git-repo-check --sandbox read-only` |
| `claude`| `claude -p` |
| `gemini`| `gemini -p` |

### Options

| Flag | Description |
|------|-------------|
| `--backend` | AI backend to use: `codex`, `claude`, or `gemini` (default: `codex`) |

## Notes

- The chosen backend's binary must be installed and available on `PATH`.
