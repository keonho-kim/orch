package tooling

import (
	"context"
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

func TestExecuteReadUsesOTRunner(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "README.md")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	toolsDir := filepath.Join(workspace, "tools", "ot")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	for scriptPath := range map[string]string{
		filepath.Join(toolsDir, "read.sh"):   "",
		filepath.Join(toolsDir, "list.sh"):   "",
		filepath.Join(toolsDir, "search.sh"): "",
		filepath.Join(toolsDir, "write.sh"):  "",
		filepath.Join(toolsDir, "patch.sh"):  "",
	} {
		if err := os.WriteFile(scriptPath, []byte(strings.TrimSpace(fixtureScriptContent(scriptPath))), 0o755); err != nil {
			t.Fatalf("write tool %s: %v", scriptPath, err)
		}
	}

	executor := NewExecutor()
	record := domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		WorkspacePath: workspace,
		CurrentCwd:    workspace,
	}

	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
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

func fixtureScriptContent(path string) string {
	switch filepath.Base(path) {
	case "read.sh":
		return "#!/usr/bin/env bash\nset -euo pipefail\ntarget=\"\"\nwhile [[ $# -gt 0 ]]; do\n  case \"$1\" in\n    --target)\n      target=\"$2\"\n      shift 2\n      ;;\n    *)\n      shift\n      ;;\n  esac\ndone\ncat \"$target\"\n"
	case "list.sh", "search.sh", "write.sh", "patch.sh":
		return "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"
	default:
		return "#!/usr/bin/env bash\nset -euo pipefail\nexit 0\n"
	}
}
