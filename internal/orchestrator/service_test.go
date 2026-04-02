package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
)

func TestResolveApprovalUpdatesRunState(t *testing.T) {
	t.Parallel()

	service := &Service{
		ctx:  context.Background(),
		runs: map[string]*runState{},
	}
	service.runs["R1"] = &runState{
		record: domain.RunRecord{
			RunID:       "R1",
			Status:      domain.StatusAwaitingApproval,
			CurrentTask: "Awaiting approval",
			UpdatedAt:   time.Now(),
		},
		pending: &approvalState{
			request: domain.ApprovalRequest{
				RunID: "R1",
				Call:  domain.ToolCall{Name: "ot"},
			},
			response: make(chan bool, 1),
		},
	}

	if err := service.ResolveApproval("R1", true); err != nil {
		t.Fatalf("resolve approval: %v", err)
	}

	state := service.runs["R1"]
	if state.pending != nil {
		t.Fatal("expected pending approval to be cleared")
	}
	if state.record.Status != domain.StatusRunning {
		t.Fatalf("unexpected run status: %s", state.record.Status)
	}
}

func TestActiveRunCount(t *testing.T) {
	t.Parallel()

	service := &Service{
		runs: map[string]*runState{
			"R1": {
				record: domain.RunRecord{RunID: "R1", Status: domain.StatusRunning},
				cancel: func() {},
			},
			"R2": {
				record: domain.RunRecord{RunID: "R2", Status: domain.StatusCompleted},
			},
		},
	}

	if got := service.ActiveRunCount(); got != 1 {
		t.Fatalf("unexpected active run count: %d", got)
	}
}

func TestOpenNewSessionResetsCurrentSessionAndRuns(t *testing.T) {
	t.Parallel()

	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	service := &Service{
		ctx:      context.Background(),
		paths:    paths,
		sessions: session.NewService(session.NewManager(paths.SessionsDir), nil),
		runs: map[string]*runState{
			"R1": {
				record: domain.RunRecord{RunID: "R1", Status: domain.StatusCompleted},
			},
		},
		settings: domain.Settings{
			DefaultProvider: domain.ProviderOllama,
			Providers: domain.ProviderCatalog{
				Ollama: domain.ProviderSettings{Model: "model"},
			},
		},
		currentSession: domain.SessionMetadata{
			SessionID: "S1",
		},
		currentRun: "R1",
		lastPrompt: "hello",
	}

	if err := service.OpenNewSession(); err != nil {
		t.Fatalf("open new session: %v", err)
	}
	if service.currentSession.SessionID == "" || service.currentSession.SessionID == "S1" {
		t.Fatalf("expected a new current session, got %+v", service.currentSession)
	}
	if service.currentRun != "" || service.lastPrompt != "" {
		t.Fatalf("expected current run state to reset, got run=%q prompt=%q", service.currentRun, service.lastPrompt)
	}
	if len(service.runs) != 0 {
		t.Fatalf("expected runs to reset, got %+v", service.runs)
	}
}
