package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func TestStoreIndexesSessionsMemoriesSkillsAndTaskOutcomes(t *testing.T) {
	t.Parallel()

	store, err := Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	meta := domain.SessionMetadata{
		SessionID:     "S1",
		WorkspacePath: "/repo",
		Provider:      domain.ProviderOllama,
		Model:         "qwen",
		Title:         "Improve memory loop",
		StartedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := store.UpsertSession(ctx, meta); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := store.AppendSessionMessage(ctx, domain.SessionRecord{
		Seq:       1,
		SessionID: "S1",
		RunID:     "R1",
		Type:      domain.SessionRecordAssistant,
		Content:   "Resilient memory snapshot for future runs",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("append session message: %v", err)
	}

	sessionResults, err := store.SearchSessionMessages(ctx, "/repo", "memory snapshot", 5)
	if err != nil {
		t.Fatalf("search session messages: %v", err)
	}
	if len(sessionResults) != 1 || sessionResults[0].EvidencePointer == "" {
		t.Fatalf("unexpected session search results: %+v", sessionResults)
	}

	entry, err := store.SaveMemoryEntry(ctx, domain.MemoryEntry{
		Kind:            domain.MemoryKindTaskLessons,
		WorkspacePath:   "/repo",
		SourceSessionID: "S1",
		SourceRunID:     "R1",
		Title:           "Snapshot lesson",
		Content:         "Keep frozen memory snapshots stable during a run.",
		Status:          domain.MemoryStatusActive,
		Fingerprint:     "lesson-1",
	})
	if err != nil {
		t.Fatalf("save memory entry: %v", err)
	}
	if entry.ID == 0 {
		t.Fatalf("expected memory id, got %+v", entry)
	}

	memoryResults, err := store.SearchMemoryEntries(ctx, "/repo", "frozen memory", 5)
	if err != nil {
		t.Fatalf("search memory entries: %v", err)
	}
	if len(memoryResults) != 1 || memoryResults[0].Entry.ID != entry.ID {
		t.Fatalf("unexpected memory search results: %+v", memoryResults)
	}

	skill, err := store.SaveSkill(ctx, domain.SkillRecord{
		SkillID:       "skill-1",
		WorkspacePath: "/repo",
		Name:          "memory-snapshot",
		Summary:       "Apply the frozen memory snapshot workflow.",
		Content:       "1. Build a frozen snapshot.\n2. Reuse it for the entire run.",
		Status:        domain.SkillStatusDraft,
		Fingerprint:   "skill-fp",
	})
	if err != nil {
		t.Fatalf("save skill: %v", err)
	}
	if skill.Version != 1 {
		t.Fatalf("expected initial skill version, got %+v", skill)
	}

	listedSkills, err := store.ListSkills(ctx, "/repo", "", 5)
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}
	if len(listedSkills) != 1 || listedSkills[0].SkillID != "skill-1" {
		t.Fatalf("unexpected listed skills: %+v", listedSkills)
	}

	loadedSkill, err := store.GetSkill(ctx, "skill-1")
	if err != nil {
		t.Fatalf("get skill: %v", err)
	}
	if loadedSkill.Name != "memory-snapshot" {
		t.Fatalf("unexpected loaded skill: %+v", loadedSkill)
	}

	outcome, err := store.SaveTaskOutcome(ctx, domain.TaskOutcome{
		SessionID:    "S1",
		RunID:        "R1",
		Title:        "Memory workflow",
		Status:       domain.TaskStatusCompleted,
		Summary:      "Persist the successful snapshot pattern.",
		Fingerprint:  "outcome-fp",
		ChangedPaths: []string{"internal/orchestrator/loop.go"},
		ChecksRun:    []string{"go_test"},
	})
	if err != nil {
		t.Fatalf("save task outcome: %v", err)
	}
	if outcome.ID == 0 {
		t.Fatalf("expected task outcome id, got %+v", outcome)
	}

	outcomes, err := store.ListTaskOutcomesByFingerprint(ctx, "outcome-fp", 5)
	if err != nil {
		t.Fatalf("list task outcomes by fingerprint: %v", err)
	}
	if len(outcomes) != 1 || outcomes[0].RunID != "R1" {
		t.Fatalf("unexpected outcomes: %+v", outcomes)
	}
}
