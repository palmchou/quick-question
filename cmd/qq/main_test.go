package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestWrapQuestion(t *testing.T) {
	t.Parallel()

	question := "what is tail recursion?"
	got := wrapQuestion(question)
	want := fmt.Sprintf(defaultSystemPrompt, question)

	if got != want {
		t.Fatalf("expected wrapped prompt %q, got %q", want, got)
	}
}

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

func TestBuildBuiltinBackendCodexIncludesJSONFlag(t *testing.T) {
	t.Parallel()

	cfg, ok := buildBuiltinBackend("codex")
	if !ok {
		t.Fatal("expected codex builtin backend")
	}

	if cfg.binary != "codex" {
		t.Fatalf("expected codex binary, got %q", cfg.binary)
	}

	found := false
	for _, arg := range cfg.args {
		if arg == "--json" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected codex backend args to include --json")
	}
	if !cfg.useTempDir {
		t.Fatal("expected codex builtin backend to use a temporary working directory")
	}
}

func TestLoadUserConfigMissingFile(t *testing.T) {
	t.Parallel()

	cfg, err := loadUserConfig(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultBackend != "" {
		t.Fatalf("expected empty default backend, got %q", cfg.DefaultBackend)
	}
	if len(cfg.BackendPaths) != 0 {
		t.Fatalf("expected no backend paths, got %#v", cfg.BackendPaths)
	}
	if len(cfg.Backends) != 0 {
		t.Fatalf("expected no custom backends, got %#v", cfg.Backends)
	}
}

func TestLoadUserConfigParsesJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	want := userConfig{
		DefaultBackend: "claude",
		BackendPaths: map[string]string{
			"codex":  "/opt/codex/bin/codex",
			"claude": "/opt/claude/bin/claude",
		},
		Backends: map[string]configuredBackend{
			"work-claude": {
				Path: "/Users/you/bin/claude-wrapper",
				Args: []string{"-p", "--model", "sonnet"},
			},
		},
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	got, err := loadUserConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.DefaultBackend != want.DefaultBackend {
		t.Fatalf("expected default backend %q, got %q", want.DefaultBackend, got.DefaultBackend)
	}
	if got.BackendPaths["codex"] != want.BackendPaths["codex"] {
		t.Fatalf("expected codex path %q, got %q", want.BackendPaths["codex"], got.BackendPaths["codex"])
	}
	if got.BackendPaths["claude"] != want.BackendPaths["claude"] {
		t.Fatalf("expected claude path %q, got %q", want.BackendPaths["claude"], got.BackendPaths["claude"])
	}
	if got.Backends["work-claude"].Path != want.Backends["work-claude"].Path {
		t.Fatalf("expected custom backend path %q, got %q", want.Backends["work-claude"].Path, got.Backends["work-claude"].Path)
	}
	if strings.Join(got.Backends["work-claude"].Args, " ") != strings.Join(want.Backends["work-claude"].Args, " ") {
		t.Fatalf("expected custom backend args %#v, got %#v", want.Backends["work-claude"].Args, got.Backends["work-claude"].Args)
	}
}

func TestApplyLegacyPathOverrideOverridesBinary(t *testing.T) {
	t.Parallel()

	cfg := backendConfig{
		binary: "codex",
		args:   []string{"exec"},
	}

	userCfg := userConfig{
		BackendPaths: map[string]string{
			"codex": "/custom/bin/codex",
		},
	}

	got := applyLegacyPathOverride(cfg, "codex", userCfg)
	if got.binary != "/custom/bin/codex" {
		t.Fatalf("expected configured binary path, got %q", got.binary)
	}
}

func TestResolveBackendUsesCustomBackendDefinition(t *testing.T) {
	t.Parallel()

	cfg, err := resolveBackend("work-claude", userConfig{
		Backends: map[string]configuredBackend{
			"work-claude": {
				Path: "/Users/you/bin/claude-wrapper",
				Args: []string{"-p", "--model", "sonnet"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.binary != "/Users/you/bin/claude-wrapper" {
		t.Fatalf("expected custom backend path, got %q", cfg.binary)
	}
	if cfg.mode != backendModeStreaming {
		t.Fatalf("expected streaming mode, got %q", cfg.mode)
	}
	if cfg.useTempDir {
		t.Fatal("expected custom backend to inherit the caller working directory")
	}
	if strings.Join(cfg.args, " ") != "-p --model sonnet" {
		t.Fatalf("expected custom args, got %#v", cfg.args)
	}
}

func TestResolveBackendOverridesBuiltinArgsAndPath(t *testing.T) {
	t.Parallel()

	cfg, err := resolveBackend("claude", userConfig{
		Backends: map[string]configuredBackend{
			"claude": {
				Path: "/Users/you/bin/claude",
				Args: []string{"-p", "--model", "opus"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.binary != "/Users/you/bin/claude" {
		t.Fatalf("expected overridden path, got %q", cfg.binary)
	}
	if !cfg.useTempDir {
		t.Fatal("expected built-in backend overrides to keep the temporary working directory behavior")
	}
	if strings.Join(cfg.args, " ") != "-p --model opus" {
		t.Fatalf("expected overridden args, got %#v", cfg.args)
	}
}

func TestPrepareBackendCommandUsesTempDirWhenConfigured(t *testing.T) {
	t.Parallel()

	cmd, cleanup, err := prepareBackendCommand(backendConfig{
		binary:     "claude",
		args:       []string{"-p"},
		useTempDir: true,
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

	cmd, cleanup, err := prepareBackendCommand(backendConfig{
		binary: "claude",
		args:   []string{"-p"},
	}, "what is tail recursion?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer cleanup()

	if cmd.Dir != "" {
		t.Fatalf("expected command to inherit the caller working directory, got %q", cmd.Dir)
	}
}

func TestResolveBackendRejectsCodexWithoutJSONArg(t *testing.T) {
	t.Parallel()

	_, err := resolveBackend("codex", userConfig{
		Backends: map[string]configuredBackend{
			"codex": {
				Args: []string{"exec", "--sandbox", "read-only"},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for codex backend without --json")
	}
	if !strings.Contains(err.Error(), "must include --json") {
		t.Fatalf("expected --json validation error, got %v", err)
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

func TestShouldShowSpinnerFalseForRegularFile(t *testing.T) {
	t.Parallel()

	file, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer file.Close()

	if shouldShowSpinner(file) {
		t.Fatal("expected spinner to be disabled for a regular file")
	}
}
