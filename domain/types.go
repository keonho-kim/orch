package domain

import (
	"fmt"
	"strings"
	"time"
)

type Provider string

const (
	ProviderOllama Provider = "ollama"
	ProviderVLLM   Provider = "vllm"
)

func (p Provider) String() string {
	return string(p)
}

func (p Provider) DisplayName() string {
	switch p {
	case ProviderOllama:
		return "Ollama"
	case ProviderVLLM:
		return "vLLM"
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
	default:
		return "", fmt.Errorf("unsupported provider %q", value)
	}
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
	Ollama ProviderSettings `json:"ollama"`
	VLLM   ProviderSettings `json:"vllm"`
}

type Settings struct {
	DefaultProvider   Provider        `json:"default_provider"`
	Providers         ProviderCatalog `json:"providers"`
	ApprovalPolicy    ApprovalPolicy  `json:"approval_policy"`
	SelfDrivingMode   bool            `json:"self_driving_mode"`
	AutoTranslate     bool            `json:"auto_translate"`
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
	default:
		return ProviderSettings{}
	}
}

func (s Settings) HasProviderModel(provider Provider) bool {
	return strings.TrimSpace(s.ConfigFor(provider).Model) != ""
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
)

type SessionRecord struct {
	Seq        int64             `json:"seq"`
	SessionID  string            `json:"session_id"`
	RunID      string            `json:"run_id,omitempty"`
	Type       SessionRecordType `json:"type"`
	Content    string            `json:"content,omitempty"`
	Title      string            `json:"title,omitempty"`
	ToolName   string            `json:"tool_name,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ThroughSeq int64             `json:"through_seq,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	Usage      UsageStats        `json:"usage,omitempty"`
}

type SessionMetadata struct {
	SessionID          string     `json:"session_id"`
	WorkspacePath      string     `json:"workspace_path"`
	ParentSessionID    string     `json:"parent_session_id,omitempty"`
	ParentRunID        string     `json:"parent_run_id,omitempty"`
	Provider           Provider   `json:"provider"`
	Model              string     `json:"model"`
	Title              string     `json:"title"`
	Summary            string     `json:"summary"`
	StartedAt          time.Time  `json:"started_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastSequence       int64      `json:"last_sequence"`
	LastCompactedSeq   int64      `json:"last_compacted_seq"`
	TokensSinceCompact int        `json:"tokens_since_compact"`
	TotalTokens        int        `json:"total_tokens"`
	FinalizePending    bool       `json:"finalize_pending"`
	FinalizedAt        *time.Time `json:"finalized_at,omitempty"`
	LastRunID          string     `json:"last_run_id,omitempty"`
}

type SubagentResult struct {
	ChildSessionID string `json:"child_session_id"`
	ChildRunID     string `json:"child_run_id"`
	Status         string `json:"status"`
	FinalOutput    string `json:"final_output"`
	Truncated      bool   `json:"truncated"`
	Error          string `json:"error,omitempty"`
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
