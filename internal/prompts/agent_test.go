package prompts

import (
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestPlanSystemPromptKeepsReadOnlyContract(t *testing.T) {
	t.Parallel()

	prompt := SystemPrompt(domain.RunModePlan, domain.AgentRoleGateway, "common charter", "gateway role")
	if !strings.Contains(prompt, "common charter") {
		t.Fatalf("expected common prompt content, got %q", prompt)
	}
	if !strings.Contains(prompt, "gateway role") {
		t.Fatalf("expected role prompt content, got %q", prompt)
	}
	if !strings.Contains(prompt, "Plan mode is read-only.") {
		t.Fatalf("expected explicit plan restriction, got %q", prompt)
	}
}

func TestWorkerSystemPromptStatesWorkerRestrictions(t *testing.T) {
	t.Parallel()

	prompt := SystemPrompt(domain.RunModeReact, domain.AgentRoleWorker, "common charter", "worker role")
	if !strings.Contains(prompt, "worker role") {
		t.Fatalf("expected worker prompt content, got %q", prompt)
	}
	if !strings.Contains(prompt, "You are running as the worker agent.") {
		t.Fatalf("expected worker restriction block, got %q", prompt)
	}
}

func TestIterationContextIncludesRoleToolsAndTaskContract(t *testing.T) {
	t.Parallel()

	context := IterationContext(
		domain.RunRecord{
			Mode:          domain.RunModeReact,
			AgentRole:     domain.AgentRoleWorker,
			WorkspacePath: "/tmp/ws",
			CurrentCwd:    "/tmp/ws",
		},
		domain.AgentRoleWorker,
		"worker tools",
		"user context",
		"recent chat history",
		"frozen memory",
		"$skill:\ncontent",
		"- @README.md -> [README.md](/tmp/ws/README.md)",
		domain.PlanCache{},
		"",
		"Fix tests",
		"Update the broken test and stop.",
		"running",
	)

	if !strings.Contains(context, "Agent role:\nWorker") {
		t.Fatalf("expected role header, got %q", context)
	}
	if !strings.Contains(context, "bootstrap/TOOLS.md:\nworker tools") {
		t.Fatalf("expected tools guide, got %q", context)
	}
	if !strings.Contains(context, "bootstrap/USER.md:\nuser context") {
		t.Fatalf("expected user context, got %q", context)
	}
	if !strings.Contains(context, ".orch/chatHistory.md:\nrecent chat history") {
		t.Fatalf("expected bounded chat history, got %q", context)
	}
	if !strings.Contains(context, "Assigned task contract:\nUpdate the broken test and stop.") {
		t.Fatalf("expected task contract, got %q", context)
	}
}
