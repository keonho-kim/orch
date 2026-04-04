package domain

import (
	"fmt"
	"strings"
	"time"
)

type Provider string

const (
	ProviderOllama  Provider = "ollama"
	ProviderVLLM    Provider = "vllm"
	ProviderGemini  Provider = "gemini"
	ProviderVertex  Provider = "vertex"
	ProviderBedrock Provider = "bedrock"
	ProviderClaude  Provider = "claude"
	ProviderAzure   Provider = "azure"
	ProviderChatGPT Provider = "chatgpt"
)

var providerOrder = []Provider{
	ProviderOllama,
	ProviderVLLM,
	ProviderGemini,
	ProviderVertex,
	ProviderBedrock,
	ProviderClaude,
	ProviderAzure,
	ProviderChatGPT,
}

func (p Provider) String() string {
	return string(p)
}

func (p Provider) DisplayName() string {
	return MustProviderCatalog(p).DisplayName
}

func ParseProvider(value string) (Provider, error) {
	normalized := Provider(strings.ToLower(strings.TrimSpace(value)))
	if _, ok := ProviderCatalogFor(normalized); ok {
		return normalized, nil
	}
	return "", fmt.Errorf("unsupported provider %q", value)
}

func Providers() []Provider {
	return append([]Provider(nil), providerOrder...)
}

type RunMode string

const (
	RunModeReact RunMode = "react"
	RunModePlan  RunMode = "plan"
)

func (m RunMode) String() string {
	return string(m)
}

func (m RunMode) DisplayName() string {
	switch m {
	case RunModeReact:
		return "ReAct Deep Agent"
	case RunModePlan:
		return "Plan"
	default:
		return strings.ToUpper(string(m))
	}
}

func ParseRunMode(value string) (RunMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(RunModeReact), "":
		return RunModeReact, nil
	case string(RunModePlan):
		return RunModePlan, nil
	default:
		return "", fmt.Errorf("unsupported run mode %q", value)
	}
}

type AgentRole string

const (
	AgentRoleGateway AgentRole = "gateway"
	AgentRoleWorker  AgentRole = "worker"
)

func (r AgentRole) String() string {
	return string(r)
}

func (r AgentRole) DisplayName() string {
	switch r {
	case AgentRoleWorker:
		return "Worker"
	case AgentRoleGateway, "":
		return "Gateway"
	default:
		return strings.ToUpper(string(r))
	}
}

func ParseAgentRole(value string) (AgentRole, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(AgentRoleGateway):
		return AgentRoleGateway, nil
	case string(AgentRoleWorker):
		return AgentRoleWorker, nil
	default:
		return "", fmt.Errorf("unsupported agent role %q", value)
	}
}

type RunStatus string

const (
	StatusIdle             RunStatus = "Idle"
	StatusRunning          RunStatus = "Running"
	StatusAwaitingApproval RunStatus = "Awaiting Approval"
	StatusCompleted        RunStatus = "Completed"
	StatusFailed           RunStatus = "Failed"
	StatusCancelled        RunStatus = "Cancelled"
)

type ApprovalPolicy string

const (
	ApprovalConfirmMutations ApprovalPolicy = "confirm_mutations"
)

