package main

import (
	"testing"

	"orch/domain"
)

func TestParseCommandDefaultsToTUI(t *testing.T) {
	t.Parallel()

	command, err := parseCommand(nil)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "tui" {
		t.Fatalf("unexpected command: %+v", command)
	}
}

func TestParseCommandExec(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"exec", "ship", "it"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "exec" || command.prompt != "ship it" {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.mode != domain.RunModeReact {
		t.Fatalf("unexpected default mode: %+v", command)
	}
}

func TestParseCommandExecPlanMode(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"exec", "--mode", "plan", "draft", "it"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.mode != domain.RunModePlan || command.prompt != "draft it" {
		t.Fatalf("unexpected command: %+v", command)
	}
}

func TestParseCommandRejectsMissingExecPrompt(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"exec"}); err == nil {
		t.Fatal("expected error for missing exec prompt")
	}
}
