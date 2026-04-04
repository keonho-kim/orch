package session

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func TestLoadCompactInputProducesPointerAwareTranscript(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := NewService(manager)

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

	input, err := service.LoadCompactInput(meta)
	if err != nil {
		t.Fatalf("load compact input: %v", err)
	}
	if input.ThroughSeq != 5 {
		t.Fatalf("unexpected through sequence: %d", input.ThroughSeq)
	}
	if !strings.Contains(input.Transcript, "[line 1] User: setup request") {
		t.Fatalf("expected user transcript line, got %q", input.Transcript)
	}
	if !strings.Contains(input.Transcript, "[line 5] Assistant: fix answer") {
		t.Fatalf("expected assistant transcript line, got %q", input.Transcript)
	}

	topics := ParseCompactTopics("Setup | lines=1,2\nFix | lines=4,5")
	if len(topics) != 2 {
		t.Fatalf("unexpected topics: %+v", topics)
	}
	if JoinCompactLines(topics[0].Lines) != "1,2" {
		t.Fatalf("unexpected topic lines: %+v", topics[0])
	}
	if got := RenderPointerParagraph("Setup summary", topics[0].Lines); !strings.Contains(got, "Pointer: ot-pointer://current?") {
		t.Fatalf("expected pointer paragraph, got %q", got)
	}
}
