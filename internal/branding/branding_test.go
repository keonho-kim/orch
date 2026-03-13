package branding

import (
	"strings"
	"testing"
)

func TestNormalizeASCIITrimsSharedIndentation(t *testing.T) {
	t.Parallel()

	normalized := NormalizeASCII([]string{
		"",
		"    alpha",
		"      beta",
		"    gamma",
		"",
	})

	if len(normalized) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(normalized))
	}
	if normalized[0] != "alpha" {
		t.Fatalf("unexpected first line: %q", normalized[0])
	}
	if normalized[1] != "  beta" {
		t.Fatalf("unexpected second line: %q", normalized[1])
	}
	if normalized[2] != "gamma" {
		t.Fatalf("unexpected third line: %q", normalized[2])
	}
}

func TestWordmarkIsNormalizedAndNonEmpty(t *testing.T) {
	t.Parallel()

	if len(Wordmark) == 0 {
		t.Fatal("expected wordmark lines")
	}
	hasZeroIndent := false
	for index, line := range Wordmark {
		if strings.TrimSpace(line) == "" {
			t.Fatalf("wordmark line %d is empty", index)
		}
		if leadingSpaces(line) == 0 {
			hasZeroIndent = true
		}
	}
	if !hasZeroIndent {
		t.Fatalf("expected normalized wordmark to contain at least one zero-indent line: %q", Wordmark[0])
	}
}
