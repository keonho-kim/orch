package tooling

import (
	"context"
	"fmt"

	"github.com/keonho-kim/orch/domain"
)

func executeContextOperation(executor *Executor, _ context.Context, _ string, record domain.RunRecord, _ []string, _ domain.OTRequest) (Execution, error) {
	if executor.contextSnapshot == nil {
		return Execution{}, fmt.Errorf("context snapshot resolver is not configured")
	}
	snapshot, err := executor.contextSnapshot(record)
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: renderContextSnapshot(snapshot)}, nil
}

func executeTaskListOperation(executor *Executor, _ context.Context, _ string, record domain.RunRecord, _ []string, request domain.OTRequest) (Execution, error) {
	if executor.listTasks == nil {
		return Execution{}, fmt.Errorf("task list resolver is not configured")
	}
	tasks, err := executor.listTasks(record, request.StatusFilter)
	if err != nil {
		return Execution{}, err
	}
	output, err := renderJSON(tasks)
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func executeTaskGetOperation(executor *Executor, _ context.Context, _ string, record domain.RunRecord, _ []string, request domain.OTRequest) (Execution, error) {
	if executor.getTask == nil {
		return Execution{}, fmt.Errorf("task resolver is not configured")
	}
	task, err := executor.getTask(record, request.TaskID)
	if err != nil {
		return Execution{}, err
	}
	output, err := renderJSON(task)
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func (e *Executor) executeReadOp(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	return e.executeRead(ctx, workspaceRoot, record, env, request)
}

func (e *Executor) executeListOp(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	return e.executeList(ctx, workspaceRoot, record, env, request)
}

func (e *Executor) executeSearchOp(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	return e.executeSearch(ctx, workspaceRoot, record, env, request)
}

func (e *Executor) executeWriteOp(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	return e.executeWrite(ctx, workspaceRoot, record, env, request)
}

func (e *Executor) executePatchOp(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	return e.executePatch(ctx, workspaceRoot, record, env, request)
}

func (e *Executor) executeCheckOp(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	return e.executeCheck(ctx, workspaceRoot, record, env, request)
}

func executeDelegateOperation(executor *Executor, ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	wait := !request.WaitProvided || request.Wait
	output, err := executor.ot.RunDelegateTask(ctx, workspaceRoot, record, env, domain.SubagentTask{
		ID:       normalizeTaskID(request),
		Title:    request.TaskTitle,
		Contract: request.TaskContract,
	}, wait)
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}
