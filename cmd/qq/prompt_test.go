package main

import (
	"fmt"
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
