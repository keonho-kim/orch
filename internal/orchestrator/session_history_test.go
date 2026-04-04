package orchestrator

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
)

type chatHistoryClientStub struct {
	content string
}

func (c chatHistoryClientStub) Provider() domain.Provider {
	return domain.ProviderOllama
}

func (c chatHistoryClientStub) Chat(_ context.Context, _ domain.ProviderSettings, _ adapters.ChatRequest, _ adapters.DeltaHandler) (adapters.ChatResult, error) {
	return adapters.ChatResult{Content: c.content}, nil
}

func TestSessionContextMessagesUseLatestCompactAndLaterRecords(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	manager := session.NewManager(paths.SessionsDir)
	meta, err := manager.Create(repoRoot, domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	records := []domain.SessionRecord{
		{Seq: 1, SessionID: meta.SessionID, Type: domain.SessionRecordUser, Content: "old user", CreatedAt: time.Now()},
		{Seq: 2, SessionID: meta.SessionID, Type: domain.SessionRecordAssistant, Content: "old assistant", CreatedAt: time.Now()},
		{Seq: 3, SessionID: meta.SessionID, Type: domain.SessionRecordCompact, Content: "old compact", ThroughSeq: 2, CreatedAt: time.Now()},
		{Seq: 4, SessionID: meta.SessionID, Type: domain.SessionRecordUser, Content: "new user", CreatedAt: time.Now()},
		{Seq: 5, SessionID: meta.SessionID, Type: domain.SessionRecordAssistant, Content: "new assistant", CreatedAt: time.Now()},
	}
	for _, record := range records {
		if err := manager.AppendRecord(meta.SessionID, record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	meta.Summary = "old compact"
	meta.LastSequence = 5
	meta.LastCompactedSeq = 2
	meta.UpdatedAt = time.Now()
	if err := manager.SaveMetadata(meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	service := &Service{
		ctx:            context.Background(),
		paths:          paths,
		sessions:       session.NewService(manager),
		currentSession: meta,
	}

	messages, err := service.sessionContextMessages()
	if err != nil {
		t.Fatalf("session context messages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if messages[0].Role != "system" || messages[0].Content == "" {
		t.Fatalf("expected compact system summary, got %+v", messages[0])
	}
	if messages[1].Content != "new user" || messages[2].Content != "new assistant" {
		t.Fatalf("unexpected post-compact messages: %+v", messages)
	}

	if got := filepath.Base(paths.SessionsDir); got != "sessions" {
		t.Fatalf("unexpected sessions dir: %s", got)
	}
}

func TestLoadInheritedContextUsesParentCompactSummaryAndLaterRecords(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	manager := session.NewManager(paths.SessionsDir)
	meta, err := manager.Create(repoRoot, domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	records := []domain.SessionRecord{
		{Seq: 1, SessionID: meta.SessionID, Type: domain.SessionRecordUser, Content: "old user", CreatedAt: time.Now()},
		{Seq: 2, SessionID: meta.SessionID, Type: domain.SessionRecordAssistant, Content: "old assistant", CreatedAt: time.Now()},
		{Seq: 3, SessionID: meta.SessionID, Type: domain.SessionRecordCompact, Content: "parent compact", ThroughSeq: 2, CreatedAt: time.Now()},
		{Seq: 4, SessionID: meta.SessionID, Type: domain.SessionRecordUser, Content: "new user", CreatedAt: time.Now()},
	}
	for _, record := range records {
		if err := manager.AppendRecord(meta.SessionID, record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	meta.Summary = "parent compact"
	meta.LastSequence = 4
	meta.LastCompactedSeq = 2
	meta.UpdatedAt = time.Now()
	if err := manager.SaveMetadata(meta); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	service := &Service{
		ctx:      context.Background(),
		paths:    paths,
		sessions: session.NewService(manager),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: chatHistoryClientStub{content: "user digest"},
		},
	}

	inherited, err := service.loadInheritedContext(BootOptions{
		ParentSessionID:      meta.SessionID,
		InheritParentContext: true,
	})
	if err != nil {
		t.Fatalf("load inherited context: %v", err)
	}
	if inherited.Summary != "parent compact" {
		t.Fatalf("unexpected inherited summary: %q", inherited.Summary)
	}
	if len(inherited.Records) != 1 || inherited.Records[0].Content != "new user" {
		t.Fatalf("unexpected inherited records: %+v", inherited.Records)
	}
}

func TestRunChatHistoryUserSummaryAppendsEntry(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	manager := session.NewManager(paths.SessionsDir)
	meta, err := manager.Create(repoRoot, domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	service := &Service{
		ctx:      context.Background(),
		paths:    paths,
		sessions: session.NewService(manager),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: chatHistoryClientStub{content: "user digest"},
		},
	}

	service.runChatHistoryUserSummary(meta, "R1", "please inspect the failing tests")

	history, err := service.sessions.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if !strings.Contains(history, "| user") || !strings.Contains(history, "user digest") {
		t.Fatalf("unexpected chat history: %q", history)
	}
}

func TestRunChatHistoryAssistantSummaryAppendsEntry(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	manager := session.NewManager(paths.SessionsDir)
	meta, err := manager.Create(repoRoot, domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	service := &Service{
		ctx:      context.Background(),
		paths:    paths,
		sessions: session.NewService(manager),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: chatHistoryClientStub{content: "assistant digest"},
		},
	}

	service.runChatHistoryAssistantSummary(meta.SessionID, "R2", "I found the regression and described the fix.")

	history, err := service.sessions.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if !strings.Contains(history, "| assistant") || !strings.Contains(history, "assistant digest") {
		t.Fatalf("unexpected chat history: %q", history)
	}
}
