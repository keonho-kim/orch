package tui

import (
	"strings"
	"testing"
)

func TestRenderInlineMarkdownStylesBasicFormatting(t *testing.T) {
	t.Parallel()

	rendered := renderInlineMarkdown("**bold** *italic* __under__")
	if strings.Contains(rendered, "**") || strings.Contains(rendered, "__") || strings.Contains(rendered, "*italic*") {
		t.Fatalf("expected markdown markers to be rendered away, got %q", rendered)
	}
	if !strings.Contains(rendered, "bold") || !strings.Contains(rendered, "italic") || !strings.Contains(rendered, "under") {
		t.Fatalf("expected formatted text content to remain, got %q", rendered)
	}
}
