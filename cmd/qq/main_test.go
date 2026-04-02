package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildBackendCodexIncludesJSONFlag(t *testing.T) {
	t.Parallel()

	cfg, err := buildBackend("codex")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
}

func TestApplyConfigToBackendOverridesBinary(t *testing.T) {
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

	got := applyConfigToBackend(cfg, "codex", userCfg)
	if got.binary != "/custom/bin/codex" {
		t.Fatalf("expected configured binary path, got %q", got.binary)
	}
}
