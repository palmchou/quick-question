package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type userConfig struct {
	DefaultBackend string                       `json:"default_backend"`
	BackendPaths   map[string]string            `json:"backend_paths"`
	Backends       map[string]backendDefinition `json:"backends"`
}

func defaultConfigPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(configDir, "qq", "config.json")
}

func loadUserConfig(path string) (userConfig, error) {
	if path == "" {
		return userConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return userConfig{}, nil
		}
		return userConfig{}, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var cfg userConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return userConfig{}, fmt.Errorf("failed to parse config file %q: %w", path, err)
	}

	return cfg, nil
}
