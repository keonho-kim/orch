package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReferenceMentionsFindsFilesAndDirs(t *testing.T) {
	t.Parallel()

	mentions := parseReferenceMentions("check @README.md and #config plus @README.md")
	if len(mentions) != 2 {
		t.Fatalf("unexpected mentions: %+v", mentions)
	}
	if mentions[0].Token != "@README.md" || mentions[0].Kind != referenceKindFile {
		t.Fatalf("unexpected file mention: %+v", mentions[0])
	}
	if mentions[1].Token != "#config" || mentions[1].Kind != referenceKindDir {
		t.Fatalf("unexpected dir mention: %+v", mentions[1])
	}
}

func TestReferenceResolverIncludesHiddenFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("TOKEN=1"), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	resolver := newReferenceResolver()
	block, err := resolver.Resolve(root, root, "inspect @.env")
	if err != nil {
		t.Fatalf("resolve references: %v", err)
	}
	if !strings.Contains(block, "[@.env]") && !strings.Contains(block, "[.env](") {
		t.Fatalf("expected hidden file reference, got %q", block)
	}
}

func TestReferenceResolverRanksExactBeforePrefixAndSubstring(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, relative := range []string{
		"alpha.txt",
		"alpha-extra.txt",
		"nested/beta-alpha.txt",
	} {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte(relative), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}

	index, err := buildWorkspaceReferenceIndex(root)
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	scored := rankReferenceCandidates(index, root, referenceMention{
		Token: "@alpha.txt",
		Name:  "alpha.txt",
		Kind:  referenceKindFile,
	})
	if len(scored) == 0 || filepath.Base(scored[0].AbsPath) != "alpha.txt" {
		t.Fatalf("unexpected ranked candidates: %+v", scored)
	}
}

func TestReferenceResolverReportsAmbiguity(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, relative := range []string{
		"a/config",
		"b/config",
	} {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relative, err)
		}
	}

	resolver := newReferenceResolver()
	block, err := resolver.Resolve(root, root, "inspect #config")
	if err != nil {
		t.Fatalf("resolve references: %v", err)
	}
	if !strings.Contains(block, "is ambiguous") {
		t.Fatalf("expected ambiguity block, got %q", block)
	}
}
