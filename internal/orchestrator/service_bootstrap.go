package orchestrator

import (
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
)

func (s *Service) bootstrap(options BootOptions) error {
	resolvedSettings, err := config.LoadResolvedSettings(s.paths)
	if err != nil {
		return err
	}
	settings := resolvedSettings.Effective

	historyEntries, err := s.store.ListMessageHistory(s.ctx, historyLimit)
	if err != nil {
		return err
	}

	planCache, err := s.store.LoadPlanCache(s.ctx)
	if err != nil {
		return err
	}

	inheritedCtx, err := s.loadInheritedContext(options)
	if err != nil {
		return err
	}

	var currentSession domain.SessionMetadata
	if strings.TrimSpace(options.RestoreSessionID) != "" {
		currentSession, err = s.sessions.LoadMetadata(options.RestoreSessionID)
		if err != nil {
			return err
		}
	} else {
		currentSession, err = s.sessions.Create(
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
			return err
		}
	}
	if currentSession.WorkerRole == "" {
		currentSession.WorkerRole = s.agentRole
	}

	runRecords, err := s.store.ListRunsBySession(s.ctx, currentSession.SessionID, runListLimit)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings = settings
	s.configState = resolvedSettings
	s.activePlan = planCache
	s.currentSession = currentSession
	s.inheritedCtx = inheritedCtx
	s.history = make([]string, 0, len(historyEntries))
	for _, entry := range historyEntries {
		s.history = append(s.history, entry.Prompt)
	}

	for _, record := range runRecords {
		record := record
		if record.Status == domain.StatusRunning || record.Status == domain.StatusAwaitingApproval {
			record.Status = domain.StatusCancelled
			record.CurrentTask = "Interrupted on previous shutdown"
			record.FinalOutput = appendFinalNote(record.FinalOutput, "Interrupted on previous shutdown.")
			_ = s.store.UpdateRun(s.ctx, record)
		}
		s.runs[record.RunID] = &runState{record: record, output: record.FinalOutput, draft: record.FinalOutput}
		s.runs[record.RunID].record.AgentRole = s.agentRole
	}

	if len(runRecords) > 0 {
		s.currentRun = runRecords[0].RunID
		s.lastPrompt = runRecords[0].Prompt
	}

	return nil
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
