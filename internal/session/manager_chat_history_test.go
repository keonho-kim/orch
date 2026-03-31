package session

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManagerChatHistoryPathUsesLocalStateRoot(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".orch", "sessions")
	manager := NewManager(root)

	got := manager.ChatHistoryPath()
	want := filepath.Join(filepath.Dir(root), "chatHistory.md")
	if got != want {
		t.Fatalf("unexpected chat history path: got %q want %q", got, want)
	}
}

func TestManagerAppendAndReadChatHistory(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".orch", "sessions")
	manager := NewManager(root)

	now := time.Date(2026, time.March, 17, 9, 10, 0, 0, time.UTC)
	if err := manager.AppendChatHistory(ChatHistoryEntry{
		CreatedAt: now,
		SessionID: "S1",
		RunID:     "R1",
		Speaker:   ChatHistorySpeakerUser,
		Summary:   "User requested a durable rolling digest.",
	}); err != nil {
		t.Fatalf("append user chat history: %v", err)
	}
	if err := manager.AppendChatHistory(ChatHistoryEntry{
		CreatedAt: now.Add(time.Second),
		SessionID: "S1",
		RunID:     "R1",
		Speaker:   ChatHistorySpeakerAssistant,
		Summary:   "Assistant explained the implementation approach.",
	}); err != nil {
		t.Fatalf("append assistant chat history: %v", err)
	}

	content, err := manager.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if !strings.Contains(content, "## 2026-03-17T09:10:00Z | user") {
		t.Fatalf("expected user heading, got %q", content)
	}
	if !strings.Contains(content, "- session_id: S1") || !strings.Contains(content, "- run_id: R1") {
		t.Fatalf("expected session/run metadata, got %q", content)
	}
	if !strings.Contains(content, "- summary: Assistant explained the implementation approach.") {
		t.Fatalf("expected assistant summary, got %q", content)
	}
}

func TestManagerAppendChatHistorySerializesConcurrentWrites(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".orch", "sessions")
	manager := NewManager(root)

	const count = 20
	var wg sync.WaitGroup
	wg.Add(count)
	for index := 0; index < count; index++ {
		index := index
		go func() {
			defer wg.Done()
			err := manager.AppendChatHistory(ChatHistoryEntry{
				CreatedAt: time.Now().Add(time.Duration(index) * time.Second),
				SessionID: "S1",
				RunID:     fmt.Sprintf("R%d", index),
				Speaker:   ChatHistorySpeakerUser,
				Summary:   fmt.Sprintf("summary-%d", index),
			})
			if err != nil {
				t.Errorf("append chat history %d: %v", index, err)
			}
		}()
	}
	wg.Wait()

	content, err := manager.ReadChatHistory()
	if err != nil {
		t.Fatalf("read chat history: %v", err)
	}
	if got := strings.Count(content, "## "); got != count {
		t.Fatalf("unexpected entry count: got %d want %d", got, count)
	}
	for index := 0; index < count; index++ {
		if !strings.Contains(content, fmt.Sprintf("- run_id: R%d", index)) {
			t.Fatalf("missing run entry for R%d", index)
		}
	}
}

func TestManagerReadChatHistoryRecentReturnsBoundedTail(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), ".orch", "sessions")
	manager := NewManager(root)

	for index := 0; index < 4; index++ {
		err := manager.AppendChatHistory(ChatHistoryEntry{
			CreatedAt: time.Now().Add(time.Duration(index) * time.Second),
			SessionID: fmt.Sprintf("S%d", index),
			RunID:     fmt.Sprintf("R%d", index),
			Speaker:   ChatHistorySpeakerUser,
			Summary:   fmt.Sprintf("summary-%d", index),
		})
		if err != nil {
			t.Fatalf("append chat history %d: %v", index, err)
		}
	}

	content, err := manager.ReadChatHistoryRecent(2, 512)
	if err != nil {
		t.Fatalf("read recent chat history: %v", err)
	}
	if strings.Contains(content, "summary-0") || strings.Contains(content, "summary-1") {
		t.Fatalf("expected older entries to be dropped, got %q", content)
	}
	if !strings.Contains(content, "summary-2") || !strings.Contains(content, "summary-3") {
		t.Fatalf("expected recent entries, got %q", content)
	}
}
