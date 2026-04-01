package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type backendConfig struct {
	binary string
	args   []string
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	flags := flag.NewFlagSet("qq", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	backendName := flags.String("backend", "codex", "AI backend to use: codex, claude, or gemini")

	if err := flags.Parse(args); err != nil {
		return 2
	}

	question := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if question == "" {
		fmt.Fprintln(os.Stderr, `usage: qq [--backend codex|claude|gemini] "your question"`)
		return 2
	}

	cfg, err := buildBackend(*backendName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	if _, err := exec.LookPath(cfg.binary); err != nil {
		fmt.Fprintf(os.Stderr, "%s is not installed or not on PATH\n", cfg.binary)
		return 127
	}

	cmd := exec.Command(cfg.binary, append(cfg.args, question)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}

		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return 0
}

func buildBackend(name string) (backendConfig, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "codex":
		return backendConfig{
			binary: "codex",
			args: []string{
				"exec",
				"--ephemeral",
				"--skip-git-repo-check",
				"--sandbox",
				"read-only",
			},
		}, nil
	case "claude":
		return backendConfig{
			binary: "claude",
			args:   []string{"-p"},
		}, nil
	case "gemini":
		return backendConfig{
			binary: "gemini",
			args:   []string{"-p"},
		}, nil
	default:
		return backendConfig{}, fmt.Errorf("unsupported backend %q", name)
	}
}
