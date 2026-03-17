package session

import (
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func TestManagerCreateAppendAndLoadRecords(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	record := domain.SessionRecord{
		Seq:       1,
		SessionID: meta.SessionID,
		Type:      domain.SessionRecordUser,
		Content:   "hello",
		CreatedAt: time.Now(),
	}
	if err := manager.AppendRecord(meta.SessionID, record); err != nil {
		t.Fatalf("append record: %v", err)
	}

	records, err := manager.LoadRecords(meta.SessionID)
	if err != nil {
		t.Fatalf("load records: %v", err)
	}
	if len(records) != 1 || records[0].Content != "hello" {
		t.Fatalf("unexpected records: %+v", records)
	}

	loadedMeta, err := manager.LoadMetadata(meta.SessionID)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if loadedMeta.SessionID != meta.SessionID {
		t.Fatalf("unexpected metadata: %+v", loadedMeta)
	}
}

func TestListSessionsSkipsEmptyMetadataOnlyEntries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	manager := NewManager(root)

	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	sessions, err := manager.ListSessions(10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected metadata-only session to be hidden, got %+v", sessions)
	}

	if err := manager.AppendRecord(meta.SessionID, domain.SessionRecord{
		Seq:       1,
		SessionID: meta.SessionID,
		Type:      domain.SessionRecordUser,
		Content:   "hello",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("append record: %v", err)
	}

	sessions, err = manager.ListSessions(10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].SessionID != meta.SessionID {
		t.Fatalf("unexpected sessions: %+v", sessions)
	}
}

func TestLatestSessionIDReturnsMostRecentMetadata(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	first, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now().Add(-time.Hour), "", "")
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "")
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	for _, sessionID := range []string{first.SessionID, second.SessionID} {
		if err := manager.AppendRecord(sessionID, domain.SessionRecord{
			Seq:       1,
			SessionID: sessionID,
			Type:      domain.SessionRecordUser,
			Content:   sessionID,
			CreatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}

	if err := manager.SaveMetadata(domain.SessionMetadata{
		SessionID:     second.SessionID,
		WorkspacePath: "/repo",
		Provider:      domain.ProviderOllama,
		Model:         "model",
		Title:         "Second",
		StartedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	sessionID, err := manager.LatestSessionID()
	if err != nil {
		t.Fatalf("latest session id: %v", err)
	}
	if sessionID != second.SessionID {
		t.Fatalf("unexpected latest session id: %s", sessionID)
	}

}

func TestManagerCreatePersistsParentLinkage(t *testing.T) {
	t.Parallel()

	manager := NewManager(t.TempDir())
	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "PARENT", "R9")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	loaded, err := manager.LoadMetadata(meta.SessionID)
	if err != nil {
		t.Fatalf("load metadata: %v", err)
	}
	if loaded.ParentSessionID != "PARENT" || loaded.ParentRunID != "R9" {
		t.Fatalf("unexpected parent linkage: %+v", loaded)
	}
}
