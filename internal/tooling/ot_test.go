package tooling

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func copyRepoOTScripts(t *testing.T, workspace string) {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}

	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	sourceRoot := filepath.Join(repoRoot, "tools", "ot")
	targetRoot := filepath.Join(workspace, "tools", "ot")

	if err := os.MkdirAll(targetRoot, 0o755); err != nil {
		t.Fatalf("mkdir tools root: %v", err)
	}

	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		t.Fatalf("read source tools: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		sourcePath := filepath.Join(sourceRoot, entry.Name())
		data, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read source script %s: %v", entry.Name(), err)
		}

		info, err := os.Stat(sourcePath)
		if err != nil {
			t.Fatalf("stat source script %s: %v", entry.Name(), err)
		}

		targetPath := filepath.Join(targetRoot, entry.Name())
		if err := os.WriteFile(targetPath, data, info.Mode()); err != nil {
			t.Fatalf("write target script %s: %v", entry.Name(), err)
		}
	}
}

func TestInspectOTReadInsideWorkspaceNormalizesRelativeDisplay(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	target := filepath.Join(workspace, "README.md")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}
	inspection, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"read", "--path", "README.md"},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if !inspection.WithinWorkspace {
		t.Fatal("expected workspace target to stay inside workspace")
	}
	if inspection.Subcommand != "read" {
		t.Fatalf("unexpected subcommand: %s", inspection.Subcommand)
	}
	if got, want := inspection.NormalizedArgs[1], target; got != want {
		t.Fatalf("unexpected target: got %q want %q", got, want)
	}
	if got := inspection.NormalizedArgs[3]; got != otScopeInside {
		t.Fatalf("unexpected scope: %q", got)
	}
	if got := inspection.NormalizedArgs[5]; got != otDisplayRelative {
		t.Fatalf("unexpected display mode: %q", got)
	}
}

func TestInspectOTReadOutsideWorkspaceUsesAbsoluteDisplay(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	parent := filepath.Dir(workspace)
	target := filepath.Join(parent, "outside-readme.txt")
	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	defer os.Remove(target)

	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}
	inspection, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"read", "--path", target},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if inspection.WithinWorkspace {
		t.Fatal("expected outside target to be marked outside workspace")
	}
	if got := inspection.NormalizedArgs[3]; got != otScopeOutside {
		t.Fatalf("unexpected scope: %q", got)
	}
	if got := inspection.NormalizedArgs[5]; got != otDisplayAbsolute {
		t.Fatalf("unexpected display mode: %q", got)
	}
}

func TestInspectOTSearchRequiresFilter(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}

	_, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"search", "--path", "."},
	})
	if err == nil {
		t.Fatal("expected missing search filter to fail")
	}
}

func TestInspectOTRejectsHiddenOutsidePath(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	outsideDir := filepath.Join(filepath.Dir(workspace), ".outside-hidden")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	defer os.RemoveAll(outsideDir)

	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}
	_, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"list", "--path", outsideDir},
	})
	if err == nil {
		t.Fatal("expected hidden outside path to fail")
	}
	if got := err.Error(); got == "" || !containsAll(got, "hidden", "outside") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectOTListDefaultsToCurrentDirectory(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}

	inspection, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"list"},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if got, want := inspection.NormalizedArgs[1], workspace; got != want {
		t.Fatalf("unexpected default target: got %q want %q", got, want)
	}
}

func TestInspectOTSubagentRequiresPrompt(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}

	_, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"subagent"},
	})
	if err == nil {
		t.Fatal("expected missing subagent prompt to fail")
	}
}

func TestInspectOTSubagentCapturesPrompt(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}

	inspection, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"subagent", "--prompt", "investigate failing tests"},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspection.Subcommand != "subagent" {
		t.Fatalf("unexpected subcommand: %s", inspection.Subcommand)
	}
	if inspection.Prompt != "investigate failing tests" {
		t.Fatalf("unexpected prompt: %q", inspection.Prompt)
	}
}

func TestInspectOTPointerRequiresValue(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}

	_, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"pointer"},
	})
	if err == nil {
		t.Fatal("expected missing pointer value to fail")
	}
}

func TestInspectOTPointerCapturesValue(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	record := domain.RunRecord{WorkspacePath: workspace, CurrentCwd: workspace}

	inspection, err := inspectOTRequest(workspace, record, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"pointer", "--value", "ot-pointer://current?lines=1"},
	})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if inspection.Subcommand != "pointer" {
		t.Fatalf("unexpected subcommand: %s", inspection.Subcommand)
	}
	if !strings.Contains(inspection.Prompt, "ot-pointer://current") {
		t.Fatalf("unexpected pointer value: %q", inspection.Prompt)
	}
}

func TestResolveSubagentRepoRootDoesNotRequireProjectSettingsFile(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	bootstrapDir := filepath.Join(repoRoot, "runtime-asset", "bootstrap")
	if err := os.MkdirAll(bootstrapDir, 0o755); err != nil {
		t.Fatalf("create bootstrap dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bootstrapDir, "AGENTS.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	workspace := filepath.Join(repoRoot, "test-workspace", "nested")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	resolved, err := resolveSubagentRepoRoot(workspace)
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if resolved != repoRoot {
		t.Fatalf("unexpected repo root: got %q want %q", resolved, repoRoot)
	}
}

func containsAll(value string, expected ...string) bool {
	for _, item := range expected {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
