package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const spinnerMessage = "Waiting for response..."

func runBackend(backend backendDefinition, question string) int {
	if backend.Mode == backendModeCodexJSON {
		return runCodex(backend, question)
	}

	return runStreamingBackend(backend, question)
}

func runStreamingBackend(backend backendDefinition, question string) int {
	cmd := exec.Command(backend.Path, append(backend.Args, question)...)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	spinner := startSpinner(os.Stderr, shouldShowSpinner(os.Stderr), spinnerMessage)

	var stopOnce sync.Once
	stopSpinner := func() {
		stopOnce.Do(func() {
			spinner.Stop()
		})
	}

	results := make(chan copyResult, 2)
	var firstOutput sync.Once
	go func() {
		results <- copyResult{
			stream: "stdout",
			err:    proxyCommandOutput(os.Stdout, stdout, &firstOutput, stopSpinner),
		}
	}()
	go func() {
		results <- copyResult{
			stream: "stderr",
			err:    proxyCommandOutput(os.Stderr, stderr, &firstOutput, stopSpinner),
		}
	}()

	waitErr := cmd.Wait()
	stopSpinner()

	for i := 0; i < 2; i++ {
		result := <-results
		if result.err != nil {
			fmt.Fprintf(os.Stderr, "failed to stream backend %s: %v\n", result.stream, result.err)
			return 1
		}
	}

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			return exitErr.ExitCode()
		}

		fmt.Fprintln(os.Stderr, waitErr)
		return 1
	}

	return 0
}

func runCodex(backend backendDefinition, question string) int {
	cmd := exec.Command(backend.Path, append(backend.Args, question)...)
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

	spinner := startSpinner(os.Stderr, shouldShowSpinner(os.Stderr), spinnerMessage)
	message, parseErr := extractCodexMessage(stdout)
	waitErr := cmd.Wait()
	spinner.Stop()

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

type copyResult struct {
	stream string
	err    error
}

func proxyCommandOutput(dst io.Writer, src io.Reader, once *sync.Once, onFirstOutput func()) error {
	_, err := io.Copy(&firstOutputWriter{
		dst:           dst,
		once:          once,
		onFirstOutput: onFirstOutput,
	}, src)
	return err
}

type firstOutputWriter struct {
	dst           io.Writer
	once          *sync.Once
	onFirstOutput func()
}

func (w *firstOutputWriter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.onFirstOutput != nil {
		if w.once != nil {
			w.once.Do(w.onFirstOutput)
		} else {
			w.onFirstOutput()
		}
	}

	return w.dst.Write(p)
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
