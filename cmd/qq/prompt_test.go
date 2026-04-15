package main

import (
	"fmt"
	"testing"
)

func TestWrapQuestionWithoutCurrentDirContext(t *testing.T) {
	t.Parallel()

	question := "what is tail recursion?"
	got := wrapQuestion(question, false)
	want := fmt.Sprintf(isolatedSystemPrompt, question)

	if got != want {
		t.Fatalf("expected wrapped prompt %q, got %q", want, got)
	}
}

func TestWrapQuestionWithCurrentDirContext(t *testing.T) {
	t.Parallel()

	question := "explain the files in this directory"
	got := wrapQuestion(question, true)
	want := fmt.Sprintf(currentDirContextSystemPrompt, question)

	if got != want {
		t.Fatalf("expected wrapped prompt %q, got %q", want, got)
	}
}
