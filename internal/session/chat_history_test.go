package session

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

type chatHistoryRunnerStub struct {
	mu         sync.Mutex
	response   string
	err        error
	calls      int
	lastPrompt string
	lastUser   string
}

func (s *chatHistoryRunnerStub) Run(
	_ context.Context,
	_ domain.Provider,
	_ string,
	systemPrompt string,
	userPrompt string,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.lastPrompt = systemPrompt
	s.lastUser = userPrompt
	if s.err != nil {
		return "", s.err
	}
	return s.response, nil
}

func TestSummarizeChatHistoryUserUsesRunner(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	runner := &chatHistoryRunnerStub{response: "short summary"}
	service := NewService(manager, runner)

	meta := domain.SessionMetadata{Provider: domain.ProviderOllama, Model: "model"}
	summary, err := service.SummarizeChatHistoryUser(context.Background(), meta, "Need a resilient rolling digest")
	if err != nil {
		t.Fatalf("summarize user: %v", err)
	}
	if summary != "short summary" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if runner.lastPrompt != chatHistoryUserPrompt {
		t.Fatalf("unexpected prompt: %q", runner.lastPrompt)
	}
}

func TestSummarizeChatHistoryAssistantFallsBackWhenRunnerMissing(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := NewService(manager, nil)

	content := strings.Repeat("x", chatHistoryFallbackLimit+20)
	summary, err := service.SummarizeChatHistoryAssistant(context.Background(), domain.SessionMetadata{}, content)
	if err != nil {
		t.Fatalf("summarize assistant fallback: %v", err)
	}
	if len(summary) != chatHistoryFallbackLimit {
		t.Fatalf("unexpected fallback summary length: got %d want %d", len(summary), chatHistoryFallbackLimit)
	}
}

func TestAppendChatHistoryUserSummaryWritesRollingEntry(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".orch", "sessions")
	manager := NewManager(root)
	runner := &chatHistoryRunnerStub{response: "summarized user request"}
	service := NewService(manager, runner)

	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session metadata: %v", err)
	}
	updatedMeta, err := service.AppendUser(meta, "R1", "Implement chat history")
	if err != nil {
		t.Fatalf("append user record: %v", err)
	}
	if _, err := service.AppendAssistant(updatedMeta, "R1", "Done with implementation", domain.UsageStats{}); err != nil {
		t.Fatalf("append assistant record: %v", err)
	}
	if err := service.AppendChatHistoryUserSummary(context.Background(), meta, "R1", "Implement chat history"); err != nil {
		t.Fatalf("append user summary: %v", err)
	}
	if err := service.AppendChatHistoryAssistantSummary(context.Background(), meta, "R1", "Done with implementation"); err != nil {
		t.Fatalf("append assistant summary: %v", err)
	}

	content, err := service.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if !strings.Contains(content, "- summary: summarized user request") {
		t.Fatalf("expected summarized entry, got %q", content)
	}
	if !strings.Contains(content, "ot-pointer://current?") {
		t.Fatalf("expected pointer entry, got %q", content)
	}
	if strings.Count(content, "## ") != 2 {
		t.Fatalf("unexpected number of chat history entries: %q", content)
	}
}

func TestAppendChatHistoryRejectsEmptySummary(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := NewService(manager, nil)
	err := service.AppendChatHistory(ChatHistoryEntry{
		CreatedAt: time.Now(),
		SessionID: "S1",
		RunID:     "R1",
		Speaker:   ChatHistorySpeakerUser,
		Summary:   "   ",
	})
	if err == nil {
		t.Fatal("expected empty summary append to fail")
	}
}
