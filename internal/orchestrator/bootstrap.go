package orchestrator

import (
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
)

func (s *Service) bootstrap(options BootOptions) error {
	data, err := s.loadBootstrapData(options)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = data.settings
	s.configState = data.configState
	s.activePlan = data.planCache
	s.currentSession = data.currentSession
	s.inheritedCtx = data.inheritedCtx
	s.history = make([]string, 0, len(data.history))
	for _, entry := range data.history {
		s.history = append(s.history, entry.Prompt)
	}
	s.restoreRunStateLocked(data.runRecords)
	return nil
}

func (s *Service) loadBootstrapData(options BootOptions) (bootstrapData, error) {
	configState, err := config.LoadConfigState(s.paths)
	if err != nil {
		return bootstrapData{}, err
	}
	historyEntries, err := s.store.ListMessageHistory(s.ctx, historyLimit)
	if err != nil {
		return bootstrapData{}, err
	}
	planCache, err := s.store.LoadPlanCache(s.ctx)
	if err != nil {
		return bootstrapData{}, err
	}
	inheritedCtx, err := s.loadInheritedContext(options)
	if err != nil {
		return bootstrapData{}, err
	}
	currentSession, err := s.resolveBootstrapSession(configState.Settings, options)
	if err != nil {
		return bootstrapData{}, err
	}
	runRecords, err := s.store.ListRunsBySession(s.ctx, currentSession.SessionID, runListLimit)
	if err != nil {
		return bootstrapData{}, err
	}
	return bootstrapData{
		configState:    configState,
		settings:       configState.Settings,
		history:        historyEntries,
		planCache:      planCache,
		inheritedCtx:   inheritedCtx,
		currentSession: currentSession,
		runRecords:     runRecords,
	}, nil
}

func (s *Service) resolveBootstrapSession(settings domain.Settings, options BootOptions) (domain.SessionMetadata, error) {
	if strings.TrimSpace(options.RestoreSessionID) != "" {
		currentSession, err := s.sessions.LoadMetadata(options.RestoreSessionID)
		if err != nil {
			return domain.SessionMetadata{}, err
		}
		if currentSession.WorkerRole == "" {
			currentSession.WorkerRole = s.agentRole
		}
		return currentSession, nil
	}
	currentSession, err := s.sessions.Create(
		s.paths.RepoRoot,
		settings.DefaultProvider,
		settings.ConfigFor(settings.DefaultProvider).Model,
		time.Now(),
		options.ParentSessionID,
		options.ParentRunID,
		options.ParentTaskID,
		s.agentRole,
		options.TaskTitle,
		options.TaskContract,
		options.TaskStatus,
	)
	if err != nil {
		return domain.SessionMetadata{}, err
	}
	if currentSession.WorkerRole == "" {
		currentSession.WorkerRole = s.agentRole
	}
	return currentSession, nil
}

func (s *Service) restoreRunStateLocked(runRecords []domain.RunRecord) {
	for _, record := range runRecords {
		restored := s.normalizeRestoredRunRecord(record)
		s.runs[restored.RunID] = &runState{record: restored, output: restored.FinalOutput, draft: restored.FinalOutput}
		s.runs[restored.RunID].record.AgentRole = s.agentRole
	}
	if len(runRecords) == 0 {
		s.currentRun = ""
		s.lastPrompt = ""
		return
	}
	s.currentRun = runRecords[0].RunID
	s.lastPrompt = runRecords[0].Prompt
}

func (s *Service) normalizeRestoredRunRecord(record domain.RunRecord) domain.RunRecord {
	if record.Status == domain.StatusRunning || record.Status == domain.StatusAwaitingApproval {
		record.Status = domain.StatusCancelled
		record.CurrentTask = "Interrupted on previous shutdown"
		record.FinalOutput = appendFinalNote(record.FinalOutput, "Interrupted on previous shutdown.")
		_ = s.store.UpdateRun(s.ctx, record)
	}
	return record
}

func (s *Service) loadInheritedContext(options BootOptions) (session.Context, error) {
	if !options.InheritParentContext || strings.TrimSpace(options.ParentSessionID) == "" {
		return session.Context{}, nil
	}

	meta, err := s.sessions.LoadMetadata(options.ParentSessionID)
	if err != nil {
		return session.Context{}, err
	}
	return s.sessions.Context(meta)
}
