package tooling

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

const (
	otScopeInside         = "inside"
	otScopeOutside        = "outside"
	otDisplayRelative     = "workspace-relative"
	otDisplayAbsolute     = "absolute"
	otSearchReasonOutside = "ot %s outside the workspace requires approval."
	subagentDepthEnv      = "ORCH_SUBAGENT_DEPTH"
	subagentPlaceholder   = "-"
)

type scriptEnvPreparer func([]string) ([]string, error)

type OTRunner struct {
	prepareScriptEnv scriptEnvPreparer
}

func NewOTRunner() *OTRunner {
	return &OTRunner{}
}

func NewOTRunnerWithScriptEnvPreparer(preparer scriptEnvPreparer) *OTRunner {
	return &OTRunner{prepareScriptEnv: preparer}
}

func (r *OTRunner) Run(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.ExecRequest) (string, error) {
	inspection, err := inspectOTRequest(workspaceRoot, record, request)
	if err != nil {
		return "", err
	}
	if inspection.Subcommand == "subagent" {
		return r.runSubagent(ctx, workspaceRoot, record, env, inspection)
	}
	if inspection.Subcommand == "pointer" {
		return r.runPointer(workspaceRoot, record, inspection)
	}

	scriptPath := filepath.Join(workspaceRoot, "tools", "ot", inspection.Subcommand+".sh")
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("resolve ot subcommand %s: %w", inspection.Subcommand, err)
	}

	scriptRequest := domain.ExecRequest{
		Command:    "bash",
		Args:       append([]string{filepath.ToSlash(filepath.Join("tools", "ot", inspection.Subcommand+".sh"))}, inspection.NormalizedArgs...),
		Cwd:        request.Cwd,
		TimeoutSec: request.TimeoutSec,
		Stdin:      request.Stdin,
	}

	scriptEnv := append([]string(nil), env...)
	if requiresEmbeddedOTHelpers(inspection.Subcommand) {
		if r.prepareScriptEnv == nil {
			if runtime.GOOS == "linux" {
				return "", fmt.Errorf("ot %s requires embedded helper preparation on linux", inspection.Subcommand)
			}
		} else {
			scriptEnv, err = r.prepareScriptEnv(scriptEnv)
			if err != nil {
				return "", err
			}
		}
	}

	return runExternal(ctx, workspaceRoot, record, scriptEnv, scriptRequest)
}
func baseEnv(workspaceRoot string, env []string) []string {
	base := append([]string(nil), env...)
	repoRoot := workspaceRoot
	if cwd, err := os.Getwd(); err == nil {
		repoRoot = cwd
	}
	base = append(base, "OT_WORKSPACE_ROOT="+workspaceRoot)
	base = append(base, "OT_REPO_ROOT="+repoRoot)
	base = append(base, "PATH="+prefixedPath(env))
	return base
}

func prefixedPath(env []string) string {
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key == "PATH" {
			return value
		}
	}

	path, err := exec.LookPath("bash")
	if err != nil {
		return ""
	}
	return filepath.Dir(path)
}

func requiresEmbeddedOTHelpers(subcommand string) bool {
	switch strings.TrimSpace(subcommand) {
	case "patch", "search":
		return true
	default:
		return false
	}
}