type ProviderSettings struct {
	Endpoint  string `json:"endpoint"`
	Model     string `json:"model"`
	APIKey    string `json:"api_key,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

func (s ProviderSettings) NormalizedEndpoint() string {
	return strings.TrimRight(strings.TrimSpace(s.Endpoint), "/")
}

type ProviderCatalog struct {
	Ollama  ProviderSettings `json:"ollama"`
	VLLM    ProviderSettings `json:"vllm"`
	Gemini  ProviderSettings `json:"gemini"`
	Vertex  ProviderSettings `json:"vertex"`
	Bedrock ProviderSettings `json:"bedrock"`
	Claude  ProviderSettings `json:"claude"`
	Azure   ProviderSettings `json:"azure"`
	ChatGPT ProviderSettings `json:"chatgpt"`
}

type Settings struct {
	DefaultProvider   Provider        `json:"default_provider"`
	Providers         ProviderCatalog `json:"providers"`
	ApprovalPolicy    ApprovalPolicy  `json:"approval_policy"`
	SelfDrivingMode   bool            `json:"self_driving_mode"`
	ReactRalphIter    int             `json:"react_ralph_iter"`
	PlanRalphIter     int             `json:"plan_ralph_iter"`
	CompactThresholdK int             `json:"compact_threshold_k"`
}

func (s *Settings) Normalize() {
	for _, provider := range Providers() {
		spec := MustProviderCatalog(provider)
		current := s.Providers.Provider(provider)
		if strings.TrimSpace(current.Endpoint) == "" && strings.TrimSpace(spec.DefaultEndpoint) != "" {
			current.Endpoint = spec.DefaultEndpoint
		}
		if normalized, err := ParseReasoningValue(current.Reasoning); err == nil {
			current.Reasoning = normalized
		}
	}
	if s.ApprovalPolicy == "" {
		s.ApprovalPolicy = ApprovalConfirmMutations
	}
	if s.ReactRalphIter <= 0 {
		s.ReactRalphIter = 3
	}
	if s.PlanRalphIter <= 0 {
		s.PlanRalphIter = 3
	}
	if s.CompactThresholdK <= 0 {
		s.CompactThresholdK = 100
	}
}

func (s Settings) ConfigFor(provider Provider) ProviderSettings {
	return *s.Providers.Provider(provider)
}

func (s Settings) HasProviderModel(provider Provider) bool {
	return strings.TrimSpace(s.ConfigFor(provider).Model) != ""
}

func (s Settings) MissingProviderFields(provider Provider) []string {
	normalized := s
	normalized.Normalize()
	config := normalized.ConfigFor(provider)
	spec := MustProviderCatalog(provider)

	missing := make([]string, 0, 3)
	if spec.RequiresEndpoint && strings.TrimSpace(config.Endpoint) == "" {
		missing = append(missing, "Endpoint")
	}
	if spec.RequiresModel && strings.TrimSpace(config.Model) == "" {
		missing = append(missing, "Model")
	}
	if spec.RequiresAPIKey && strings.TrimSpace(config.APIKey) == "" {
		missing = append(missing, "API Key")
	}
	return missing
}

func (s Settings) IsProviderReady(provider Provider) bool {
	return len(s.MissingProviderFields(provider)) == 0
}

func (s Settings) ProviderConfigError(provider Provider) error {
	missing := s.MissingProviderFields(provider)
	if len(missing) == 0 {
		return nil
	}
	if len(missing) == 1 && missing[0] == "Model" {
		return fmt.Errorf("model is not configured for %s", provider.DisplayName())
	}
	return fmt.Errorf("%s is not configured; missing %s", provider.DisplayName(), strings.Join(missing, ", "))
}

type MessageHistoryEntry struct {
	ID        int64
	Prompt    string
	CreatedAt time.Time
}

type RunRecord struct {
	RunID          string
	SessionID      string
	Mode           RunMode
	AgentRole      AgentRole
	Provider       Provider
	Model          string
	Prompt         string
	CurrentTask    string
	Status         RunStatus
	WorkspacePath  string
	CurrentCwd     string
	RalphIteration int
	FinalOutput    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type UsageStats struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type SessionRecordType string

const (
	SessionRecordUser      SessionRecordType = "user"
	SessionRecordAssistant SessionRecordType = "assistant"
	SessionRecordTool      SessionRecordType = "tool"
	SessionRecordCompact   SessionRecordType = "compact"
	SessionRecordTitle     SessionRecordType = "title"
	SessionRecordContext   SessionRecordType = "context_snapshot"
)

const (
	TaskStatusQueued    = "queued"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

type SessionRecord struct {
	Seq             int64             `json:"seq"`
	SessionID       string            `json:"session_id"`
	RunID           string            `json:"run_id,omitempty"`
	Type            SessionRecordType `json:"type"`
	Content         string            `json:"content,omitempty"`
	Title           string            `json:"title,omitempty"`
	ToolName        string            `json:"tool_name,omitempty"`
	ToolCallID      string            `json:"tool_call_id,omitempty"`
	ThroughSeq      int64             `json:"through_seq,omitempty"`
	ContextSnapshot *ContextSnapshot  `json:"context_snapshot,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	Usage           UsageStats        `json:"usage,omitempty"`
}

