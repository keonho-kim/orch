package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
)

func TestSelectedSkillNamesDeduplicateMentions(t *testing.T) {
	t.Parallel()

	names := selectedSkillNames("use $alpha and $beta then repeat $alpha")
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Fatalf("unexpected selected skill names: %+v", names)
	}
}

func TestResolveSelectedSkillsLoadsSkillContent(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	skillPath := filepath.Join(workspace, "bootstrap", "skills", "alpha", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("skill alpha"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}

	selected, err := resolveSelectedSkills(workspace, "please use $alpha")
	if err != nil {
		t.Fatalf("resolve selected skills: %v", err)
	}
	if len(selected) != 1 || selected[0].Name != "alpha" || selected[0].Content != "skill alpha" {
		t.Fatalf("unexpected selected skills: %+v", selected)
	}
}

func TestResolveSelectedSkillsRejectsUnknownSkill(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	_, err := resolveSelectedSkills(workspace, "please use $missing")
	if err == nil || !strings.Contains(err.Error(), "$missing") {
		t.Fatalf("expected unknown skill error, got %v", err)
	}
}

func TestBuildIterationContextIncludesSelectedSkillsAndChatHistory(t *testing.T) {
	t.Parallel()

	service, paths := newIterationContextTestService(t)
	writeIterationContextFixtures(t, paths)
	appendIterationContextHistory(t, paths.SessionsDir)

	context, err := service.buildIterationContext(domain.RunRecord{
		Mode:          domain.RunModeReact,
		AgentRole:     domain.AgentRoleGateway,
		SessionID:     "S1",
		WorkspacePath: paths.TestWorkspace,
		CurrentCwd:    paths.TestWorkspace,
	}, []selectedSkill{{
		Name:    "alpha",
		Path:    "bootstrap/skills/alpha/SKILL.md",
		Content: "skill alpha",
	}}, "- @README.md -> [README.md](/tmp/ws/README.md) at /tmp/ws/README.md", domain.PlanCache{}, "")
	if err != nil {
		t.Fatalf("build iteration context: %v", err)
	}
	assertIterationContextContains(t, context)
}

func newIterationContextTestService(t *testing.T) (*Service, config.Paths) {
	t.Helper()
	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	manager := session.NewManager(paths.SessionsDir)
	service := &Service{
		ctx:            context.Background(),
		paths:          paths,
		sessionManager: manager,
		sessions:       session.NewService(manager),
	}
	if err := os.MkdirAll(paths.SessionsDir, 0o755); err != nil {
		t.Fatalf("mkdir sessions dir: %v", err)
	}
	if err := manager.SaveMetadata(domain.SessionMetadata{
		SessionID:     "S1",
		WorkspacePath: paths.TestWorkspace,
		WorkerRole:    domain.AgentRoleGateway,
		TaskStatus:    "running",
	}); err != nil {
		t.Fatalf("save session metadata: %v", err)
	}
	return service, paths
}

func writeIterationContextFixtures(t *testing.T, paths config.Paths) {
	t.Helper()
	for relative, content := range map[string]string{
		"AGENTS.md":                            "agents",
		filepath.Join("bootstrap", "USER.md"):  "user note",
		filepath.Join("bootstrap", "TOOLS.md"): "tools guide",
	} {
		path := filepath.Join(paths.TestWorkspace, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
}

func appendIterationContextHistory(t *testing.T, sessionsDir string) {
	t.Helper()
	manager := session.NewManager(sessionsDir)
	if err := manager.AppendChatHistory(session.ChatHistoryEntry{
		CreatedAt: time.Now(),
		SessionID: "S1",
		RunID:     "R1",
		Speaker:   session.ChatHistorySpeakerAssistant,
		Summary:   "recent shared memory",
	}); err != nil {
		t.Fatalf("append chat history: %v", err)
	}
}

func assertIterationContextContains(t *testing.T, context string) {
	t.Helper()
	if !strings.Contains(context, "Selected skill content for this call:") {
		t.Fatalf("expected selected skill content in context, got %q", context)
	}
	if !strings.Contains(context, "bootstrap/TOOLS.md:\ntools guide") {
		t.Fatalf("expected tools guide in context, got %q", context)
	}
	if !strings.Contains(context, "bootstrap/USER.md:\nUser memory:\nuser note") {
		t.Fatalf("expected bounded user memory in context, got %q", context)
	}
	if !strings.Contains(context, ".orch/chatHistory.md:\n## ") || !strings.Contains(context, "recent shared memory") {
		t.Fatalf("expected bounded chat history excerpt, got %q", context)
	}
	if !strings.Contains(context, "Resolved workspace references for this request:") {
		t.Fatalf("expected resolved references in context, got %q", context)
	}
}
