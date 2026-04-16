package domain

import (
	"fmt"
	"strings"
	"time"
)

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