type SessionMetadata struct {
	SessionID            string     `json:"session_id"`
	WorkspacePath        string     `json:"workspace_path"`
	ParentSessionID      string     `json:"parent_session_id,omitempty"`
	ParentRunID          string     `json:"parent_run_id,omitempty"`
	ParentTaskID         string     `json:"parent_task_id,omitempty"`
	WorkerRole           AgentRole  `json:"worker_role,omitempty"`
	TaskTitle            string     `json:"task_title,omitempty"`
	TaskContract         string     `json:"task_contract,omitempty"`
	TaskStatus           string     `json:"task_status,omitempty"`
	TaskSummary          string     `json:"task_summary,omitempty"`
	TaskChangedPaths     []string   `json:"task_changed_paths,omitempty"`
	TaskChecksRun        []string   `json:"task_checks_run,omitempty"`
	TaskEvidencePointers []string   `json:"task_evidence_pointers,omitempty"`
	TaskFollowups        []string   `json:"task_followups,omitempty"`
	TaskErrorKind        string     `json:"task_error_kind,omitempty"`
	Provider             Provider   `json:"provider"`
	Model                string     `json:"model"`
	Title                string     `json:"title"`
	Summary              string     `json:"summary"`
	StartedAt            time.Time  `json:"started_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	LastSequence         int64      `json:"last_sequence"`
	LastCompactedSeq     int64      `json:"last_compacted_seq"`
	TokensSinceCompact   int        `json:"tokens_since_compact"`
	TotalTokens          int        `json:"total_tokens"`
	FinalizePending      bool       `json:"finalize_pending"`
	FinalizedAt          *time.Time `json:"finalized_at,omitempty"`
	LastRunID            string     `json:"last_run_id,omitempty"`
}

type SubagentResult struct {
	ChildSessionID       string   `json:"child_session_id"`
	ChildRunID           string   `json:"child_run_id"`
	TaskID               string   `json:"task_id,omitempty"`
	TaskTitle            string   `json:"task_title,omitempty"`
	TaskStatus           string   `json:"task_status,omitempty"`
	WorkerRole           string   `json:"worker_role,omitempty"`
	Status               string   `json:"status"`
	TaskSummary          string   `json:"task_summary,omitempty"`
	TaskChangedPaths     []string `json:"task_changed_paths,omitempty"`
	TaskChecksRun        []string `json:"task_checks_run,omitempty"`
	TaskEvidencePointers []string `json:"task_evidence_pointers,omitempty"`
	TaskFollowups        []string `json:"task_followups,omitempty"`
	TaskErrorKind        string   `json:"task_error_kind,omitempty"`
	FinalOutput          string   `json:"final_output"`
	Truncated            bool     `json:"truncated"`
	Error                string   `json:"error,omitempty"`
}

type TaskView struct {
	TaskID               string     `json:"task_id,omitempty"`
	Title                string     `json:"title,omitempty"`
	Status               string     `json:"status,omitempty"`
	ParentSessionID      string     `json:"parent_session_id,omitempty"`
	ParentRunID          string     `json:"parent_run_id,omitempty"`
	ChildSessionID       string     `json:"child_session_id,omitempty"`
	ChildRunID           string     `json:"child_run_id,omitempty"`
	WorkerRole           string     `json:"worker_role,omitempty"`
	Provider             string     `json:"provider,omitempty"`
	Model                string     `json:"model,omitempty"`
	TaskSummary          string     `json:"task_summary,omitempty"`
	TaskChangedPaths     []string   `json:"task_changed_paths,omitempty"`
	TaskChecksRun        []string   `json:"task_checks_run,omitempty"`
	TaskEvidencePointers []string   `json:"task_evidence_pointers,omitempty"`
	TaskFollowups        []string   `json:"task_followups,omitempty"`
	TaskErrorKind        string     `json:"task_error_kind,omitempty"`
	FinalOutputExcerpt   string     `json:"final_output_excerpt,omitempty"`
	StartedAt            time.Time  `json:"started_at,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at,omitempty"`
	FinalizedAt          *time.Time `json:"finalized_at,omitempty"`
}

type ContextSnapshot struct {
	SessionID               string   `json:"session_id"`
	RunID                   string   `json:"run_id"`
	Provider                string   `json:"provider"`
	Model                   string   `json:"model"`
	WorkspacePath           string   `json:"workspace_path"`
	CurrentCwd              string   `json:"current_cwd"`
	CompactSummaryPresent   bool     `json:"compact_summary_present"`
	PostCompactRecordCount  int      `json:"post_compact_record_count"`
	InheritedSummaryPresent bool     `json:"inherited_summary_present"`
	InheritedRecordCount    int      `json:"inherited_record_count"`
	SelectedSkills          []string `json:"selected_skills,omitempty"`
	ResolvedReferenceCount  int      `json:"resolved_reference_count"`
	UserMemoryPresent       bool     `json:"user_memory_present"`
	ChatHistoryExcerptBytes int      `json:"chat_history_excerpt_bytes"`
	PlanCachePresent        bool     `json:"plan_cache_present"`
}

type RunEvent struct {
	RunID     string
	Kind      string
	Message   string
	CreatedAt time.Time
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type ExecRequest struct {
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	Cwd        string   `json:"cwd,omitempty"`
	TimeoutSec int      `json:"timeout_sec,omitempty"`
	Stdin      string   `json:"stdin,omitempty"`
}

type OTRequest struct {
	Op               string   `json:"op"`
	Path             string   `json:"path,omitempty"`
	StartLine        int      `json:"start_line,omitempty"`
	EndLine          int      `json:"end_line,omitempty"`
	NamePattern      string   `json:"name_pattern,omitempty"`
	ContentPattern   string   `json:"content_pattern,omitempty"`
	Content          string   `json:"content,omitempty"`
	Patch            string   `json:"patch,omitempty"`
	Check            string   `json:"check,omitempty"`
	TaskID           string   `json:"task_id,omitempty"`
	TaskTitle        string   `json:"task_title,omitempty"`
	TaskContract     string   `json:"task_contract,omitempty"`
	Message          string   `json:"message,omitempty"`
	Wait             bool     `json:"wait,omitempty"`
	WaitProvided     bool     `json:"-"`
	StatusFilter     string   `json:"status_filter,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	ChangedPaths     []string `json:"changed_paths,omitempty"`
	ChecksRun        []string `json:"checks_run,omitempty"`
	EvidencePointers []string `json:"evidence_pointers,omitempty"`
	Followups        []string `json:"followups,omitempty"`
	ErrorKind        string   `json:"error_kind,omitempty"`
}

type SubagentTask struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Contract      string `json:"contract"`
	StartFilePath string `json:"start_file_path,omitempty"`
}

type ToolResult struct {
	ToolCallID string
	Name       string
	Content    string
}

type ApprovalRequest struct {
	RunID  string
	Call   ToolCall
	Reason string
}

type PlanCache struct {
	SourceRunID string
	Content     string
	UpdatedAt   time.Time
}
