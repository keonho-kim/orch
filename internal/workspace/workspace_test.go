package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeEnvPreservesExpectedKeys(t *testing.T) {
	t.Parallel()

	env := sanitizeEnv([]string{
		"PATH=/usr/bin",
		"HOME=/Users/test",
		"CUSTOM_OPENAI_KEY=abc",
		"ORCH_SUBAGENT_DEPTH=1",
		"SECRET_TOKEN=skip",
	}, []string{"CUSTOM_OPENAI_KEY"})

	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "PATH=/usr/bin") {
		t.Fatalf("expected PATH to be preserved")
	}
	if !strings.Contains(joined, "CUSTOM_OPENAI_KEY=abc") {
		t.Fatalf("expected configured api key env to be preserved")
	}
	if !strings.Contains(joined, "ORCH_SUBAGENT_DEPTH=1") {
		t.Fatalf("expected subagent depth env to be preserved")
	}
	if strings.Contains(joined, "SECRET_TOKEN=skip") {
		t.Fatalf("did not expect unrelated secret to be preserved")
	}
}

func TestProvisionCopiesBootstrapFilesWithoutClaudeMirror(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	root := filepath.Join(baseDir, "test-workspace")
	assets := filepath.Join(baseDir, "runtime-asset", "bootstrap")
	skillDir := filepath.Join(assets, "skills", "workspace-bootstrap")
	toolsRoot := filepath.Join(baseDir, "tools")
	otTools := filepath.Join(toolsRoot, "ot")

	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir assets: %v", err)
	}
	if err := os.MkdirAll(otTools, 0o755); err != nil {
		t.Fatalf("mkdir tools: %v", err)
	}
	files := map[string]string{
		filepath.Join(assets, "AGENTS.md"):          "workspace agents",
		filepath.Join(assets, "USER.md"):            "user bootstrap",
		filepath.Join(assets, "SKILLS.md"):          "skills bootstrap",
		filepath.Join(assets, "TOOLS.md"):           "tools bootstrap",
		filepath.Join(skillDir, "SKILL.md"):         "skill instructions",
		filepath.Join(skillDir, "notes.txt"):        "helper notes",
		filepath.Join(otTools, "read.sh"):           "#!/usr/bin/env bash\necho read\n",
		filepath.Join(toolsRoot, "custom.sh"):       "#!/usr/bin/env bash\necho custom\n",
		filepath.Join(root, "bootstrap", "USER.md"): "preserved user memory",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir parent %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	provisioned, err := Provision(root, assets, []string{"PATH=/usr/bin"}, nil)
	if err != nil {
		t.Fatalf("provision workspace: %v", err)
	}

	if provisioned.Root != root {
		t.Fatalf("unexpected root: %s", provisioned.Root)
	}
	assertFileContent(t, filepath.Join(root, "AGENTS.md"), "workspace agents")
	assertFileContent(t, filepath.Join(root, "bootstrap", "USER.md"), "preserved user memory")
	assertFileContent(t, filepath.Join(root, "bootstrap", "SKILLS.md"), "skills bootstrap")
	assertFileContent(t, filepath.Join(root, "bootstrap", "TOOLS.md"), "tools bootstrap")
	assertFileContent(t, filepath.Join(root, "bootstrap", "skills", "workspace-bootstrap", "SKILL.md"), "skill instructions")
	assertFileContent(t, filepath.Join(root, "tools", "ot", "read.sh"), "#!/usr/bin/env bash\necho read\n")
	assertFileContent(t, filepath.Join(root, "tools", "custom.sh"), "#!/usr/bin/env bash\necho custom\n")

	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("expected no .claude directory, got err=%v", err)
	}
}

func TestSyncDirectoryMissingSourceSucceeds(t *testing.T) {
	t.Parallel()

	target := filepath.Join(t.TempDir(), "target")
	if err := syncDirectory(filepath.Join(t.TempDir(), "missing"), target); err != nil {
		t.Fatalf("syncDirectory missing source: %v", err)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("expected target directory to exist, err=%v", err)
	}
}

func TestSyncDirectoryRebuildsTargetFromSource(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "file.txt"), []byte("fresh"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "old.txt"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	if err := syncDirectory(source, target); err != nil {
		t.Fatalf("syncDirectory: %v", err)
	}

	if _, err := os.Stat(filepath.Join(target, "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected stale file removal, stat err=%v", err)
	}
	assertFileContent(t, filepath.Join(target, "nested", "file.txt"), "fresh")
}

func TestCopyFileIfMissingPreservesExistingTarget(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := filepath.Join(root, "source.txt")
	target := filepath.Join(root, "target.txt")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	if err := copyFileIfMissing(source, target); err != nil {
		t.Fatalf("copyFileIfMissing: %v", err)
	}
	assertFileContent(t, target, "existing")
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("unexpected content for %s: %q", path, string(data))
	}
}
