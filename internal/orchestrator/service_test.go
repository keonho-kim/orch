package orchestrator

import (
	"context"
	"testing"
	"time"

	"orch/domain"
)

func TestResolveApprovalUpdatesRunState(t *testing.T) {
	t.Parallel()

	service := &Service{
		ctx:     context.Background(),
		runs:    map[string]*runState{},
		updates: make(chan UIEvent, 8),
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
				Call:  domain.ToolCall{Name: "exec"},
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
