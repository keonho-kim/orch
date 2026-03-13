package prompts

import (
	"strings"
	"testing"

	"orch/domain"
)

func TestPlanSystemPromptUsesPathInspectionContract(t *testing.T) {
	t.Parallel()

	prompt := SystemPrompt(domain.RunModePlan)
	if !strings.Contains(prompt, "ot read --path <path>") {
		t.Fatalf("expected plan prompt to use path inspection contract, got %q", prompt)
	}
	if strings.Contains(prompt, "ot read --path <file>") {
		t.Fatalf("did not expect legacy file-only contract, got %q", prompt)
	}
}

func TestReactSystemPromptPrefersReadAndSearchTools(t *testing.T) {
	t.Parallel()

	prompt := SystemPrompt(domain.RunModeReact)
	if !strings.Contains(prompt, "ot read --path <path>") {
		t.Fatalf("expected react prompt to mention ot read inspection, got %q", prompt)
	}
	if !strings.Contains(prompt, "rg or find") {
		t.Fatalf("expected react prompt to mention rg/find, got %q", prompt)
	}
}
