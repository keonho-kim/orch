package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/session"
	"github.com/keonho-kim/orch/internal/workspace"
)

type promptSubmission struct {
	trimmed        string
	record         domain.RunRecord
	env            []string
	selectedSkills []selectedSkill
	resolvedRefs   string
}

func (s *Service) SubmitPromptMode(prompt string, mode domain.RunMode) (string, error) {
	submission, err := s.preparePromptSubmission(prompt, mode)
	if err != nil {
		return "", err
	}
	if err := s.store.CreateRun(s.ctx, submission.record); err != nil {
		return "", err
	}
	runCtx, currentSession := s.registerPromptSubmission(submission)
	if err := s.finalizePromptSubmission(currentSession, submission.record, submission.trimmed); err != nil {
		return "", err
	}
	s.publish(UIEvent{Type: "run_created", RunID: submission.record.RunID, SessionID: currentSession.SessionID, Message: "Run started."})
	go s.executeRun(runCtx, submission.record.RunID)
	return submission.record.RunID, nil
}

func (s *Service) preparePromptSubmission(prompt string, mode domain.RunMode) (promptSubmission, error) {
	trimmed, parsedMode, settings, providerSettings, err := s.validatePromptSubmission(prompt, mode)
	if err != nil {
		return promptSubmission{}, err
	}
	if err := s.store.AddMessageHistory(s.ctx, trimmed); err != nil {
		return promptSubmission{}, err
	}
	runID, err := s.store.NextRunID(s.ctx)
	if err != nil {
		return promptSubmission{}, err
	}
	provisioned, selectedSkills, resolvedReferences, err := s.preparePromptWorkspace(trimmed)
	if err != nil {
		return promptSubmission{}, err
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
	return promptSubmission{
		trimmed:        trimmed,
		record:         record,
		env:            provisioned.Env,
		selectedSkills: selectedSkills,
		resolvedRefs:   resolvedReferences,
	}, nil
}

func (s *Service) validatePromptSubmission(prompt string, mode domain.RunMode) (string, domain.RunMode, domain.Settings, domain.ProviderSettings, error) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "", "", domain.Settings{}, domain.ProviderSettings{}, fmt.Errorf("prompt is required")
	}
	parsedMode, err := domain.ParseRunMode(mode.String())
	if err != nil {
		return "", "", domain.Settings{}, domain.ProviderSettings{}, err
	}
	settings := s.Snapshot().Settings
	if settings.DefaultProvider == "" {
		return "", "", domain.Settings{}, domain.ProviderSettings{}, fmt.Errorf("default provider is not configured")
	}
	if err := settings.ProviderConfigError(settings.DefaultProvider); err != nil {
		return "", "", domain.Settings{}, domain.ProviderSettings{}, err
	}
	return trimmed, parsedMode, settings, settings.ConfigFor(settings.DefaultProvider), nil
}

func (s *Service) preparePromptWorkspace(trimmed string) (workspace.ProvisionedWorkspace, []selectedSkill, string, error) {
	provisioned, err := workspace.Provision(
		s.paths.TestWorkspace,
		s.paths.BootstrapAssets,
		os.Environ(),
		nil,
	)
	if err != nil {
		return workspace.ProvisionedWorkspace{}, nil, "", err
	}
	selectedSkills, err := resolveSelectedSkills(provisioned.Root, trimmed)
	if err != nil {
		return workspace.ProvisionedWorkspace{}, nil, "", err
	}
	resolvedReferences, err := s.references.Resolve(provisioned.Root, provisioned.Root, trimmed)
	if err != nil {
		return workspace.ProvisionedWorkspace{}, nil, "", err
	}
	return provisioned, selectedSkills, resolvedReferences, nil
}

func (s *Service) registerPromptSubmission(submission promptSubmission) (context.Context, domain.SessionMetadata) {
	runCtx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.history = append([]string{submission.trimmed}, s.history...)
	s.lastPrompt = submission.trimmed
	s.currentRun = submission.record.RunID
	s.runs[submission.record.RunID] = &runState{
		record:         submission.record,
		env:            submission.env,
		cancel:         cancel,
		selectedSkills: submission.selectedSkills,
		resolvedRefs:   submission.resolvedRefs,
	}
	currentSession := s.currentSession
	s.mu.Unlock()
	return runCtx, currentSession
}

func (s *Service) finalizePromptSubmission(currentSession domain.SessionMetadata, record domain.RunRecord, prompt string) error {
	taskStatus := ""
	if s.agentRole == domain.AgentRoleWorker {
		taskStatus = domain.TaskStatusRunning
	}
	updated, err := s.sessions.MarkRunStarted(currentSession, session.RunStartUpdate{
		Provider:   record.Provider,
		Model:      record.Model,
		RunID:      record.RunID,
		WorkerRole: s.agentRole,
		TaskStatus: taskStatus,
		UpdatedAt:  record.CreatedAt,
	})
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	if err := s.appendSessionUser(record.RunID, prompt); err != nil {
		return err
	}
	go s.runChatHistoryUserSummary(updated, record.RunID, prompt)
	return nil
}

func (s *Service) DiscoverOllama(ctx context.Context, baseURL string) (string, []string, error) {
	models, normalized, err := adapters.ListOllamaModels(ctx, baseURL)
	if err != nil {
		return "", nil, err
	}
	return normalized, models, nil
}
