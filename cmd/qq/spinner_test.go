package main

import (
	"os"
	"testing"
)

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
