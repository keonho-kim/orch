package session

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func TestAppendChatHistoryUserAndAssistantWriteRollingEntries(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".orch", "sessions")
	manager := NewManager(root)
	service := NewService(manager)

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
	if err := service.AppendChatHistoryUser(meta, "R1", "summarized user request"); err != nil {
		t.Fatalf("append user summary: %v", err)
	}
	if err := service.AppendChatHistoryAssistant(meta, "R1", "summarized assistant response"); err != nil {
		t.Fatalf("append assistant summary: %v", err)
	}

	content, err := manager.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if !strings.Contains(content, "- summary: summarized user request") {
		t.Fatalf("expected summarized entry, got %q", content)
	}
	if !strings.Contains(content, "- summary: summarized assistant response") {
		t.Fatalf("expected assistant summary entry, got %q", content)
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
	err := manager.AppendChatHistory(ChatHistoryEntry{
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
