package session

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/prompts"
)

type compactRunnerStub struct{}

func (compactRunnerStub) Run(_ context.Context, _ domain.Provider, _ string, systemPrompt string, _ string) (string, error) {
	switch {
	case systemPrompt == prompts.SessionTopicsPrompt():
		return "Setup | lines=1,2\nFix | lines=4,5", nil
	case strings.Contains(systemPrompt, "Setup"):
		return "Setup summary", nil
	case strings.Contains(systemPrompt, "Fix"):
		return "Fix summary", nil
	default:
		return "", nil
	}
}

func TestCompactProducesOrderedPointerAwareSummary(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := NewService(manager, compactRunnerStub{})

	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	records := []domain.SessionRecord{
		{Seq: 1, SessionID: meta.SessionID, Type: domain.SessionRecordUser, Content: "setup request", CreatedAt: time.Now()},
		{Seq: 2, SessionID: meta.SessionID, Type: domain.SessionRecordAssistant, Content: "setup answer", CreatedAt: time.Now()},
		{Seq: 3, SessionID: meta.SessionID, Type: domain.SessionRecordTool, ToolName: "exec", Content: "tool output", CreatedAt: time.Now()},
		{Seq: 4, SessionID: meta.SessionID, Type: domain.SessionRecordUser, Content: "fix request", CreatedAt: time.Now()},
		{Seq: 5, SessionID: meta.SessionID, Type: domain.SessionRecordAssistant, Content: "fix answer", CreatedAt: time.Now()},
	}

	summary, err := service.generateCompactSummary(context.Background(), meta, records, 5)
	if err != nil {
		t.Fatalf("generate compact summary: %v", err)
	}
	if !strings.Contains(summary, "Setup summary\nPointer: ot-pointer://current?") {
		t.Fatalf("expected setup pointer summary, got %q", summary)
	}
	if !strings.Contains(summary, "Fix summary\nPointer: ot-pointer://current?") {
		t.Fatalf("expected fix pointer summary, got %q", summary)
	}
	if strings.Index(summary, "Setup summary") > strings.Index(summary, "Fix summary") {
		t.Fatalf("expected chronological topic order, got %q", summary)
	}
}
