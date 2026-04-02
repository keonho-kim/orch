package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAppliesUnifiedPatch(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "README.md")
	if err := os.WriteFile(target, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	patch := strings.Join([]string{
		"--- README.md",
		"+++ README.md",
		"@@ -1,2 +1,2 @@",
		" alpha",
		"-beta",
		"+gamma",
		"",
	}, "\n")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	if err := run([]string{"-p0", "-u"}, strings.NewReader(patch)); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "alpha\ngamma\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestRunRejectsUnsupportedArgs(t *testing.T) {
	t.Parallel()

	if err := run([]string{"-p1"}, strings.NewReader("")); err == nil {
		t.Fatal("expected unsupported args to fail")
	}
}
