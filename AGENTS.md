# AGENTS.md

This repository contains a small Go CLI named `qq`.

## Purpose

- `qq` forwards a single question to one of three AI CLIs: `codex`, `claude`, or `gemini`.
- The executable users install is `qq`.
- The Go entrypoint lives at `cmd/qq` so `go install github.com/palmchou/quick-question/cmd/qq@latest` produces a `qq` binary.

## Repo Layout

- `cmd/qq/main.go`: main CLI entrypoint
- `README.md`: user-facing install and usage docs
- `LICENSE`: MIT license

## Behavior

- Default backend is `codex`.
- Supported flag: `--backend`
- Expected usage:

```bash
qq "your question"
qq --backend claude "your question"
qq --backend gemini "your question"
```

- Backend commands currently invoked:
  - `codex exec --ephemeral --skip-git-repo-check --sandbox read-only`
  - `claude -p`
  - `gemini -p`

## Development

- Build with:

```bash
go build -o qq ./cmd/qq
```

- Quick compile check:

```bash
go build ./cmd/qq
```

## Editing Guidelines

- Keep the CLI minimal.
- Prefer matching documented README behavior exactly.
- If you change flags, install path, or backend invocation behavior, update `README.md` in the same change.
- Do not rename the entrypoint away from `cmd/qq` unless you also intentionally change the install story.
- Avoid adding dependencies unless they materially improve the CLI.

## Release Notes

- This repo already has an initial GitHub release/tag: `v0.1.0`.
