package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type backendMode string

const (
	backendModeStreaming backendMode = "streaming"
	backendModeCodexJSON backendMode = "codex_json"
)

type backendDefinition struct {
	Path       string      `json:"path"`
	Args       []string    `json:"args"`
	Mode       backendMode `json:"-"`
	UseTempDir bool        `json:"-"`
}

func builtinBackends() map[string]backendDefinition {
	return map[string]backendDefinition{
		"codex": {
			Path: "codex",
			Args: []string{
				"exec",
				"--ephemeral",
				"--skip-git-repo-check",
				"--json",
				"--sandbox",
				"read-only",
			},
			Mode:       backendModeCodexJSON,
			UseTempDir: true,
		},
		"claude": {
			Path:       "claude",
			Args:       []string{"-p"},
			Mode:       backendModeStreaming,
			UseTempDir: true,
		},
		"gemini": {
			Path:       "gemini",
			Args:       []string{"-p"},
			Mode:       backendModeStreaming,
			UseTempDir: true,
		},
	}
}

func resolveBackend(name string, userCfg userConfig) (backendDefinition, error) {
	normalizedName := normalizeBackendName(name)
	if normalizedName == "" {
		return backendDefinition{}, fmt.Errorf("backend name cannot be empty")
	}

	backends := builtinBackends()
	applyConfiguredBackends(backends, userCfg.Backends)

	backend, ok := backends[normalizedName]
	if !ok {
		return backendDefinition{}, fmt.Errorf("unsupported backend %q", name)
	}

	backend = applyLegacyPathOverride(backend, normalizedName, userCfg.BackendPaths)
	if strings.TrimSpace(backend.Path) == "" {
		return backendDefinition{}, fmt.Errorf("backend %q must define a path", name)
	}

	if backend.Mode == "" {
		backend.Mode = backendModeStreaming
	}
	if backend.Mode == backendModeCodexJSON && !hasArg(backend.Args, "--json") {
		return backendDefinition{}, fmt.Errorf("backend %q must include --json in args", name)
	}

	return backend, nil
}

func applyConfiguredBackends(backends map[string]backendDefinition, configured map[string]backendDefinition) {
	for name, override := range configured {
		normalizedName := normalizeBackendName(name)
		if normalizedName == "" {
			continue
		}

		backends[normalizedName] = mergeBackendDefinition(backends[normalizedName], override)
	}
}

func applyLegacyPathOverride(backend backendDefinition, backendName string, legacyPaths map[string]string) backendDefinition {
	if legacyPaths == nil {
		return backend
	}

	configuredPath := strings.TrimSpace(legacyPaths[normalizeBackendName(backendName)])
	if configuredPath == "" {
		return backend
	}

	backend.Path = configuredPath
	return backend
}

func mergeBackendDefinition(base backendDefinition, override backendDefinition) backendDefinition {
	if path := strings.TrimSpace(override.Path); path != "" {
		base.Path = path
	}
	if override.Args != nil {
		base.Args = append([]string(nil), override.Args...)
	}
	if override.Mode != "" {
		base.Mode = override.Mode
	}
	return base
}

func normalizeBackendName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}

	return false
}

func resolveBinaryPath(path string) (string, error) {
	if strings.ContainsRune(path, os.PathSeparator) {
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("%q is a directory", path)
		}
		return path, nil
	}

	return exec.LookPath(path)
}
