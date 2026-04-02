package tooling

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestReviewGatewayReadDoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"read","path":"."}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.RequiresApproval {
		t.Fatal("did not expect approval for gateway read")
	}
}

func TestReviewWorkerWriteRequiresApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleWorker,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"write","path":"README.md","content":"hello"}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !got.RequiresApproval {
		t.Fatal("expected approval for worker write")
	}
}

func TestReviewRejectsGatewayWrite(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	_, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"write","path":"README.md","content":"hello"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "gateway role does not allow") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewPlanModeOnlyAllowsReadListSearch(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModePlan,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	for _, call := range []domain.ToolCall{
		{Name: "ot", Arguments: `{"op":"context"}`},
		{Name: "ot", Arguments: `{"op":"task_list"}`},
		{Name: "ot", Arguments: `{"op":"task_get","task_id":"task-1"}`},
		{Name: "ot", Arguments: `{"op":"read","path":"."}`},
		{Name: "ot", Arguments: `{"op":"list","path":"."}`},
		{Name: "ot", Arguments: `{"op":"search","path":".","name_pattern":"*.md"}`},
	} {
		if _, err := executor.Review(workspace, record, nil, domain.Settings{}, call); err != nil {
			t.Fatalf("review %s: %v", call.Arguments, err)
		}
	}

	if _, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"delegate","task_title":"x","task_contract":"y"}`,
	}); err == nil {
		t.Fatal("expected delegate to be rejected in plan mode")
	}
}

func TestExecuteContextTaskListAndTaskGetUseResolvers(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	executor.SetStateResolvers(
		func(record domain.RunRecord) (domain.ContextSnapshot, error) {
			return domain.ContextSnapshot{SessionID: record.SessionID, RunID: record.RunID, Provider: "ollama"}, nil
		},
		func(record domain.RunRecord, statusFilter string) ([]domain.TaskView, error) {
			return []domain.TaskView{{TaskID: "task-1", Status: statusFilter, ChildSessionID: record.SessionID}}, nil
		},
		func(record domain.RunRecord, taskID string) (domain.TaskView, error) {
			return domain.TaskView{TaskID: taskID, ChildSessionID: record.SessionID}, nil
		},
	)

	record := domain.RunRecord{
		RunID:         "R1",
		SessionID:     "S1",
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	contextExec, err := executor.Execute(context.Background(), workspace, record, nil, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"context"}`,
	})
	if err != nil {
		t.Fatalf("execute context: %v", err)
	}
	if !strings.Contains(contextExec.Output, "session_id: S1") {
		t.Fatalf("unexpected context output: %q", contextExec.Output)
	}

	listExec, err := executor.Execute(context.Background(), workspace, record, nil, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"task_list","status_filter":"running"}`,
	})
	if err != nil {
		t.Fatalf("execute task_list: %v", err)
	}
	if !strings.Contains(listExec.Output, `"task_id": "task-1"`) || !strings.Contains(listExec.Output, `"status": "running"`) {
		t.Fatalf("unexpected task list output: %q", listExec.Output)
	}

	getExec, err := executor.Execute(context.Background(), workspace, record, nil, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"task_get","task_id":"task-1"}`,
	})
	if err != nil {
		t.Fatalf("execute task_get: %v", err)
	}
	if !strings.Contains(getExec.Output, `"task_id": "task-1"`) {
		t.Fatalf("unexpected task get output: %q", getExec.Output)
	}
}

func TestExecuteReadRejectsOutsideWorkspacePath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	_, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"read","path":"../outside"}`,
	})
	if err == nil || !strings.Contains(err.Error(), "escapes workspace root") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteReadUsesExternalOTBinary(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "README.md")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	otDir := t.TempDir()
	otPath := filepath.Join(otDir, "ot")
	script := strings.Join([]string{
		"#!/usr/bin/env bash",
		"set -euo pipefail",
		"subcommand=\"$1\"",
		"shift",
		"case \"$subcommand\" in",
		"  read)",
		"    path=\"\"",
		"    while [[ $# -gt 0 ]]; do",
		"      case \"$1\" in",
		"        --path)",
		"          path=\"$2\"",
		"          shift 2",
		"          ;;",
		"        *)",
		"          shift",
		"          ;;",
		"      esac",
		"    done",
		"    cat \"$path\"",
		"    ;;",
		"  *)",
		"    exit 1",
		"    ;;",
		"esac",
		"",
	}, "\n")
	if err := os.WriteFile(otPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write ot stub: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + otDir + string(os.PathListSeparator) + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"read","path":"README.md"}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(got.Output) != "hello" {
		t.Fatalf("unexpected output: %q", got.Output)
	}
}

func TestResolveOTExecutableFromPrefersSiblingBinary(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	current := filepath.Join(dir, "orch")
	sibling := filepath.Join(dir, "ot")
	if err := os.WriteFile(sibling, []byte(""), 0o755); err != nil {
		t.Fatalf("write sibling: %v", err)
	}

	resolved, err := resolveOTExecutableFrom(current, func(string) (string, error) {
		return "", fmt.Errorf("unexpected lookup")
	})
	if err != nil {
		t.Fatalf("resolveOTExecutableFrom: %v", err)
	}
	if resolved != sibling {
		t.Fatalf("unexpected resolved path: %q", resolved)
	}
}

func TestResolveOTExecutableFromFallsBackToPath(t *testing.T) {
	t.Parallel()

	resolved, err := resolveOTExecutableFrom("/tmp/orch", func(name string) (string, error) {
		if name != "ot" {
			t.Fatalf("unexpected lookup for %q", name)
		}
		return "/usr/local/bin/ot", nil
	})
	if err != nil {
		t.Fatalf("resolveOTExecutableFrom: %v", err)
	}
	if resolved != "/usr/local/bin/ot" {
		t.Fatalf("unexpected resolved path: %q", resolved)
	}
}

func TestExecuteWorkerCompleteSetsTerminalStatus(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleWorker,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	got, err := executor.Execute(context.Background(), workspace, record, nil, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"complete","message":"done"}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.TerminalStatus != domain.StatusCompleted {
		t.Fatalf("unexpected terminal status: %+v", got)
	}
}

func TestExecuteWorkerCompleteCarriesStructuredTaskOutcome(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleWorker,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	got, err := executor.Execute(context.Background(), workspace, record, nil, domain.ToolCall{
		Name:      "ot",
		Arguments: `{"op":"complete","summary":"done","changed_paths":["README.md"],"checks_run":["go_test"],"evidence_pointers":["ot-pointer://current?lines=1"],"followups":["run integration tests"]}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got.TerminalStatus != domain.StatusCompleted {
		t.Fatalf("unexpected terminal status: %+v", got)
	}
	if got.TaskSummary != "done" || len(got.TaskChangedPaths) != 1 || got.TaskChangedPaths[0] != "README.md" {
		t.Fatalf("unexpected task outcome: %+v", got)
	}
	if !strings.Contains(got.Output, "changed_paths: README.md") {
		t.Fatalf("unexpected complete output: %q", got.Output)
	}
}
