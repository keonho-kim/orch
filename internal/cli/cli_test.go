package cli

import (
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestParseCommandDefaultsToTUI(t *testing.T) {
	t.Parallel()

	command, err := parseCommand(nil)
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "interactive" {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.repoRoot != "." {
		t.Fatalf("unexpected default repo root: %+v", command)
	}
}

func TestParseCommandInteractiveWorkspaceFlag(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"--workspace", "/repo"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "interactive" || command.repoRoot != "/repo" {
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

func TestParseCommandExecWorkspaceFlag(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"exec", "--workspace", "/repo", "ship", "it"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.repoRoot != "/repo" || command.prompt != "ship it" {
		t.Fatalf("unexpected command: %+v", command)
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

func TestParseCommandHistoryLatest(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"history", "--latest", "--workspace", "."})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !command.restoreLatest || command.repoRoot != "." {
		t.Fatalf("unexpected history latest command: %+v", command)
	}
}

func TestParseCommandHistoryRestoreSession(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"history", "20260313-120000.000", "--workspace", "."})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.restoreSessionID != "20260313-120000.000" || command.repoRoot != "." {
		t.Fatalf("unexpected history restore command: %+v", command)
	}
}

func TestParseCommandSubagentRun(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"__subagent-run", "/repo", "S1", "R2", `{"id":"task-1","title":"Inspect","contract":"inspect failing tests"}`})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "__subagent-run" {
		t.Fatalf("unexpected command: %+v", command)
	}
	if command.repoRoot != "/repo" || command.parentSessionID != "S1" || command.parentRunID != "R2" {
		t.Fatalf("unexpected hidden subagent command: %+v", command)
	}
	if command.subagentTask == "" {
		t.Fatalf("expected encoded task payload")
	}
}

func TestBuildSubagentResultTruncatesFailedOutput(t *testing.T) {
	t.Parallel()

	record := domain.RunRecord{
		RunID:       "R9",
		Status:      domain.StatusFailed,
		CurrentTask: "Failed",
		FinalOutput: strings.Repeat("x", 12010),
	}

	result := buildSubagentResult("S9", domain.SubagentTask{ID: "task-9", Title: "Inspect"}, domain.SessionMetadata{
		TaskStatus: "failed",
		WorkerRole: domain.AgentRoleWorker,
	}, record)
	if result.ChildSessionID != "S9" || result.ChildRunID != "R9" {
		t.Fatalf("unexpected result identity: %+v", result)
	}
	if !result.Truncated {
		t.Fatal("expected output to be truncated")
	}
	if result.Error == "" {
		t.Fatal("expected failed result to expose an error")
	}
}

func TestParseCommandRejectsUnknownPositionalCommand(t *testing.T) {
	t.Parallel()

	if _, err := parseCommand([]string{"abcdef"}); err == nil {
		t.Fatal("expected unknown positional command to be rejected")
	}
}
