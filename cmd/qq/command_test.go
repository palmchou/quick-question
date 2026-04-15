package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestExtractCodexMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name: "returns final completed agent message",
			input: strings.Join([]string{
				`{"type":"item.started","item":{"type":"agent_message","text":"draft"}}`,
				`{"type":"item.completed","item":{"type":"tool_call","text":"ignored"}}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"first answer"}}`,
				`{"type":"item.completed","item":{"type":"agent_message","text":"final answer"}}`,
			}, "\n"),
			want: "final answer",
		},
		{
			name: "trims whitespace and ignores empty lines",
			input: "\n" + strings.Join([]string{
				`{"type":"item.completed","item":{"type":"agent_message","text":"  spaced answer  "}}`,
				"",
			}, "\n"),
			want: "spaced answer",
		},
		{
			name: "returns empty when no completed agent message exists",
			input: strings.Join([]string{
				`{"type":"item.started","item":{"type":"agent_message","text":"draft"}}`,
				`{"type":"item.completed","item":{"type":"tool_call","text":"ignored"}}`,
			}, "\n"),
			want: "",
		},
		{
			name:    "fails on invalid json",
			input:   `{"type":"item.completed","item":{"type":"agent_message","text":"ok"}}` + "\n" + `not-json`,
			wantErr: "failed to parse codex JSON output",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := extractCodexMessage(strings.NewReader(tt.input))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestProxyCommandOutputCopiesAndStopsOnceAcrossStreams(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var firstOutput sync.Once
	stops := 0
	stopSpinner := func() {
		stops++
	}

	if err := proxyCommandOutput(&stdout, strings.NewReader("hello"), &firstOutput, stopSpinner); err != nil {
		t.Fatalf("unexpected error copying stdout: %v", err)
	}
	if err := proxyCommandOutput(io.Discard, strings.NewReader("world"), &firstOutput, stopSpinner); err != nil {
		t.Fatalf("unexpected error copying stderr: %v", err)
	}

	if stdout.String() != "hello" {
		t.Fatalf("expected copied stdout %q, got %q", "hello", stdout.String())
	}
	if stops != 1 {
		t.Fatalf("expected spinner stop callback once, got %d", stops)
	}
}

func TestProxyCommandOutputSkipsStopWhenNoOutputArrives(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var firstOutput sync.Once
	stops := 0

	if err := proxyCommandOutput(&stdout, strings.NewReader(""), &firstOutput, func() {
		stops++
	}); err != nil {
		t.Fatalf("unexpected error copying output: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no copied output, got %q", stdout.String())
	}
	if stops != 0 {
		t.Fatalf("expected spinner stop callback not to run, got %d", stops)
	}
}

func TestPrepareBackendCommandUsesTempDirWhenConfigured(t *testing.T) {
	t.Parallel()

	cmd, cleanup, err := prepareBackendCommand(backendDefinition{
		Path:       "claude",
		Args:       []string{"-p"},
		UseTempDir: true,
	}, "what is tail recursion?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cmd.Dir == "" {
		t.Fatal("expected command to run from a temporary directory")
	}

	entries, err := os.ReadDir(cmd.Dir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected temp dir to start empty, got %d entries", len(entries))
	}

	cleanup()

	if _, err := os.Stat(cmd.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cleanup to remove temp dir, got err=%v", err)
	}
}

func TestPrepareBackendCommandLeavesDirUnsetWhenDisabled(t *testing.T) {
	t.Parallel()

	cmd, cleanup, err := prepareBackendCommand(backendDefinition{
		Path: "claude",
		Args: []string{"-p"},
	}, "what is tail recursion?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if cmd.Dir != "" {
		t.Fatalf("expected command to inherit the caller working directory, got %q", cmd.Dir)
	}
}
