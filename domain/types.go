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
	switch p {
	case ProviderOllama:
		return "Ollama"
	case ProviderVLLM:
		return "vLLM"
	case ProviderGemini:
		return "Gemini"
	case ProviderVertex:
		return "Vertex"
	case ProviderBedrock:
		return "Bedrock"
	case ProviderClaude:
		return "Claude"
	case ProviderAzure:
		return "Azure"
	case ProviderChatGPT:
		return "ChatGPT"
	default:
		return strings.ToUpper(string(p))
	}
}

func ParseProvider(value string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ProviderOllama):
		return ProviderOllama, nil
	case string(ProviderVLLM):
		return ProviderVLLM, nil
	case string(ProviderGemini):
		return ProviderGemini, nil
	case string(ProviderVertex):
		return ProviderVertex, nil
	case string(ProviderBedrock):
		return ProviderBedrock, nil
	case string(ProviderClaude):
		return ProviderClaude, nil
	case string(ProviderAzure):
		return ProviderAzure, nil
	case string(ProviderChatGPT):
		return ProviderChatGPT, nil
	default:
		return "", fmt.Errorf("unsupported provider %q", value)
	}
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
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	APIKeyEnv string `json:"api_key_env,omitempty"`
}

func (s ProviderSettings) NormalizedBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
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
	if strings.TrimSpace(s.Providers.Ollama.BaseURL) == "" {
		s.Providers.Ollama.BaseURL = "http://localhost:11434/v1"
	}
	if strings.TrimSpace(s.Providers.VLLM.BaseURL) == "" {
		s.Providers.VLLM.BaseURL = "http://localhost:8000/v1"
	}
	if strings.TrimSpace(s.Providers.VLLM.APIKeyEnv) == "" {
		s.Providers.VLLM.APIKeyEnv = "VLLM_API_KEY"
	}
	if strings.TrimSpace(s.Providers.Gemini.BaseURL) == "" {
		s.Providers.Gemini.BaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	if strings.TrimSpace(s.Providers.Gemini.APIKeyEnv) == "" {
		s.Providers.Gemini.APIKeyEnv = "GEMINI_API_KEY"
	}
	if strings.TrimSpace(s.Providers.Vertex.BaseURL) == "" {
		s.Providers.Vertex.BaseURL = "https://aiplatform.googleapis.com/v1"
	}
	if strings.TrimSpace(s.Providers.Vertex.APIKeyEnv) == "" {
		s.Providers.Vertex.APIKeyEnv = "GOOGLE_API_KEY"
	}
	if strings.TrimSpace(s.Providers.Bedrock.APIKeyEnv) == "" {
		s.Providers.Bedrock.APIKeyEnv = "AWS_BEARER_TOKEN_BEDROCK"
	}
	if strings.TrimSpace(s.Providers.Claude.BaseURL) == "" {
		s.Providers.Claude.BaseURL = "https://api.anthropic.com/v1"
	}
	if strings.TrimSpace(s.Providers.Claude.APIKeyEnv) == "" {
		s.Providers.Claude.APIKeyEnv = "ANTHROPIC_API_KEY"
	}
	if strings.TrimSpace(s.Providers.Azure.APIKeyEnv) == "" {
		s.Providers.Azure.APIKeyEnv = "AZURE_OPENAI_API_KEY"
	}
	if strings.TrimSpace(s.Providers.ChatGPT.BaseURL) == "" {
		s.Providers.ChatGPT.BaseURL = "https://api.openai.com/v1"
	}
	if strings.TrimSpace(s.Providers.ChatGPT.APIKeyEnv) == "" {
		s.Providers.ChatGPT.APIKeyEnv = "OPENAI_API_KEY"
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
	switch provider {
	case ProviderOllama:
		return s.Providers.Ollama
	case ProviderVLLM:
		return s.Providers.VLLM
	case ProviderGemini:
		return s.Providers.Gemini
	case ProviderVertex:
		return s.Providers.Vertex
	case ProviderBedrock:
		return s.Providers.Bedrock
	case ProviderClaude:
		return s.Providers.Claude
	case ProviderAzure:
		return s.Providers.Azure
	case ProviderChatGPT:
		return s.Providers.ChatGPT
	default:
		return ProviderSettings{}
	}
}

func (s Settings) HasProviderModel(provider Provider) bool {
	return strings.TrimSpace(s.ConfigFor(provider).Model) != ""
}

func (s Settings) MissingProviderFields(provider Provider) []string {
	normalized := s
	normalized.Normalize()
	config := normalized.ConfigFor(provider)

	missing := make([]string, 0, 3)
	if providerRequiresBaseURL(provider) && strings.TrimSpace(config.BaseURL) == "" {
		missing = append(missing, "Base URL")
	}
	if providerRequiresModel(provider) && strings.TrimSpace(config.Model) == "" {
		missing = append(missing, "Model")
	}
	if providerRequiresAPIKeyEnv(provider) && strings.TrimSpace(config.APIKeyEnv) == "" {
		missing = append(missing, "API Key Env")
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

func (s Settings) SecretEnvNames() []string {
	normalized := s
	normalized.Normalize()

	seen := make(map[string]struct{})
	names := make([]string, 0, len(providerOrder))
	for _, provider := range providerOrder {
		name := strings.TrimSpace(normalized.ConfigFor(provider).APIKeyEnv)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

func providerRequiresBaseURL(provider Provider) bool {
	switch provider {
	case ProviderBedrock, ProviderAzure:
		return true
	default:
		return false
	}
}

func providerRequiresModel(provider Provider) bool {
	switch provider {
	case ProviderOllama,
		ProviderVLLM,
		ProviderGemini,
		ProviderVertex,
		ProviderBedrock,
		ProviderClaude,
		ProviderAzure,
		ProviderChatGPT:
		return true
	default:
		return false
	}
}

func providerRequiresAPIKeyEnv(provider Provider) bool {
	switch provider {
	case ProviderGemini,
		ProviderVertex,
		ProviderBedrock,
		ProviderClaude,
		ProviderAzure,
		ProviderChatGPT:
		return true
	default:
		return false
	}
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
