package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
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

func TestParseCommandTreatsUnknownPositionalAsExec(t *testing.T) {
	t.Parallel()

	command, err := parseCommand([]string{"abcdef"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if command.name != "exec" || command.prompt != "abcdef" {
		t.Fatalf("unexpected positional exec command: %+v", command)
	}
}

func TestStartAttachedAPIServerStartsAndPublishesDiscovery(t *testing.T) {
<<<<<<< HEAD
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	repoRoot := t.TempDir()
	app, err := newApp(repoRoot, "", orchestrator.BootOptions{})
=======
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	app, err := newApp(repoRoot, orchestrator.BootOptions{})
>>>>>>> cef7a8c (update)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	defer app.close()
	if app.api != nil {
		t.Fatal("did not expect api server to start automatically in newApp")
	}

	var stderr bytes.Buffer
	status, event := startAttachedAPIServer(app, &stderr)
	if app.api == nil {
		t.Fatal("expected attached api server to start")
	}
	if strings.TrimSpace(status) == "" || event.Type != "api_server_ready" {
		t.Fatalf("unexpected api startup outputs: status=%q event=%+v", status, event)
	}
	if _, err := os.Stat(filepath.Join(app.paths.APIDir, "current.json")); err != nil {
		t.Fatalf("expected current discovery file: %v", err)
	}
	if !strings.Contains(stderr.String(), "orch api: ready at http://127.0.0.1:") {
		t.Fatalf("unexpected stderr output: %q", stderr.String())
	}
}

func TestStartAttachedAPIServerFailureIsNonFatal(t *testing.T) {
<<<<<<< HEAD
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	repoRoot := t.TempDir()
	app, err := newApp(repoRoot, "", orchestrator.BootOptions{})
=======
	orchHome := filepath.Join(t.TempDir(), ".orch-home")
	t.Setenv("ORCH_HOME", orchHome)

	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir git dir: %v", err)
	}
	app, err := newApp(repoRoot, orchestrator.BootOptions{})
>>>>>>> cef7a8c (update)
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	defer app.close()

	badPath := filepath.Join(t.TempDir(), "api")
	if err := os.WriteFile(badPath, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("write bad api path: %v", err)
	}
	app.paths.APIDir = badPath

	var stderr bytes.Buffer
	status, event := startAttachedAPIServer(app, &stderr)
	if strings.TrimSpace(status) == "" {
		t.Fatal("expected non-fatal status message")
	}
	if event.Type != "" {
		t.Fatalf("did not expect ready event on failure: %+v", event)
	}
	if app.api != nil {
		t.Fatal("did not expect api server on failure")
	}
	if !strings.Contains(stderr.String(), "orch api:") {
		t.Fatalf("expected stderr output for api failure, got %q", stderr.String())
	}
}
