package session

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func TestAppendAndLoadLatestContextSnapshot(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := NewService(manager)
	meta, err := manager.Create("/repo", domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	snapshot := domain.ContextSnapshot{
		SessionID:              meta.SessionID,
		RunID:                  "R1",
		Provider:               domain.ProviderOllama.String(),
		Model:                  "model",
		WorkspacePath:          "/repo/test-workspace",
		CurrentCwd:             "/repo/test-workspace",
		CompactSummaryPresent:  true,
		PostCompactRecordCount: 3,
	}
	if _, err := service.AppendContextSnapshot(meta, "R1", snapshot); err != nil {
		t.Fatalf("append context snapshot: %v", err)
	}

	loaded, err := service.LatestContextSnapshot(meta.SessionID, "R1")
	if err != nil {
		t.Fatalf("load latest context snapshot: %v", err)
	}
	if loaded.RunID != "R1" || loaded.PostCompactRecordCount != 3 {
		t.Fatalf("unexpected context snapshot: %+v", loaded)
	}
}

func TestListTasksAndGetTaskDeriveFromSessionMetadata(t *testing.T) {
	t.Parallel()

	manager := NewManager(filepath.Join(t.TempDir(), ".orch", "sessions"))
	service := NewService(manager)
	parent, err := manager.Create("/repo", domain.ProviderOllama, "parent-model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create parent session: %v", err)
	}

	childStarted := time.Now().Add(time.Second)
	child, err := manager.Create("/repo", domain.ProviderChatGPT, "gpt-4.1", childStarted, parent.SessionID, "R1", "task-1", domain.AgentRoleWorker, "Inspect tests", "Check the failing tests", domain.TaskStatusRunning)
	if err != nil {
		t.Fatalf("create child session: %v", err)
	}
	child.LastRunID = "R9"
	child.TaskSummary = "Inspected the failing tests and identified the regression."
	child.TaskChangedPaths = []string{"pkg/a.go", "pkg/a_test.go"}
	child.TaskChecksRun = []string{"go_test"}
	child.TaskEvidencePointers = []string{FormatOTPointerForSession(child.SessionID, []int64{1, 2})}
	child.TaskFollowups = []string{"Update regression coverage"}
	child.TaskStatus = domain.TaskStatusCompleted
	child.TaskErrorKind = ""
	if err := manager.SaveMetadata(child); err != nil {
		t.Fatalf("save child metadata: %v", err)
	}
	if err := manager.AppendRecord(child.SessionID, domain.SessionRecord{
		Seq:       1,
		SessionID: child.SessionID,
		RunID:     "R9",
		Type:      domain.SessionRecordAssistant,
		Content:   "Detailed child output",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("append child assistant record: %v", err)
	}

	tasks, err := service.ListTasks(parent.SessionID, "R1", "")
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
	task := tasks[0]
	if task.TaskID != "task-1" || task.ChildSessionID != child.SessionID || task.Status != domain.TaskStatusCompleted {
		t.Fatalf("unexpected task view: %+v", task)
	}
	if task.FinalOutputExcerpt != "Inspected the failing tests and identified the regression." {
		t.Fatalf("unexpected final output excerpt: %+v", task)
	}

	loaded, err := service.GetTask(parent.SessionID, "task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if loaded.TaskID != "task-1" || loaded.ChildRunID != "R9" {
		t.Fatalf("unexpected loaded task: %+v", loaded)
	}
}
