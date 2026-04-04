package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func (s *Service) RunRecord(runID string) (domain.RunRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.runs[runID]
	if !ok {
		return domain.RunRecord{}, false
	}

	return state.record, true
}

func (s *Service) RunActive(runID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.runs[runID]
	if !ok {
		return false
	}
	return state.cancel != nil && !isTerminalStatus(state.record.Status)
}

func (s *Service) RunOutput(runID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if state, ok := s.runs[runID]; ok {
		return state.output
	}
	return ""
}

func (s *Service) ResolveApproval(runID string, approved bool) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok || state.pending == nil {
		s.mu.Unlock()
		return fmt.Errorf("run %s has no pending approval", runID)
	}
	response := state.pending.response
	request := state.pending.request
	state.pending = nil
	state.record.Status = domain.StatusRunning
	state.record.CurrentTask = "Resuming after approval"
	record := state.record
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return err
	}

	select {
	case response <- approved:
	default:
	}

	if approved {
		s.publish(ServiceEvent{Type: "approval_resolved", RunID: runID, Message: "Approved tool execution."})
	} else {
		s.publish(ServiceEvent{Type: "approval_resolved", RunID: runID, Message: fmt.Sprintf("Denied tool execution: %s.", request.Call.Name)})
	}

	return nil
}

func (s *Service) ActiveRunCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, state := range s.runs {
		if state.cancel != nil && !isTerminalStatus(state.record.Status) {
			count++
		}
	}
	return count
}

func (s *Service) ShutdownAll() error {
	s.mu.Lock()
	active := make([]*runState, 0, len(s.runs))
	for _, state := range s.runs {
		if state.cancel == nil || isTerminalStatus(state.record.Status) {
			continue
		}
		active = append(active, state)
		state.record.Status = domain.StatusCancelled
		state.record.CurrentTask = "Cancelled by /exit"
		state.record.FinalOutput = appendFinalNote(state.output, "Cancelled by /exit.")
		state.record.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	var failures []string
	for _, state := range active {
		state.cancel()
		if err := s.persistRun(state.record); err != nil {
			failures = append(failures, err.Error())
		}
	}

	if len(active) > 0 {
		s.publish(ServiceEvent{Type: "snapshot", Message: fmt.Sprintf("Cancelled %d active run(s).", len(active))})
	}

	if len(failures) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *Service) saveActivePlan(cache domain.PlanCache) error {
	if s.store != nil {
		if err := s.store.SavePlanCache(s.ctx, cache); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.activePlan = cache
	s.mu.Unlock()
	s.publish(ServiceEvent{Type: "snapshot", Message: "Plan cache updated."})
	return nil
}

func isTerminalStatus(status domain.RunStatus) bool {
	switch status {
	case domain.StatusCompleted, domain.StatusFailed, domain.StatusCancelled:
		return true
	default:
		return false
	}
}

func appendFinalNote(existing string, note string) string {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return existing
	}
	if strings.TrimSpace(existing) == "" {
		return trimmed
	}
	return strings.TrimRight(existing, "\n") + "\n" + trimmed
}

func (s *Service) persistRun(record domain.RunRecord) error {
	if s.store == nil {
		return nil
	}
	return s.store.UpdateRun(s.ctx, record)
}

func (s *Service) setRunIteration(runID string, iteration int) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.record.RalphIteration = iteration
	state.record.UpdatedAt = time.Now()
	record := state.record
	s.mu.Unlock()
	if err := s.persistRun(record); err != nil {
		return err
	}
	s.publish(ServiceEvent{Type: "run_updated", RunID: runID})
	return nil
}

func (s *Service) setRunCwd(runID string, cwd string) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.record.CurrentCwd = cwd
	state.record.UpdatedAt = time.Now()
	record := state.record
	s.mu.Unlock()
	if err := s.persistRun(record); err != nil {
		return err
	}
	s.publish(ServiceEvent{Type: "run_updated", RunID: runID, Message: "Working directory updated."})
	return nil
}

func (s *Service) setRunDraft(runID string, draft string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.runs[runID]
	if !ok {
		return
	}
	if strings.TrimSpace(draft) == "" {
		return
	}
	state.draft = draft
}
