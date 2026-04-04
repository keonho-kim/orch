package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func TestStorePersistsProviderHistoryAndRuns(t *testing.T) {
	t.Parallel()

	store := openTestStore(t)
	ctx := context.Background()
	assertDefaultProviderPersistence(t, store, ctx)
	assertMessageHistoryPersistence(t, store, ctx)
	assertRunPersistence(t, store, ctx)
	assertPlanCachePersistence(t, store, ctx)
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "state.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func assertDefaultProviderPersistence(t *testing.T, store *Store, ctx context.Context) {
	t.Helper()
	if err := store.SaveDefaultProvider(ctx, domain.ProviderOllama); err != nil {
		t.Fatalf("save default provider: %v", err)
	}
	settings, err := store.LoadSettings(ctx)
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings.DefaultProvider != domain.ProviderOllama {
		t.Fatalf("unexpected default provider: %q", settings.DefaultProvider)
	}
}

func assertMessageHistoryPersistence(t *testing.T, store *Store, ctx context.Context) {
	t.Helper()
	if err := store.AddMessageHistory(ctx, "implement login"); err != nil {
		t.Fatalf("add message history: %v", err)
	}
	history, err := store.ListMessageHistory(ctx, 10)
	if err != nil {
		t.Fatalf("list message history: %v", err)
	}
	if len(history) != 1 || history[0].Prompt != "implement login" {
		t.Fatalf("unexpected message history: %+v", history)
	}
}

func assertRunPersistence(t *testing.T, store *Store, ctx context.Context) {
	t.Helper()
	record := testRunRecord()
	if err := store.CreateRun(ctx, record); err != nil {
		t.Fatalf("create run: %v", err)
	}
	record.Status = domain.StatusCompleted
	record.CurrentTask = "Completed"
	record.FinalOutput = "done"
	if err := store.UpdateRun(ctx, record); err != nil {
		t.Fatalf("update run: %v", err)
	}
	runs, err := store.ListRuns(ctx, 10)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected one run, got %d", len(runs))
	}
	if runs[0].Status != domain.StatusCompleted {
		t.Fatalf("unexpected run status: %s", runs[0].Status)
	}
	if runs[0].Mode != domain.RunModeReact {
		t.Fatalf("unexpected run mode: %s", runs[0].Mode)
	}
	if runs[0].CurrentCwd != "/tmp/test-workspace" {
		t.Fatalf("unexpected current cwd: %s", runs[0].CurrentCwd)
	}
	nextID, err := store.NextRunID(ctx)
	if err != nil {
		t.Fatalf("next run id: %v", err)
	}
	if nextID != "R2" {
		t.Fatalf("unexpected next run id: %s", nextID)
	}
}

func assertPlanCachePersistence(t *testing.T, store *Store, ctx context.Context) {
	t.Helper()
	cache := domain.PlanCache{
		SourceRunID: "R1",
		Content:     "Plan content",
	}
	if err := store.SavePlanCache(ctx, cache); err != nil {
		t.Fatalf("save plan cache: %v", err)
	}
	loadedCache, err := store.LoadPlanCache(ctx)
	if err != nil {
		t.Fatalf("load plan cache: %v", err)
	}
	if loadedCache.SourceRunID != "R1" || loadedCache.Content != "Plan content" {
		t.Fatalf("unexpected plan cache: %+v", loadedCache)
	}
}

func testRunRecord() domain.RunRecord {
	now := time.Now()
	return domain.RunRecord{
		RunID:          "R1",
		Mode:           domain.RunModeReact,
		Provider:       domain.ProviderOllama,
		Model:          "qwen2.5-coder",
		Prompt:         "implement login",
		CurrentTask:    "Thinking",
		Status:         domain.StatusRunning,
		WorkspacePath:  "/tmp/test-workspace",
		CurrentCwd:     "/tmp/test-workspace",
		RalphIteration: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
