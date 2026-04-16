package knowledge

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/keonho-kim/orch/domain"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
)

func TestLearnFromTaskCreatesLessonAndProcedureSkill(t *testing.T) {
	t.Parallel()

	store, err := sqlitestore.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	service := NewService(store)
	ctx := context.Background()

	for index := 0; index < 3; index++ {
		if _, err := store.SaveTaskOutcome(ctx, domain.TaskOutcome{
			SessionID:        "S1",
			RunID:            "R1",
			Title:            "Frozen memory workflow",
			Status:           domain.TaskStatusCompleted,
			Summary:          "Reuse the same memory snapshot across Ralph iterations.",
			ChangedPaths:     []string{"internal/orchestrator/loop.go"},
			ChecksRun:        []string{"go_test"},
			EvidencePointers: []string{"ot-pointer://session/S1?lines=1"},
			Fingerprint:      OutcomeFingerprint("Frozen memory workflow", []string{"internal/orchestrator/loop.go"}, []string{"go_test"}),
		}); err != nil {
			t.Fatalf("seed task outcome: %v", err)
		}
	}

	err = service.LearnFromTask(ctx, LearningInput{
		WorkspacePath: "/repo",
		Prompt:        "Please respond in Korean and keep code comments in English.",
		SessionMeta: domain.SessionMetadata{
			SessionID:     "S1",
			WorkspacePath: "/repo",
		},
		Outcome: domain.TaskOutcome{
			SessionID:        "S1",
			RunID:            "R1",
			Title:            "Frozen memory workflow",
			Status:           domain.TaskStatusCompleted,
			Summary:          "Reuse the same memory snapshot across Ralph iterations.",
			ChangedPaths:     []string{"internal/orchestrator/loop.go"},
			ChecksRun:        []string{"go_test"},
			EvidencePointers: []string{"ot-pointer://session/S1?lines=1"},
			Fingerprint:      OutcomeFingerprint("Frozen memory workflow", []string{"internal/orchestrator/loop.go"}, []string{"go_test"}),
		},
		SessionSummary: "This Go repository prefers gofmt and go test validation.",
	})
	if err != nil {
		t.Fatalf("learn from task: %v", err)
	}

	memories, err := service.MemorySearch(ctx, "/repo", "response language", 10)
	if err != nil {
		t.Fatalf("memory search: %v", err)
	}
	if len(memories) == 0 {
		t.Fatalf("expected learned user profile memory, got %+v", memories)
	}

	skills, err := service.ListSkills(ctx, "/repo", "", 10)
	if err != nil {
		t.Fatalf("list skills: %v", err)
	}
	if len(skills) == 0 || skills[0].ReplayCount < 3 {
		t.Fatalf("expected promoted procedure skill, got %+v", skills)
	}
}
