package tooling

import (
	"context"
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func (e *Executor) executeRead(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	normalizedPath, err := normalizeWorkspaceRelativePath(workspaceRoot, record, request.Path)
	if err != nil {
		return Execution{}, err
	}
	args := []string{"read", "--path", normalizedPath}
	if request.StartLine > 0 {
		args = append(args, "--start", fmt.Sprintf("%d", request.StartLine))
	}
	if request.EndLine > 0 {
		args = append(args, "--end", fmt.Sprintf("%d", request.EndLine))
	}
	output, err := runOTBinary(ctx, workspaceRoot, record, env, domain.ExecRequest{Args: args})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func (e *Executor) executeList(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	normalizedPath, err := normalizeWorkspaceRelativePath(workspaceRoot, record, request.Path)
	if err != nil {
		return Execution{}, err
	}
	args := []string{"list", "--path", normalizedPath}
	output, err := runOTBinary(ctx, workspaceRoot, record, env, domain.ExecRequest{Args: args})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func (e *Executor) executeSearch(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	normalizedPath, err := normalizeWorkspaceRelativePath(workspaceRoot, record, request.Path)
	if err != nil {
		return Execution{}, err
	}
	args := []string{"search", "--path", normalizedPath}
	if strings.TrimSpace(request.NamePattern) != "" {
		args = append(args, "--name", strings.TrimSpace(request.NamePattern))
	}
	if strings.TrimSpace(request.ContentPattern) != "" {
		args = append(args, "--content", strings.TrimSpace(request.ContentPattern))
	}
	output, err := runOTBinary(ctx, workspaceRoot, record, env, domain.ExecRequest{Args: args})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}
