package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/session"
)

func (s *Service) RestoreSession(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return fmt.Errorf("session id is required")
	}
	if s.ActiveRunCount() > 0 {
		return fmt.Errorf("cannot restore a session while runs are active")
	}

	meta, err := s.sessions.LoadMetadata(sessionID)
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
	s.publish(UIEvent{Type: "session_restored", SessionID: sessionID, Message: fmt.Sprintf("Restored session %s.", sessionID)})
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

	newSession, err := s.sessions.Create(
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

	s.publish(UIEvent{Type: "session_opened", SessionID: newSession.SessionID, Message: fmt.Sprintf("Opened new session %s.", newSession.SessionID)})
	if strings.TrimSpace(oldSessionID) != "" && oldSessionID != newSession.SessionID {
		go func() {
			_ = s.finalizeSessionByID(oldSessionID)
		}()
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

func (s *Service) updateCurrentSessionTaskStatus(status string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		return nil
	}

	s.mu.Lock()
	meta := s.currentSession
	if meta.WorkerRole == "" {
		meta.WorkerRole = s.agentRole
	}
	meta.TaskStatus = status
	meta.UpdatedAt = time.Now()
	s.currentSession = meta
	s.mu.Unlock()

	return s.sessions.SaveMetadata(meta)
}
