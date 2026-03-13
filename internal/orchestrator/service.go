package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"orch/domain"
	"orch/internal/adapters"
	"orch/internal/config"
	sqlitestore "orch/internal/store/sqlite"
	"orch/internal/tooling"
	"orch/internal/workspace"
)

const (
	historyLimit = 200
	runListLimit = 100
)

type Service struct {
	ctx     context.Context
	store   *sqlitestore.Store
	tooling *tooling.Executor
	paths   config.Paths
	clients map[domain.Provider]adapters.Client

	mu         sync.RWMutex
	settings   domain.Settings
	history    []string
	runs       map[string]*runState
	currentRun string
	lastPrompt string
	activePlan domain.PlanCache
	updates    chan UIEvent
}

type runState struct {
	record   domain.RunRecord
	output   string
	thinking string
	draft    string
	env      []string
	cancel   context.CancelFunc
	pending  *approvalState
}

type approvalState struct {
	request  domain.ApprovalRequest
	response chan bool
}

type UIEvent struct {
	RunID   string
	Message string
}

type Snapshot struct {
	Settings        domain.Settings
	History         []string
	Runs            []domain.RunRecord
	CurrentRunID    string
	CurrentOutput   string
	CurrentThinking string
	PendingApproval *domain.ApprovalRequest
	LastPrompt      string
	ActivePlan      domain.PlanCache
}

func NewService(
	ctx context.Context,
	store *sqlitestore.Store,
	executor *tooling.Executor,
	paths config.Paths,
) (*Service, error) {
	service := &Service{
		ctx:     ctx,
		store:   store,
		tooling: executor,
		paths:   paths,
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama: adapters.NewOllamaClient(),
			domain.ProviderVLLM:   adapters.NewVLLMClient(),
		},
		runs:    make(map[string]*runState),
		updates: make(chan UIEvent, 256),
	}

	if err := service.bootstrap(); err != nil {
		return nil, err
	}

	return service, nil
}

func (s *Service) bootstrap() error {
	settings, err := config.LoadSettings(s.paths)
	if err != nil {
		return err
	}
	if settings.DefaultProvider == "" {
		stored, err := s.store.LoadSettings(s.ctx)
		if err == nil && stored.DefaultProvider != "" {
			settings.DefaultProvider = stored.DefaultProvider
			if err := config.SaveSettings(s.paths, settings); err != nil {
				return err
			}
		}
	}

	historyEntries, err := s.store.ListHistory(s.ctx, historyLimit)
	if err != nil {
		return err
	}

	runRecords, err := s.store.ListRuns(s.ctx, runListLimit)
	if err != nil {
		return err
	}
	planCache, err := s.store.LoadPlanCache(s.ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.settings = settings
	s.activePlan = planCache
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
	}

	if len(runRecords) > 0 {
		s.currentRun = runRecords[0].RunID
		s.lastPrompt = runRecords[0].Prompt
	}

	return nil
}

func (s *Service) Events() <-chan UIEvent {
	return s.updates
}

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
		Settings:     s.settings,
		History:      historyCopy,
		Runs:         runs,
		CurrentRunID: s.currentRun,
		LastPrompt:   s.lastPrompt,
		ActivePlan:   s.activePlan,
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

func (s *Service) NeedsSettingsConfiguration() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.settings.DefaultProvider == "" {
		return true
	}
	return !s.settings.HasProviderModel(s.settings.DefaultProvider)
}

func (s *Service) SaveSettings(settings domain.Settings) error {
	settings.Normalize()
	if settings.DefaultProvider != "" {
		if _, err := domain.ParseProvider(settings.DefaultProvider.String()); err != nil {
			return err
		}
	}

	if err := config.SaveSettings(s.paths, settings); err != nil {
		return err
	}
	if settings.DefaultProvider != "" {
		if err := s.store.SaveDefaultProvider(s.ctx, settings.DefaultProvider); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()

	s.publish(UIEvent{Message: fmt.Sprintf("Settings saved. Default provider: %s.", settings.DefaultProvider.DisplayName())})
	return nil
}

func (s *Service) DiscoverOllama(ctx context.Context, baseURL string) (string, []string, error) {
	models, normalized, err := adapters.ListOllamaModels(ctx, baseURL)
	if err != nil {
		return "", nil, err
	}
	return normalized, models, nil
}

func (s *Service) SubmitPromptMode(prompt string, mode domain.RunMode) (string, error) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "", fmt.Errorf("prompt is required")
	}
	parsedMode, err := domain.ParseRunMode(mode.String())
	if err != nil {
		return "", err
	}

	settings := s.Snapshot().Settings
	if settings.DefaultProvider == "" {
		return "", fmt.Errorf("default provider is not configured")
	}

	providerSettings := settings.ConfigFor(settings.DefaultProvider)
	if strings.TrimSpace(providerSettings.Model) == "" {
		return "", fmt.Errorf("model is not configured for %s", settings.DefaultProvider.DisplayName())
	}

	if err := s.store.AddHistory(s.ctx, trimmed); err != nil {
		return "", err
	}

	runID, err := s.store.NextRunID(s.ctx)
	if err != nil {
		return "", err
	}

	provisioned, err := workspace.Provision(
		s.paths.TestWorkspace,
		s.paths.BootstrapAssets,
		filepath.Join(s.paths.RepoRoot, "PRODUCT.md"),
		os.Environ(),
	)
	if err != nil {
		return "", err
	}

	now := time.Now()
	record := domain.RunRecord{
		RunID:         runID,
		Mode:          parsedMode,
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
	s.runs[runID] = &runState{
		record: record,
		env:    provisioned.Env,
		cancel: cancel,
	}
	s.mu.Unlock()

	s.publish(UIEvent{RunID: runID, Message: "Run started."})
	go s.executeRun(runCtx, runID)

	return runID, nil
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
		s.publish(UIEvent{RunID: runID, Message: "Approved tool execution."})
	} else {
		s.publish(UIEvent{RunID: runID, Message: fmt.Sprintf("Denied tool execution: %s.", request.Call.Name)})
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
		s.publish(UIEvent{Message: fmt.Sprintf("Cancelled %d active run(s).", len(active))})
	}

	if len(failures) > 0 {
		return fmt.Errorf("shutdown errors: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (s *Service) publish(event UIEvent) {
	select {
	case s.updates <- event:
	default:
	}
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
	s.publish(UIEvent{Message: "Plan cache updated."})
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
	s.publish(UIEvent{RunID: runID})
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
	s.publish(UIEvent{RunID: runID, Message: "Working directory updated."})
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
