package tooling

import (
	"context"

	"github.com/keonho-kim/orch/domain"
)

func (e *Executor) executeWrite(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	output, err := runOTBinary(ctx, workspaceRoot, record, env, domain.ExecRequest{
		Args:  []string{"write", "--path", normalizeWorkspacePath(request.Path), "--from-stdin"},
		Stdin: request.Content,
	})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func (e *Executor) executePatch(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	output, err := runOTBinary(ctx, workspaceRoot, record, env, domain.ExecRequest{
		Args:  []string{"patch", "--from-stdin"},
		Stdin: request.Patch,
	})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}
