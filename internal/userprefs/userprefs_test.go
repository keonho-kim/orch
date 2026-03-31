package userprefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertManagedValueWritesManagedBlock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "USER.md")
	if err := os.WriteFile(path, []byte("# User Intent\n"), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	written, err := UpsertManagedValue(path, "preferred_editor", "vim")
	if err != nil {
		t.Fatalf("upsert managed value: %v", err)
	}
	if !written {
		t.Fatal("expected managed block to be written")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read user file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `preferred_editor = "vim"`) {
		t.Fatalf("expected managed value block, got %q", content)
	}
}

func TestReadMemoryExcerptReturnsUserAndManagedSlices(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "USER.md")
	content := "# User Intent\n\nStable user note.\n\n" + startMarker + "\n" + `preferred_editor = "vim"` + "\n" + endMarker + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	excerpt, err := ReadMemoryExcerpt(path, 128, 128)
	if err != nil {
		t.Fatalf("read memory excerpt: %v", err)
	}
	if !strings.Contains(excerpt, "User memory:") || !strings.Contains(excerpt, "Stable user note.") {
		t.Fatalf("expected user memory in excerpt, got %q", excerpt)
	}
	if !strings.Contains(excerpt, "Managed memory:") || !strings.Contains(excerpt, `preferred_editor = "vim"`) {
		t.Fatalf("expected managed memory in excerpt, got %q", excerpt)
	}
}
