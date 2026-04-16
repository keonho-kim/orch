package tooling

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

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

func resolveCheckPath(workspaceRoot string, record domain.RunRecord, path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return workspaceRoot, nil
	}
	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func goPackagePattern(workspaceRoot string, resolvedPath string) string {
	info, err := os.Stat(resolvedPath)
	if err == nil && !info.IsDir() {
		resolvedPath = filepath.Dir(resolvedPath)
	}

	rel, err := filepath.Rel(workspaceRoot, resolvedPath)
	if err != nil || rel == "." {
		return "./..."
	}
	return "./" + filepath.ToSlash(rel) + "/..."
}
