# qq — quick-question

A minimal CLI tool for asking quick questions to AI assistants from your terminal.

## Usage

```bash
qq "what does git rebase --onto do?"
qq --backend claude "explain CAP theorem in one paragraph"
qq --backend gemini "what is tail recursion?"
qq -c "explain the files in this directory"
qq --backend work-claude "summarize Raft leader election"
```

By default, `qq` uses `codex`, unless you override that in config. `--backend` can target either a built-in backend or a named custom backend from your config file.

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
- Built-in backends are `codex`, `claude`, and `gemini`. You can also define additional named backends in config.
- You must already be authenticated directly with the respective CLI before using `qq`.

## Configuration

`qq` reads optional configuration from:

```bash
~/.config/qq/config.json
```

The config file can set a default backend, override built-in backends, and define entirely new named backends. Built-in backends are treated as default backend definitions, so `backends.codex`, `backends.claude`, and `backends.gemini` use the same `path` and `args` shape as any custom backend:

```json
{
  "default_backend": "work-claude",
  "backends": {
    "codex": {
      "path": "/opt/homebrew/bin/codex",
      "args": ["exec", "--ephemeral", "--skip-git-repo-check", "--json", "--sandbox", "read-only"]
    },
    "claude": {
      "path": "/Users/you/bin/claude",
      "args": ["-p"]
    },
    "work-claude": {
      "path": "/Users/you/bin/claude",
      "args": ["-p", "--model", "sonnet"]
    },
    "local-wrapper": {
      "path": "/Users/you/bin/ask-ai",
      "args": ["--stdio"]
    }
  }
}
```

Fields:

- `default_backend`: optional default backend when `--backend` is not provided
- `backends`: optional named backend definitions
- `backends.<name>.path`: optional binary path for that backend
- `backends.<name>.args`: optional argument list for that backend
- `backend_paths`: optional legacy path-only overrides for built-in backends

The `--backend` flag still takes precedence over `default_backend`.

## Backends

`qq` forwards a question to your AI CLI of choice and streams the response back. These are the built-in backend definitions:

| Backend | Command invoked |
|---------|-----------------|
| `codex` | `codex exec --ephemeral --skip-git-repo-check --json --sandbox read-only` |
| `claude`| `claude -p` |
| `gemini`| `gemini -p` |

Built-in backends run from a fresh, empty temporary working directory on each invocation by default. Pass `-c` or `--cwd-context` to run them from your current working directory instead.

Custom backends use the configured `path` and `args` from `config.json` and inherit the caller's current working directory unless the configured wrapper changes it.

### Options

| Flag | Description |
|------|-------------|
| `--backend` | Backend name to use. This can be a built-in backend or a custom backend defined in config. |
| `-c`, `--cwd-context` | Use the current working directory as backend context instead of the default empty temporary directory. |

## Notes

- `go install github.com/palmchou/quick-question@latest` would produce a `quick-question` binary, not `qq`. To install a `qq` binary directly, the Go entrypoint lives at `cmd/qq`.
- Built-in backends (`codex`, `claude`, and `gemini`) start in a fresh empty temporary directory by default, even if you override their `path` or `args` in config. Pass `-c` or `--cwd-context` when you want the built-in backend to see your current directory instead.
- On interactive terminals, `qq` shows a small spinner on `stderr` while the selected backend is still silent. It clears itself as soon as output starts or the command finishes, so redirected output is not polluted.
- If you override the built-in `codex` backend, keep `--json` in its args. `qq` expects Codex JSON output so it can print the final answer cleanly.
