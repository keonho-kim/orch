package userprefs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDetectedLanguageAppendsManagedBlock(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "USER.md")
	if err := os.WriteFile(path, []byte("# User Intent\n"), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	written, err := EnsureDetectedLanguage(path, "kor")
	if err != nil {
		t.Fatalf("ensure detected language: %v", err)
	}
	if !written {
		t.Fatal("expected managed block to be written")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read user file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `detected_language = "kor"`) {
		t.Fatalf("expected detected language block, got %q", content)
	}
}

func TestEnsureDetectedLanguageDoesNotOverwriteExistingValue(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "USER.md")
	content := "# User Intent\n\n" + startMarker + "\n" + `detected_language = "en"` + "\n" + endMarker + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	written, err := EnsureDetectedLanguage(path, "kor")
	if err != nil {
		t.Fatalf("ensure detected language: %v", err)
	}
	if written {
		t.Fatal("did not expect overwrite of existing detected language")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read user file: %v", err)
	}
	if strings.Contains(string(data), `detected_language = "kor"`) {
		t.Fatalf("unexpected overwrite: %q", string(data))
	}
}
