package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/prompts"
	"github.com/keonho-kim/orch/internal/session"
)

type maintenanceClientStub struct {
	responses map[string]string
	errs      map[string]error
}

func (c maintenanceClientStub) Provider() domain.Provider {
	return domain.ProviderOllama
}

func (c maintenanceClientStub) Chat(_ context.Context, _ domain.ProviderSettings, request adapters.ChatRequest, _ adapters.DeltaHandler) (adapters.ChatResult, error) {
	systemPrompt := ""
	if len(request.Messages) > 0 {
		systemPrompt = request.Messages[0].Content
	}
	if err, ok := c.errs[systemPrompt]; ok {
		return adapters.ChatResult{}, err
	}
	if content, ok := c.responses[systemPrompt]; ok {
		return adapters.ChatResult{Content: content}, nil
	}
	return adapters.ChatResult{}, nil
}

func TestGenerateCompactSummaryProducesOrderedPointerAwareSummary(t *testing.T) {
	t.Parallel()

	manager := session.NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := &Service{
		sessionManager: manager,
		sessions:       session.NewService(manager),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: maintenanceClientStub{
				responses: map[string]string{
					prompts.SessionTopicsPrompt():                     "Setup | lines=1,2\nFix | lines=4,5",
					prompts.SessionTopicSummaryPrompt("Setup", "1,2"): "Setup summary",
					prompts.SessionTopicSummaryPrompt("Fix", "4,5"):   "Fix summary",
				},
			},
		},
		settings: domain.Settings{},
	}

	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
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
	for _, record := range records {
		if err := manager.AppendRecord(meta.SessionID, record); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	summary, throughSeq, err := service.generateCompactSummary(context.Background(), meta)
	if err != nil {
		t.Fatalf("generate compact summary: %v", err)
	}
	if throughSeq != 5 {
		t.Fatalf("unexpected through sequence: %d", throughSeq)
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

func TestDeriveSessionTitleFallsBackToFirstUserRecord(t *testing.T) {
	t.Parallel()

	manager := session.NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := &Service{
		sessionManager: manager,
		sessions:       session.NewService(manager),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: maintenanceClientStub{
				errs: map[string]error{
					prompts.SessionTitlePrompt(): fmt.Errorf("provider unavailable"),
				},
			},
		},
		settings: domain.Settings{},
	}

	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := service.sessions.AppendUser(meta, "R1", "Investigate failing regression tests"); err != nil {
		t.Fatalf("append user: %v", err)
	}

	title, err := service.deriveSessionTitle(context.Background(), meta)
	if err != nil {
		t.Fatalf("derive title: %v", err)
	}
	if title != "Investigate failing regression tests" {
		t.Fatalf("unexpected fallback title: %q", title)
	}
}

func TestRunChatHistoryUserSummarySkipsAppendOnProviderError(t *testing.T) {
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
		ctx:            context.Background(),
		paths:          paths,
		sessionManager: manager,
		sessions:       session.NewService(manager),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: maintenanceClientStub{
				errs: map[string]error{
					prompts.ChatHistoryUserPrompt(): fmt.Errorf("provider unavailable"),
				},
			},
		},
	}

	service.runChatHistoryUserSummary(meta, "R1", "please inspect the failing tests")

	history, err := manager.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if strings.TrimSpace(history) != "" {
		t.Fatalf("expected empty chat history, got %q", history)
	}
}
