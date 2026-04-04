package orchestrator

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/session"
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
	return s.sessionManager.ListSessions(limit)
}

func (s *Service) LatestSessionID() (string, error) {
	return s.sessionManager.LatestSessionID()
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

func (s *Service) RestoreSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if s.ActiveRunCount() > 0 {
		return fmt.Errorf("cannot restore a session while runs are active")
	}

	meta, err := s.sessionManager.LoadMetadata(sessionID)
	if err != nil {
		return err
	}
	runRecords, err := s.store.ListRunsBySession(s.ctx, sessionID, runListLimit)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentSession = meta
	s.currentRun = ""
	s.lastPrompt = ""
	s.runs = make(map[string]*runState, len(runRecords))
	for _, record := range runRecords {
		record := record
		s.runs[record.RunID] = &runState{record: record, output: record.FinalOutput, draft: record.FinalOutput}
	}
	if len(runRecords) > 0 {
		s.currentRun = runRecords[0].RunID
		s.lastPrompt = runRecords[0].Prompt
	}
	s.publish(ServiceEvent{Type: "session_restored", SessionID: sessionID, Message: fmt.Sprintf("Restored session %s.", sessionID)})
	return nil
}

func (s *Service) OpenNewSession() error {
	if s.ActiveRunCount() > 0 {
		return fmt.Errorf("cannot open a new session while runs are active")
	}

	snapshot := s.Snapshot()
	settings := snapshot.Settings
	if settings.DefaultProvider == "" {
		return fmt.Errorf("default provider is not configured")
	}
	if err := settings.ProviderConfigError(settings.DefaultProvider); err != nil {
		return err
	}
	model := strings.TrimSpace(settings.ConfigFor(settings.DefaultProvider).Model)

	newSession, err := s.sessionManager.Create(
		s.paths.RepoRoot,
		settings.DefaultProvider,
		model,
		time.Now(),
		"",
		"",
		"",
		s.agentRole,
		"",
		"",
		"",
	)
	if err != nil {
		return err
	}

	oldSessionID := snapshot.CurrentSession.SessionID

	s.mu.Lock()
	s.currentSession = newSession
	s.currentRun = ""
	s.lastPrompt = ""
	s.inheritedCtx = session.Context{}
	s.runs = make(map[string]*runState)
	s.mu.Unlock()

	s.publish(ServiceEvent{Type: "session_opened", SessionID: newSession.SessionID, Message: fmt.Sprintf("Opened new session %s.", newSession.SessionID)})
	if strings.TrimSpace(oldSessionID) != "" && oldSessionID != newSession.SessionID {
		go func(sessionID string) {
			_ = s.finalizeSessionByID(sessionID)
		}(oldSessionID)
	}
	return nil
}

func (s *Service) currentSessionID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.currentSession.SessionID
}

func (s *Service) contextSnapshotForRun(record domain.RunRecord) (domain.ContextSnapshot, error) {
	return s.sessions.LatestContextSnapshot(record.SessionID, record.RunID)
}

func (s *Service) listTasksForRun(record domain.RunRecord, statusFilter string) ([]domain.TaskView, error) {
	return s.sessions.ListTasks(record.SessionID, "", statusFilter)
}

func (s *Service) getTaskForRun(record domain.RunRecord, taskID string) (domain.TaskView, error) {
	return s.sessions.GetTask(record.SessionID, taskID)
}

func (s *Service) NeedsSettingsConfiguration() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.settings.DefaultProvider == "" {
		return true
	}
	return !s.settings.IsProviderReady(s.settings.DefaultProvider)
}
