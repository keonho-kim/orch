package tooling

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/session"
)

func TestReviewOtReadInsideWorkspaceDoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, SessionID: "S1", WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["read","--path","README.md"]}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.RequiresApproval {
		t.Fatal("expected workspace ot read to be auto-allowed")
	}
}

func TestReviewOtOutsideWorkspaceRequiresApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := filepath.Join(filepath.Dir(workspace), "outside-review.txt")
	if err := os.WriteFile(outside, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	defer os.Remove(outside)

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, SessionID: "S1", WorkspacePath: workspace, CurrentCwd: workspace}

	for _, args := range [][]string{
		{"read", "--path", outside},
		{"list", "--path", outside},
		{"search", "--path", outside, "--content", "hello"},
	} {
		got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
			Name:      "exec",
			Arguments: toExecArgs("ot", args...),
		})
		if err != nil {
			t.Fatalf("review %v: %v", args, err)
		}
		if !got.RequiresApproval {
			t.Fatalf("expected approval for %v", args)
		}
	}
}

func TestReviewOtWriteRequiresApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, SessionID: "S1", WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
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

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	_, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
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

func TestReviewSelfDrivingStillRequiresApprovalForExternalOTRead(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outside := filepath.Join(filepath.Dir(workspace), "self-driving-outside.txt")
	if err := os.WriteFile(outside, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	defer os.Remove(outside)

	executor := NewExecutor()
	settings := domain.Settings{SelfDrivingMode: true}
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}

	got, err := executor.Review(workspace, record, nil, settings, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "read", "--path", outside),
	})
	if err != nil {
		t.Fatalf("review external ot read: %v", err)
	}
	if !got.RequiresApproval {
		t.Fatal("expected external ot read to require approval in self-driving mode")
	}

	got, err = executor.Review(workspace, record, nil, settings, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"python","args":["-V"]}`,
	})
	if err != nil {
		t.Fatalf("review python: %v", err)
	}
	if got.RequiresApproval {
		t.Fatal("expected self-driving to auto-allow python")
	}
}

func TestReviewDirectRMRequiresApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
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

func TestReviewOtSubagentRequiresApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}

	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["subagent","--prompt","inspect the issue"]}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if !got.RequiresApproval {
		t.Fatal("expected ot subagent to require approval")
	}
}

func TestReviewOtSubagentAutoAllowedInSelfDrivingMode(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}

	got, err := executor.Review(workspace, record, nil, domain.Settings{SelfDrivingMode: true}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["subagent","--prompt","inspect the issue"]}`,
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.RequiresApproval {
		t.Fatal("expected self-driving mode to auto-allow ot subagent")
	}
}

func TestReviewOtSubagentRejectsNestedRuns(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}

	_, err := executor.Review(workspace, record, []string{"ORCH_SUBAGENT_DEPTH=1"}, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["subagent","--prompt","inspect the issue"]}`,
	})
	if err == nil {
		t.Fatal("expected nested ot subagent run to fail")
	}
}

func TestReviewOtPointerDoesNotRequireApproval(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	pointer := session.FormatOTPointer([]int64{1})

	got, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "pointer", "--value", pointer),
	})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.RequiresApproval {
		t.Fatal("expected ot pointer to be auto-allowed")
	}
}

func TestExecuteOtReadRunsWorkspaceScript(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
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

func TestExecuteOtListShowsHiddenFilesInsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	if err := os.WriteFile(filepath.Join(workspace, ".secret"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write visible file: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["list","--path","."]}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(got.Output, ".secret") {
		t.Fatalf("expected hidden file in listing, got %q", got.Output)
	}
	if !strings.Contains(got.Output, "visible.txt") {
		t.Fatalf("expected visible file in listing, got %q", got.Output)
	}
}

func TestExecuteOtPointerReadsSessionJSONLLines(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	workspace := filepath.Join(repoRoot, "test-workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "runtime-asset", "bootstrap"), 0o755); err != nil {
		t.Fatalf("mkdir bootstrap: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "runtime-asset", "bootstrap", "AGENTS.md"), []byte("agents\n"), 0o644); err != nil {
		t.Fatalf("write bootstrap agents: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "orch.settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	sessionsDir := filepath.Join(repoRoot, ".orch", "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := filepath.Join(sessionsDir, "S1.jsonl")
	if err := os.WriteFile(sessionPath, []byte("{\"line\":1}\n{\"line\":2}\n"), 0o644); err != nil {
		t.Fatalf("write session jsonl: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, SessionID: "S1", WorkspacePath: workspace, CurrentCwd: workspace}
	pointer := session.FormatOTPointer([]int64{2})
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "pointer", "--value", pointer),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(got.Output) != `2:{"line":2}` {
		t.Fatalf("unexpected pointer output: %q", got.Output)
	}
}

func TestExecuteOtListHidesHiddenFilesOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	outsideDir := filepath.Join(filepath.Dir(workspace), "ot-list-outside")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("mkdir outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)
	if err := os.WriteFile(filepath.Join(outsideDir, ".secret"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write visible: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "list", "--path", outsideDir),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(got.Output, ".secret") {
		t.Fatalf("did not expect hidden file in outside listing, got %q", got.Output)
	}
	if !strings.Contains(got.Output, filepath.Join(outsideDir, "visible.txt")) {
		t.Fatalf("expected absolute visible path in listing, got %q", got.Output)
	}
}

func TestExecuteOtSearchFindsHiddenContentInsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	if err := os.MkdirAll(filepath.Join(workspace, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".hidden", "notes.txt"), []byte("needle inside"), 0o644); err != nil {
		t.Fatalf("write hidden note: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["search","--path",".","--content","needle"]}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(got.Output, ".hidden/notes.txt:1:needle inside") {
		t.Fatalf("expected hidden workspace content match, got %q", got.Output)
	}
}

func TestExecuteOtSearchSkipsHiddenOutsideWorkspace(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	outsideDir := filepath.Join(filepath.Dir(workspace), "ot-search-outside")
	if err := os.MkdirAll(filepath.Join(outsideDir, ".hidden"), 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, ".hidden", "notes.txt"), []byte("needle hidden"), 0o644); err != nil {
		t.Fatalf("write hidden note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "visible.txt"), []byte("needle visible"), 0o644); err != nil {
		t.Fatalf("write visible note: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "search", "--path", outsideDir, "--content", "needle"),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(got.Output, ".hidden") {
		t.Fatalf("did not expect hidden outside match, got %q", got.Output)
	}
	if !strings.Contains(got.Output, filepath.Join(outsideDir, "visible.txt")+":1:needle visible") {
		t.Fatalf("expected visible outside match, got %q", got.Output)
	}
}

func TestExecuteOtSearchRejectsHiddenOutsideTarget(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	hiddenOutsideDir := filepath.Join(filepath.Dir(workspace), ".ot-search-hidden")
	if err := os.MkdirAll(hiddenOutsideDir, 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	defer os.RemoveAll(hiddenOutsideDir)

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	_, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "search", "--path", hiddenOutsideDir, "--name", "*.txt"),
	})
	if err == nil {
		t.Fatal("expected hidden outside target to fail")
	}
	if !strings.Contains(err.Error(), "hidden") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteOtSearchCombinesNameAndContentFilters(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	if err := os.WriteFile(filepath.Join(workspace, "match.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write match file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "match.log"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "other.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("write other file: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"ot","args":["search","--path",".","--name","*.txt","--content","needle"]}`,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(got.Output, "match.txt:1:needle") {
		t.Fatalf("expected txt content match, got %q", got.Output)
	}
	if strings.Contains(got.Output, "match.log") || strings.Contains(got.Output, "other.txt") {
		t.Fatalf("did not expect non-matching files, got %q", got.Output)
	}
}

