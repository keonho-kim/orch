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
	DefaultProvider Provider        `json:"default_provider"`
	Providers       ProviderCatalog `json:"providers"`
	ApprovalPolicy  ApprovalPolicy  `json:"approval_policy"`
	SelfDrivingMode bool            `json:"self_driving_mode"`
	AutoTranslate   bool            `json:"auto_translate"`
	ReactRalphIter  int             `json:"react_ralph_iter"`
	PlanRalphIter   int             `json:"plan_ralph_iter"`
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

type HistoryEntry struct {
	ID        int64
	Prompt    string
	CreatedAt time.Time
}

type RunRecord struct {
	RunID          string
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

type RunEvent struct {
	RunID      string
	Kind       string
	Message    string
	CreatedAt  time.Time
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
