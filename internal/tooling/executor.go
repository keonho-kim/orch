package tooling

import (
	"context"
	"fmt"

	"github.com/keonho-kim/orch/domain"
)

const maxCommandOutputBytes = 64000

type Executor struct {
	ot              *OTRunner
	contextSnapshot func(domain.RunRecord) (domain.ContextSnapshot, error)
	listTasks       func(domain.RunRecord, string) ([]domain.TaskView, error)
	getTask         func(domain.RunRecord, string) (domain.TaskView, error)
}

type Execution struct {
	Output               string
	RequiresApproval     bool
	Reason               string
	NextCwd              string
	TerminalStatus       domain.RunStatus
	TerminalMessage      string
	TaskSummary          string
	TaskChangedPaths     []string
	TaskChecksRun        []string
	TaskEvidencePointers []string
	TaskFollowups        []string
	TaskErrorKind        string
}

func NewExecutor() *Executor {
	return &Executor{ot: NewOTRunner()}
}

func (e *Executor) SetStateResolvers(
	contextSnapshot func(domain.RunRecord) (domain.ContextSnapshot, error),
	listTasks func(domain.RunRecord, string) ([]domain.TaskView, error),
	getTask func(domain.RunRecord, string) (domain.TaskView, error),
) {
	e.contextSnapshot = contextSnapshot
	e.listTasks = listTasks
	e.getTask = getTask
}

func (e *Executor) DecodeOTRequest(call domain.ToolCall) (domain.OTRequest, error) {
	return decodeOTRequest(call)
}

func (e *Executor) Review(workspaceRoot string, record domain.RunRecord, env []string, settings domain.Settings, call domain.ToolCall) (Execution, error) {
	request, err := decodeOTRequest(call)
	if err != nil {
		return Execution{}, err
	}
	if err := validateOTRequest(record, request); err != nil {
		return Execution{}, err
	}

	requiresApproval, reason, err := classifyOTApproval(record, settings, request)
	if err != nil {
		return Execution{}, err
	}
	return Execution{
		RequiresApproval: requiresApproval,
		Reason:           reason,
	}, nil
}

func (e *Executor) Execute(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, call domain.ToolCall) (Execution, error) {
	request, err := decodeOTRequest(call)
	if err != nil {
		return Execution{}, err
	}
	if err := validateOTRequest(record, request); err != nil {
		return Execution{}, err
	}
	spec, ok := lookupOTOperation(request.Op)
	if !ok {
		return Execution{}, fmt.Errorf("unsupported ot op %q", request.Op)
	}
	return spec.Execute(e, ctx, workspaceRoot, record, env, request)
}
