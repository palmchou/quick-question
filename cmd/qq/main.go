package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
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

	if strings.EqualFold(*backendName, "codex") {
		return runCodex(cfg, question)
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

func runCodex(cfg backendConfig, question string) int {
	cmd := exec.Command(cfg.binary, append(cfg.args, question)...)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	message, parseErr := extractCodexMessage(stdout)
	waitErr := cmd.Wait()

	if waitErr != nil {
		if stderr.Len() > 0 {
			_, _ = io.Copy(os.Stderr, &stderr)
			if !bytes.HasSuffix(stderr.Bytes(), []byte("\n")) {
				fmt.Fprintln(os.Stderr)
			}
		}

		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return exitErr.ExitCode()
		}

		fmt.Fprintln(os.Stderr, waitErr)
		return 1
	}

	if parseErr != nil {
		fmt.Fprintln(os.Stderr, parseErr)
		return 1
	}

	if message == "" {
		fmt.Fprintln(os.Stderr, "codex completed without a final agent message")
		return 1
	}

	fmt.Fprintln(os.Stdout, message)
	return 0
}

func extractCodexMessage(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var message string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event codexEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return "", fmt.Errorf("failed to parse codex JSON output: %w", err)
		}

		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			message = strings.TrimSpace(event.Item.Text)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read codex output: %w", err)
	}

	return message, nil
}

type codexEvent struct {
	Type string `json:"type"`
	Item struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
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
				"--json",
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
