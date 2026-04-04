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

func (e *Executor) executeCheck(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	checkName := strings.TrimSpace(request.Check)
	resolvedPath, err := resolveCheckPath(workspaceRoot, record, request.Path)
	if err != nil {
		return Execution{}, err
	}

	var execRequest domain.ExecRequest
	switch checkName {
	case "go_test":
		execRequest = domain.ExecRequest{Command: "go", Args: []string{"test", goPackagePattern(workspaceRoot, resolvedPath)}}
	case "go_vet":
		execRequest = domain.ExecRequest{Command: "go", Args: []string{"vet", goPackagePattern(workspaceRoot, resolvedPath)}}
	case "golangci_lint":
		target := "./..."
		if strings.TrimSpace(request.Path) != "" {
			target = goPackagePattern(workspaceRoot, resolvedPath)
		}
		execRequest = domain.ExecRequest{Command: "golangci-lint", Args: []string{"run", target}}
	default:
		return Execution{}, fmt.Errorf("unsupported check %q", request.Check)
	}

	output, err := runExternal(ctx, workspaceRoot, record, env, execRequest)
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}
