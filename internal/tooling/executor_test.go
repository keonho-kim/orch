package tooling

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orch/domain"
)

func TestReviewOtReadDoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: t.TempDir(), CurrentCwd: t.TempDir()}
	got, err := executor.Review(record.WorkspacePath, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["read","--path","README.md"]}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.RequiresApproval {
		t.Fatalf("expected ot read to be auto-allowed")
	}
}

func TestReviewOtWriteRequiresApproval(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: t.TempDir(), CurrentCwd: t.TempDir()}
	got, err := executor.Review(record.WorkspacePath, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["write","--path","README.md","--from-stdin"]}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !got.RequiresApproval {
		t.Fatalf("expected ot write to require approval")
	}
}

func TestReviewRejectsShellLikeCommandStrings(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: t.TempDir(), CurrentCwd: t.TempDir()}
	_, err := executor.Review(record.WorkspacePath, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot read --path .","args":[]}`,
	})
	if err == nil {
		t.Fatal("expected shell-like command string to be rejected")
	}
	if !strings.Contains(err.Error(), "bare executable name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReviewSelfDrivingAutoAllowsExceptRmAndMv(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	settings := domain.Settings{SelfDrivingMode: true}
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: t.TempDir(), CurrentCwd: t.TempDir()}

	got, err := executor.Review(record.WorkspacePath, record, settings, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"python","args":["-V"]}`,
	})
	if err != nil {
		t.Fatalf("review python: %v", err)
	}
	if got.RequiresApproval {
		t.Fatalf("expected self-driving to auto-allow python")
	}

	got, err = executor.Review(record.WorkspacePath, record, settings, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["exec","rm","-rf","tmp"]}`,
	})
	if err != nil {
		t.Fatalf("review rm: %v", err)
	}
	if !got.RequiresApproval {
		t.Fatalf("expected rm to keep requiring approval")
	}
}

func TestReviewDirectRMRequiresApproval(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: t.TempDir(), CurrentCwd: t.TempDir()}
	got, err := executor.Review(record.WorkspacePath, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"rm","args":["-f","README.md"]}`,
	})
	if err != nil {
		t.Fatalf("review rm: %v", err)
	}
	if !got.RequiresApproval {
		t.Fatalf("expected direct rm to require approval")
	}
}

func TestExecuteOtReadRunsWorkspaceScript(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "tools", "ot"), 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "tools", "ot", "read.sh"), []byte("#!/usr/bin/env bash\ncat \"$OT_WORKSPACE_ROOT/$2\"\n"), 0o755); err != nil {
		t.Fatalf("write read script: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["read","--path","README.md"]}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(got.Output) != "hello" {
		t.Fatalf("unexpected output: %q", got.Output)
	}
}

func TestExecuteOtReadListsDirectoryEntries(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "tools", "ot"), 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	readScript := `#!/usr/bin/env bash
set -euo pipefail
target="$OT_WORKSPACE_ROOT/$2"
if [[ -d "$target" ]]; then
  find "$target" -mindepth 1 -maxdepth 1 -print | sed "s#^$OT_WORKSPACE_ROOT/##"
  exit 0
fi
cat "$target"
`
	if err := os.WriteFile(filepath.Join(workspace, "tools", "ot", "read.sh"), []byte(readScript), 0o755); err != nil {
		t.Fatalf("write read script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "docs", "guide.md"), []byte("guide"), 0o644); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["read","--path","."]}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(got.Output, "README.md") || !strings.Contains(got.Output, "docs") {
		t.Fatalf("expected directory listing output, got %q", got.Output)
	}
}

func TestExecuteOtReadRejectsRangesForDirectories(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "tools", "ot"), 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "tools", "ot", "read.sh"), []byte("#!/usr/bin/env bash\nexit 99\n"), 0o755); err != nil {
		t.Fatalf("write read script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	_, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["read","--path","docs","--start","1","--end","5"]}`,
	})
	if err == nil {
		t.Fatal("expected directory line-range read to fail")
	}
	if !strings.Contains(err.Error(), "only supported for files") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteOtWriteRejectsDirectoryTargets(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "tools", "ot"), 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "tools", "ot", "write.sh"), []byte("#!/usr/bin/env bash\nexit 99\n"), 0o755); err != nil {
		t.Fatalf("write write script: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	_, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["write","--path","docs","--from-stdin"],"stdin":"hello"}`,
	})
	if err == nil {
		t.Fatal("expected directory write to fail")
	}
	if !strings.Contains(err.Error(), "requires a file path, not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteDirectRMAllowedAfterApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "remove-me.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	_, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"rm","args":["-f","remove-me.txt"]}`,
	})
	if err != nil {
		t.Fatalf("execute rm: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed, stat err=%v", err)
	}
}

func TestPlanModeAllowsCDAndReadOnly(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{Mode: domain.RunModePlan, WorkspacePath: workspace, CurrentCwd: workspace}
	executor := NewExecutor()

	review, err := executor.Review(workspace, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"cd","args":["."]}`,
	})
	if err != nil {
		t.Fatalf("review cd: %v", err)
	}
	if review.RequiresApproval {
		t.Fatal("expected plan mode cd to be auto-allowed")
	}

	review, err = executor.Review(workspace, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["read","--path","."]}`,
	})
	if err != nil {
		t.Fatalf("review read dir: %v", err)
	}
	if review.RequiresApproval {
		t.Fatal("expected plan mode ot read to be auto-allowed")
	}

	_, err = executor.Review(workspace, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"python","args":["-V"]}`,
	})
	if err == nil {
		t.Fatal("expected plan mode to reject python")
	}

	_, err = executor.Review(workspace, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"rg","args":["TODO","."]}`,
	})
	if err == nil {
		t.Fatal("expected plan mode to reject rg")
	}

	_, err = executor.Review(workspace, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"find","args":[".","-maxdepth","1"]}`,
	})
	if err == nil {
		t.Fatal("expected plan mode to reject find")
	}
}

func TestReviewReactModeAllowsRGAndFind(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: t.TempDir(), CurrentCwd: t.TempDir()}

	review, err := executor.Review(record.WorkspacePath, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"rg","args":["TODO","."]}`,
	})
	if err != nil {
		t.Fatalf("review rg: %v", err)
	}
	if !review.RequiresApproval {
		t.Fatal("expected rg to be allowed but still require approval in react mode")
	}

	review, err = executor.Review(record.WorkspacePath, record, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"find","args":[".","-maxdepth","1"]}`,
	})
	if err != nil {
		t.Fatalf("review find: %v", err)
	}
	if !review.RequiresApproval {
		t.Fatal("expected find to be allowed but still require approval in react mode")
	}
}

func TestExecuteOtRejectsRemovedSubcommands(t *testing.T) {
	t.Parallel()

	executor := NewExecutor()
	workspace := t.TempDir()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}

	_, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["list","--path","."]}`,
	})
	if err == nil {
		t.Fatal("expected removed ot list subcommand to fail")
	}
	if !strings.Contains(err.Error(), "ot list is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}
