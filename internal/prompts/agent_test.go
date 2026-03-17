package prompts

import (
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestPlanSystemPromptUsesPathInspectionContract(t *testing.T) {
	t.Parallel()

	prompt := SystemPrompt(domain.RunModePlan)
	if !strings.Contains(prompt, "ot read --path <path>") {
		t.Fatalf("expected plan prompt to use path inspection contract, got %q", prompt)
	}
	if !strings.Contains(prompt, "ot list [--path <path>]") {
		t.Fatalf("expected plan prompt to mention ot list, got %q", prompt)
	}
	if !strings.Contains(prompt, "ot search [--path <path>] [--name <glob>] [--content <pattern>]") {
		t.Fatalf("expected plan prompt to mention ot search, got %q", prompt)
	}
	if !strings.Contains(prompt, "@filename") || !strings.Contains(prompt, "#dir-name") {
		t.Fatalf("expected plan prompt to mention workspace references, got %q", prompt)
	}
	if !strings.Contains(prompt, "$<skill-name>") {
		t.Fatalf("expected plan prompt to mention explicit skill selection, got %q", prompt)
	}
}

func TestReactSystemPromptPrefersReadAndSearchTools(t *testing.T) {
	t.Parallel()

	prompt := SystemPrompt(domain.RunModeReact)
	if !strings.Contains(prompt, "ot read --path <path>") {
		t.Fatalf("expected react prompt to mention ot read inspection, got %q", prompt)
	}
	if !strings.Contains(prompt, "Prefer ot list") {
		t.Fatalf("expected react prompt to mention ot list, got %q", prompt)
	}
	if !strings.Contains(prompt, "ot search") {
		t.Fatalf("expected react prompt to mention ot search, got %q", prompt)
	}
	if !strings.Contains(prompt, "ot subagent --prompt <task>") {
		t.Fatalf("expected react prompt to mention ot subagent, got %q", prompt)
	}
	if !strings.Contains(prompt, "ot pointer --value <ot-pointer>") {
		t.Fatalf("expected react prompt to mention ot pointer, got %q", prompt)
	}
	if !strings.Contains(prompt, "Use rg or find directly only") {
		t.Fatalf("expected react prompt to keep rg/find fallback wording, got %q", prompt)
	}
	if !strings.Contains(prompt, "@filename") || !strings.Contains(prompt, "#dir-name") {
		t.Fatalf("expected react prompt to mention workspace references, got %q", prompt)
	}
	if !strings.Contains(prompt, "$<skill-name>") {
		t.Fatalf("expected react prompt to mention explicit skill selection, got %q", prompt)
	}
}

func TestIterationContextIncludesToolSummaryAndSkillsIndex(t *testing.T) {
	t.Parallel()

	context := IterationContext(
		domain.RunRecord{
			Mode:          domain.RunModeReact,
			WorkspacePath: "/tmp/ws",
			CurrentCwd:    "/tmp/ws",
		},
		"product",
		"agents",
		"user",
		"skills index",
		"$workspace-bootstrap (bootstrap/skills/workspace-bootstrap/SKILL.md):\ncontent",
		"rolling chat history",
		"- @README.md -> [README.md](/tmp/ws/README.md) at /tmp/ws/README.md",
		domain.PlanCache{},
		"",
	)

	if !strings.Contains(context, "bootstrap/SKILLS.md:\nskills index") {
		t.Fatalf("expected context to include skills index, got %q", context)
	}
	if !strings.Contains(context, "Available tools for this call:") {
		t.Fatalf("expected context to include tool summary heading, got %q", context)
	}
	if !strings.Contains(context, "- exec: Run one allowed CLI-style command.") {
		t.Fatalf("expected context to include concise tool summary, got %q", context)
	}
	if !strings.Contains(context, "Selected skill content for this call:") {
		t.Fatalf("expected context to include selected skill content, got %q", context)
	}
	if !strings.Contains(context, ".orch/chatHistory.md:\nrolling chat history") {
		t.Fatalf("expected context to include chatHistory, got %q", context)
	}
	if !strings.Contains(context, "Resolved workspace references for this request:") {
		t.Fatalf("expected context to include resolved references, got %q", context)
	}
}
