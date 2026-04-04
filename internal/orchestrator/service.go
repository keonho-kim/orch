package orchestrator

import (
	"context"
	"sync"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
	"github.com/keonho-kim/orch/internal/tooling"
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

	eventMu       sync.RWMutex
	maintenanceMu sync.Mutex

	mu               sync.RWMutex
	settings         domain.Settings
	configState      config.ConfigState
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

type bootstrapData struct {
	configState    config.ConfigState
	settings       domain.Settings
	history        []domain.MessageHistoryEntry
	planCache      domain.PlanCache
	inheritedCtx   session.Context
	currentSession domain.SessionMetadata
	runRecords     []domain.RunRecord
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
	service.sessions = session.NewService(session.NewManager(paths.SessionsDir))
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

func (s *Service) SaveSettings(settings domain.Settings) error {
	if err := config.SaveSettings(s.paths, settings); err != nil {
		return err
	}
	return s.reloadConfigState("Settings saved.")
}

func (s *Service) ConfigState() config.ConfigState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configState
}

func (s *Service) reloadConfigState(message string) error {
	configState, err := config.LoadConfigState(s.paths)
	if err != nil {
		return err
	}
	if configState.Settings.DefaultProvider != "" {
		if err := s.store.SaveDefaultProvider(s.ctx, configState.Settings.DefaultProvider); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.settings = configState.Settings
	s.configState = configState
	s.mu.Unlock()

	s.publish(UIEvent{Type: "config_updated", Message: message})
	return nil
}
