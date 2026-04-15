package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuiltinBackendsIncludeCodexJSONFlag(t *testing.T) {
	t.Parallel()

	cfg, ok := builtinBackends()["codex"]
	if !ok {
		t.Fatal("expected codex builtin backend")
	}

	if cfg.Path != "codex" {
		t.Fatalf("expected codex path, got %q", cfg.Path)
	}
	if cfg.Mode != backendModeCodexJSON {
		t.Fatalf("expected codex json mode, got %q", cfg.Mode)
	}
	if !hasArg(cfg.Args, "--json") {
		t.Fatal("expected codex backend args to include --json")
	}
	if !cfg.UseTempDir {
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
		Backends: map[string]backendDefinition{
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

func TestApplyLegacyPathOverrideOverridesPath(t *testing.T) {
	t.Parallel()

	cfg := backendDefinition{
		Path: "codex",
		Args: []string{"exec"},
	}

	got := applyLegacyPathOverride(cfg, "codex", map[string]string{
		"codex": "/custom/bin/codex",
	})
	if got.Path != "/custom/bin/codex" {
		t.Fatalf("expected configured path, got %q", got.Path)
	}
}

func TestResolveBackendUsesBuiltinDefinition(t *testing.T) {
	t.Parallel()

	cfg, err := resolveBackend("claude", userConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Path != "claude" {
		t.Fatalf("expected builtin path, got %q", cfg.Path)
	}
	if strings.Join(cfg.Args, " ") != "-p" {
		t.Fatalf("expected builtin args, got %#v", cfg.Args)
	}
	if cfg.Mode != backendModeStreaming {
		t.Fatalf("expected streaming mode, got %q", cfg.Mode)
	}
}

func TestResolveBackendUsesCustomBackendDefinition(t *testing.T) {
	t.Parallel()

	cfg, err := resolveBackend("work-claude", userConfig{
		Backends: map[string]backendDefinition{
			"work-claude": {
				Path: "/Users/you/bin/claude-wrapper",
				Args: []string{"-p", "--model", "sonnet"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Path != "/Users/you/bin/claude-wrapper" {
		t.Fatalf("expected custom backend path, got %q", cfg.Path)
	}
	if cfg.Mode != backendModeStreaming {
		t.Fatalf("expected streaming mode, got %q", cfg.Mode)
	}
	if cfg.UseTempDir {
		t.Fatal("expected custom backend to inherit the caller working directory")
	}
	if strings.Join(cfg.Args, " ") != "-p --model sonnet" {
		t.Fatalf("expected custom args, got %#v", cfg.Args)
	}
}

func TestResolveBackendOverridesBuiltinArgsAndPath(t *testing.T) {
	t.Parallel()

	cfg, err := resolveBackend("claude", userConfig{
		Backends: map[string]backendDefinition{
			"claude": {
				Path: "/Users/you/bin/claude",
				Args: []string{"-p", "--model", "opus"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Path != "/Users/you/bin/claude" {
		t.Fatalf("expected overridden path, got %q", cfg.Path)
	}
	if !cfg.UseTempDir {
		t.Fatal("expected built-in backend overrides to keep the temporary working directory behavior")
	}
	if strings.Join(cfg.Args, " ") != "-p --model opus" {
		t.Fatalf("expected overridden args, got %#v", cfg.Args)
	}
}

func TestApplyCurrentDirContextDisablesTempDir(t *testing.T) {
	t.Parallel()

	got := applyCurrentDirContext(backendDefinition{
		Path:       "claude",
		Args:       []string{"-p"},
		UseTempDir: true,
	}, true)

	if got.UseTempDir {
		t.Fatal("expected current-dir context to disable the temporary working directory")
	}
}

func TestApplyCurrentDirContextLeavesBackendUnchangedWhenDisabled(t *testing.T) {
	t.Parallel()

	want := backendDefinition{
		Path:       "claude",
		Args:       []string{"-p"},
		Mode:       backendModeStreaming,
		UseTempDir: true,
	}

	got := applyCurrentDirContext(want, false)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected backend to remain unchanged, got %#v", got)
	}
}

func TestResolveBackendRejectsCodexWithoutJSONArg(t *testing.T) {
	t.Parallel()

	_, err := resolveBackend("codex", userConfig{
		Backends: map[string]backendDefinition{
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
