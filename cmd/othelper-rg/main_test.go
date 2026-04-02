package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunFindsMatches(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, "README.md")
	if err := os.WriteFile(target, []byte("alpha\nbeta\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	var stdout bytes.Buffer
	matched, err := run([]string{"--line-number", "--color", "never", "--no-heading", "--", "alpha", root}, &stdout)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !matched {
		t.Fatal("expected match")
	}
	if !strings.Contains(stdout.String(), "README.md:1:alpha") {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestRunSkipsHiddenPathsWithoutFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	hiddenDir := filepath.Join(root, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0o755); err != nil {
		t.Fatalf("mkdir hidden dir: %v", err)
	}
	target := filepath.Join(hiddenDir, "secret.txt")
	if err := os.WriteFile(target, []byte("secret\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	var stdout bytes.Buffer
	matched, err := run([]string{"--line-number", "--color", "never", "--no-heading", "--", "secret", root}, &stdout)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if matched {
		t.Fatal("did not expect hidden match without --hidden")
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}
