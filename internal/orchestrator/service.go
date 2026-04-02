package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
	"github.com/keonho-kim/orch/internal/tooling"
	"github.com/keonho-kim/orch/internal/workspace"
)

const (
	historyLimit = 200
	runListLimit = 100
)

type Service struct {
	ctx       context.Context
	store     *sqlitestore.Store
	tooling   *tooling.Executor
	paths     config.Paths
	agentRole domain.AgentRole
	clients   map[domain.Provider]adapters.Client
	sessions  *session.Service

	eventMu sync.RWMutex

	mu               sync.RWMutex
	settings         domain.Settings
	configState      config.ResolvedSettings
	history          []string
	runs             map[string]*runState
	currentRun       string
	lastPrompt       string
	activePlan       domain.PlanCache
	currentSession   domain.SessionMetadata
	inheritedCtx     session.Context
	references       *referenceResolver
	subscribers      map[int]chan ServiceEvent
	nextSubscriberID int
}

type runState struct {
	record         domain.RunRecord
	output         string
	thinking       string
	draft          string
	env            []string
	cancel         context.CancelFunc
	pending        *approvalState
	selectedSkills []selectedSkill
	resolvedRefs   string
}

type approvalState struct {
	request  domain.ApprovalRequest
	response chan bool
}

type selectedSkill struct {
	Name    string
	Content string
	Path    string
}

type Snapshot struct {
	Settings        domain.Settings
	MessageHistory  []string
	Runs            []domain.RunRecord
	CurrentRunID    string
	CurrentOutput   string
	CurrentThinking string
	PendingApproval *domain.ApprovalRequest
	LastPrompt      string
	ActivePlan      domain.PlanCache
	CurrentSession  domain.SessionMetadata
}

type Status struct {
	CurrentSession  domain.SessionMetadata  `json:"current_session"`
	CurrentRunID    string                  `json:"current_run_id,omitempty"`
	Provider        string                  `json:"provider,omitempty"`
	Model           string                  `json:"model,omitempty"`
	PendingApproval *domain.ApprovalRequest `json:"pending_approval,omitempty"`
	ActiveRunCount  int                     `json:"active_run_count"`
}

type RunSnapshot struct {
	Record          domain.RunRecord        `json:"record"`
	Output          string                  `json:"output,omitempty"`
	Thinking        string                  `json:"thinking,omitempty"`
	PendingApproval *domain.ApprovalRequest `json:"pending_approval,omitempty"`
}

type BootOptions struct {
	RestoreSessionID     string
	ParentSessionID      string
	ParentRunID          string
	ParentTaskID         string
	TaskTitle            string
	TaskContract         string
	TaskStatus           string
	AgentRole            domain.AgentRole
	InheritParentContext bool
}

func NewService(
	ctx context.Context,
	store *sqlitestore.Store,
	executor *tooling.Executor,
	paths config.Paths,
	options BootOptions,
) (*Service, error) {
	service := &Service{
		ctx:       ctx,
		store:     store,
		tooling:   executor,
		paths:     paths,
		agentRole: normalizeAgentRole(options.AgentRole),
		clients: map[domain.Provider]adapters.Client{
			domain.ProviderOllama:  adapters.NewOllamaClient(),
			domain.ProviderVLLM:    adapters.NewVLLMClient(),
			domain.ProviderGemini:  adapters.NewGeminiClient(),
			domain.ProviderVertex:  adapters.NewVertexClient(),
			domain.ProviderBedrock: adapters.NewBedrockClient(),
			domain.ProviderClaude:  adapters.NewClaudeClient(),
			domain.ProviderAzure:   adapters.NewAzureClient(),
			domain.ProviderChatGPT: adapters.NewChatGPTClient(),
		},
		runs:        make(map[string]*runState),
		subscribers: make(map[int]chan ServiceEvent),
	}
	service.references = newReferenceResolver()
	service.sessions = session.NewService(session.NewManager(paths.SessionsDir), maintenanceRunner{
		clients: service.clients,
		settings: func(provider domain.Provider) domain.ProviderSettings {
			service.mu.RLock()
			defer service.mu.RUnlock()
			return service.settings.ConfigFor(provider)
		},
	})
	if executor != nil {
		executor.SetStateResolvers(service.contextSnapshotForRun, service.listTasksForRun, service.getTaskForRun)
	}

	if err := service.bootstrap(options); err != nil {
		return nil, err
	}

	return service, nil
}

func normalizeAgentRole(role domain.AgentRole) domain.AgentRole {
	if parsed, err := domain.ParseAgentRole(role.String()); err == nil {
		return parsed
	}
	return domain.AgentRoleGateway
}

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
		go s.finalizeSessionByID(oldSessionID)
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

func (s *Service) SaveSettings(settings domain.Settings) error {
	return s.SaveScopeSettings(config.ScopeProject, config.ScopeSettingsFromDomainSettings(settings))
}

func (s *Service) ConfigState() config.ResolvedSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configState
}

func (s *Service) SaveScopeSettings(scope config.Scope, settings config.ScopeSettings) error {
	if err := s.migrateLegacyDefaultProvider(); err != nil {
		return err
	}
	if err := config.SaveScopeSettings(s.paths, scope, settings); err != nil {
		return err
	}
	return s.reloadResolvedSettings(fmt.Sprintf("Settings saved to %s scope.", strings.Title(string(scope))))
}

func (s *Service) UnsetScopeSettings(scope config.Scope, keys []config.SettingKey) error {
	if err := s.migrateLegacyDefaultProvider(); err != nil {
		return err
	}
	if err := config.UnsetScopeSettings(s.paths, scope, keys); err != nil {
		return err
	}
	return s.reloadResolvedSettings(fmt.Sprintf("Settings updated in %s scope.", strings.Title(string(scope))))
}

func (s *Service) reloadResolvedSettings(message string) error {
	resolvedSettings, err := config.LoadResolvedSettings(s.paths)
	if err != nil {
		return err
	}
	if resolvedSettings.Effective.DefaultProvider != "" {
		if err := s.store.SaveDefaultProvider(s.ctx, resolvedSettings.Effective.DefaultProvider); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.settings = resolvedSettings.Effective
	s.configState = resolvedSettings
	s.mu.Unlock()

	s.publish(UIEvent{Type: "config_updated", Message: message})
	return nil
}

func (s *Service) migrateLegacyDefaultProvider() error {
	s.mu.RLock()
	resolved := s.configState
	s.mu.RUnlock()
	if _, ok := resolved.Sources[config.KeyDefaultProvider]; ok {
		return nil
	}

	stored, err := s.store.LoadSettings(s.ctx)
	if err != nil || stored.DefaultProvider == "" {
		return nil
	}

	userSettings, err := config.LoadScopeSettings(s.paths, config.ScopeUser)
	if err != nil {
		return err
	}
	provider := stored.DefaultProvider.String()
	userSettings.DefaultProvider = &provider
	return config.SaveScopeSettings(s.paths, config.ScopeUser, userSettings)
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
		os.Environ(),
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
		s.publish(UIEvent{Type: "approval_resolved", RunID: runID, Message: "Approved tool execution."})
	} else {
		s.publish(UIEvent{Type: "approval_resolved", RunID: runID, Message: fmt.Sprintf("Denied tool execution: %s.", request.Call.Name)})
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
