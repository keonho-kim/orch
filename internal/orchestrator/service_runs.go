package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/workspace"
)

func (s *Service) SubmitPromptMode(prompt string, mode domain.RunMode) (string, error) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "", fmt.Errorf("prompt is required")
	}
	parsedMode, err := domain.ParseRunMode(mode.String())
	if err != nil {
		return "", err
	}

	snapshot := s.Snapshot()
	settings := snapshot.Settings
	if settings.DefaultProvider == "" {
		return "", fmt.Errorf("default provider is not configured")
	}
	if err := settings.ProviderConfigError(settings.DefaultProvider); err != nil {
		return "", err
	}
	providerSettings := settings.ConfigFor(settings.DefaultProvider)

	if err := s.store.AddMessageHistory(s.ctx, trimmed); err != nil {
		return "", err
	}

	runID, err := s.store.NextRunID(s.ctx)
	if err != nil {
		return "", err
	}

	provisioned, err := workspace.Provision(
		s.paths.TestWorkspace,
		s.paths.BootstrapAssets,
		filepath.Dir(s.paths.OTToolsAssets),
		s.toolBaseEnv(s.agentRole),
		settings.SecretEnvNames(),
	)
	if err != nil {
		return "", err
	}
	selectedSkills, err := resolveSelectedSkills(provisioned.Root, trimmed)
	if err != nil {
		return "", err
	}
	resolvedReferences, err := s.references.Resolve(provisioned.Root, provisioned.Root, trimmed)
	if err != nil {
		return "", err
	}
	memorySnapshot := domain.MemorySnapshot{}
	if s.knowledge != nil {
		memorySnapshot, err = s.knowledge.BuildFrozenSnapshot(s.ctx, chooseNonEmpty(snapshot.CurrentSession.WorkspacePath, s.paths.RepoRoot), trimmed)
		if err != nil {
			return "", err
		}
	}

	now := time.Now()
	record := domain.RunRecord{
		RunID:         runID,
		SessionID:     s.currentSessionID(),
		Mode:          parsedMode,
		AgentRole:     s.agentRole,
		Provider:      settings.DefaultProvider,
		Model:         providerSettings.Model,
		Prompt:        trimmed,
		CurrentTask:   "Preparing run",
		Status:        domain.StatusRunning,
		WorkspacePath: provisioned.Root,
		CurrentCwd:    provisioned.Root,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.store.CreateRun(s.ctx, record); err != nil {
		return "", err
	}

	runCtx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.history = append([]string{trimmed}, s.history...)
	s.lastPrompt = trimmed
	s.currentRun = runID
	s.currentSession.Provider = settings.DefaultProvider
	s.currentSession.Model = providerSettings.Model
	s.currentSession.UpdatedAt = now
	s.currentSession.LastRunID = runID
	s.currentSession.WorkerRole = s.agentRole
	if s.agentRole == domain.AgentRoleWorker {
		s.currentSession.TaskStatus = "running"
	}
	s.runs[runID] = &runState{
		record:         record,
		env:            provisioned.Env,
		cancel:         cancel,
		selectedSkills: selectedSkills,
		resolvedRefs:   resolvedReferences,
		memorySnapshot: memorySnapshot,
	}
	currentSession := s.currentSession
	s.mu.Unlock()

	if err := s.sessions.SaveMetadata(currentSession); err != nil {
		return "", err
	}
	if err := s.appendSessionUser(runID, trimmed); err != nil {
		return "", err
	}
	if s.agentRole == domain.AgentRoleWorker {
		if err := s.updateCurrentSessionTaskStatus("running"); err != nil {
			return "", err
		}
	}
	go s.runChatHistoryUserSummary(currentSession, runID, trimmed)

	s.publish(UIEvent{Type: "run_created", RunID: runID, SessionID: currentSession.SessionID, Message: "Run started."})
	go s.executeRun(runCtx, runID)

	return runID, nil
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
		s.publish(UIEvent{Type: "approval_resolved", RunID: runID, Message: "Approved tool execution."})
	} else {
		s.publish(UIEvent{Type: "approval_resolved", RunID: runID, Message: fmt.Sprintf("Denied tool execution: %s.", request.Call.Name)})
	}
	s.emitHookEvent(HookEvent{
		Type:      hookApprovalResolved,
		SessionID: record.SessionID,
		RunID:     runID,
		RunRecord: record,
	})

	return nil
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
		s.publish(UIEvent{Type: "snapshot", Message: fmt.Sprintf("Cancelled %d active run(s).", len(active))})
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
	s.publish(UIEvent{Type: "snapshot", Message: "Plan cache updated."})
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
	s.publish(UIEvent{Type: "run_updated", RunID: runID})
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
	s.publish(UIEvent{Type: "run_updated", RunID: runID, Message: "Working directory updated."})
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
