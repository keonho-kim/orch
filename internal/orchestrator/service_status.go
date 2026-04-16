package orchestrator

import (
	"fmt"
	"sort"

	"github.com/keonho-kim/orch/domain"
)

func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	historyCopy := append([]string(nil), s.history...)
	runs := make([]domain.RunRecord, 0, len(s.runs))
	for _, state := range s.runs {
		runs = append(runs, state.record)
	}
	sort.Slice(runs, func(i int, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].RunID > runs[j].RunID
		}
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	snapshot := Snapshot{
		Settings:       s.settings,
		MessageHistory: historyCopy,
		Runs:           runs,
		CurrentRunID:   s.currentRun,
		LastPrompt:     s.lastPrompt,
		ActivePlan:     s.activePlan,
		CurrentSession: s.currentSession,
	}

	if state, ok := s.runs[s.currentRun]; ok {
		snapshot.CurrentOutput = state.output
		snapshot.CurrentThinking = state.thinking
		if state.pending != nil {
			request := state.pending.request
			snapshot.PendingApproval = &request
		}
	}

	return snapshot
}

func (s *Service) Status() Status {
	snapshot := s.Snapshot()
	status := Status{
		CurrentSession: snapshot.CurrentSession,
		CurrentRunID:   snapshot.CurrentRunID,
		Provider:       snapshot.Settings.DefaultProvider.String(),
		ActiveRunCount: s.ActiveRunCount(),
	}
	if snapshot.Settings.DefaultProvider != "" {
		status.Model = snapshot.Settings.ConfigFor(snapshot.Settings.DefaultProvider).Model
	}
	if snapshot.PendingApproval != nil {
		request := *snapshot.PendingApproval
		status.PendingApproval = &request
	}
	return status
}

func (s *Service) RunSnapshot(runID string) (RunSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.runs[runID]
	if !ok {
		return RunSnapshot{}, fmt.Errorf("run %s not found", runID)
	}
	snapshot := RunSnapshot{
		Record:   state.record,
		Output:   state.output,
		Thinking: state.thinking,
	}
	if state.pending != nil {
		request := state.pending.request
		snapshot.PendingApproval = &request
	}
	return snapshot, nil
}

func (s *Service) ListSessions(limit int) ([]domain.SessionMetadata, error) {
	return s.sessions.ListSessions(limit)
}

func (s *Service) LatestSessionID() (string, error) {
	return s.sessions.LatestSessionID()
}

func (s *Service) CurrentContextSnapshot() (domain.ContextSnapshot, error) {
	s.mu.RLock()
	sessionID := s.currentSession.SessionID
	runID := s.currentRun
	s.mu.RUnlock()
	return s.sessions.LatestContextSnapshot(sessionID, runID)
}

func (s *Service) ListCurrentTasks(statusFilter string) ([]domain.TaskView, error) {
	s.mu.RLock()
	sessionID := s.currentSession.SessionID
	s.mu.RUnlock()
	return s.sessions.ListTasks(sessionID, "", statusFilter)
}

func (s *Service) GetCurrentTask(taskID string) (domain.TaskView, error) {
	s.mu.RLock()
	sessionID := s.currentSession.SessionID
	s.mu.RUnlock()
	return s.sessions.GetTask(sessionID, taskID)
}

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