func TestExecuteOtReadExternalDirectoryHidesHiddenEntries(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
	outsideDir := filepath.Join(filepath.Dir(workspace), "ot-read-outside")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("mkdir outside dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)
	if err := os.WriteFile(filepath.Join(outsideDir, ".secret"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "visible.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write visible: %v", err)
	}

	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}
	got, err := executor.Execute(context.Background(), workspace, record, []string{"PATH=" + os.Getenv("PATH")}, domain.ToolCall{
		Name:      "exec",
		Arguments: toExecArgs("ot", "read", "--path", outsideDir),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(got.Output, ".secret") {
		t.Fatalf("did not expect hidden entry in external read, got %q", got.Output)
	}
	if !strings.Contains(got.Output, filepath.Join(outsideDir, "visible.txt")) {
		t.Fatalf("expected visible absolute path in external read, got %q", got.Output)
	}
}

func TestExecuteOtWriteRejectsDirectoryTargets(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	copyRepoOTScripts(t, workspace)
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

func TestPlanModeAllowsCDAndOTInspectionCommands(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	record := domain.RunRecord{Mode: domain.RunModePlan, WorkspacePath: workspace, CurrentCwd: workspace}
	executor := NewExecutor()

	for _, call := range []domain.ToolCall{
		{Name: "exec", Arguments: `{"command":"cd","args":["."]}`},
		{Name: "exec", Arguments: `{"command":"ot","args":["read","--path","."]}`},
		{Name: "exec", Arguments: `{"command":"ot","args":["list","--path","."]}`},
		{Name: "exec", Arguments: `{"command":"ot","args":["search","--path",".","--name","*.md"]}`},
	} {
		review, err := executor.Review(workspace, record, nil, domain.Settings{}, call)
		if err != nil {
			t.Fatalf("review %s: %v", call.Arguments, err)
		}
		if review.RequiresApproval {
			t.Fatalf("expected plan mode command to be auto-allowed: %s", call.Arguments)
		}
	}

	for _, call := range []domain.ToolCall{
		{Name: "exec", Arguments: `{"command":"python","args":["-V"]}`},
		{Name: "exec", Arguments: `{"command":"rg","args":["TODO","."]}`},
		{Name: "exec", Arguments: `{"command":"find","args":[".","-maxdepth","1"]}`},
		{Name: "exec", Arguments: `{"command":"ot","args":["write","--path","README.md","--from-stdin"],"stdin":"x"}`},
	} {
		if _, err := executor.Review(workspace, record, nil, domain.Settings{}, call); err == nil {
			t.Fatalf("expected plan mode to reject %s", call.Arguments)
		}
	}
}

func TestReviewReactModeAllowsRGAndFind(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	executor := NewExecutor()
	record := domain.RunRecord{Mode: domain.RunModeReact, WorkspacePath: workspace, CurrentCwd: workspace}

	review, err := executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
		Name:      "exec",
		Arguments: `{"command":"rg","args":["TODO","."]}`,
	})
	if err != nil {
		t.Fatalf("review rg: %v", err)
	}
	if !review.RequiresApproval {
		t.Fatal("expected rg to be allowed but still require approval in react mode")
	}

	review, err = executor.Review(workspace, record, nil, domain.Settings{}, domain.ToolCall{
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

func toExecArgs(command string, args ...string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, `"`+arg+`"`)
	}
	return `{"command":"` + command + `","args":[` + strings.Join(quoted, ",") + `]}`
}
