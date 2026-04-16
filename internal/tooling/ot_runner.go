package tooling

import (
	"context"
	"fmt"
	"os"
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
type extraEnvResolver func() map[string]string

type OTRunner struct {
	prepareScriptEnv scriptEnvPreparer
	resolveExtraEnv  extraEnvResolver
}

type otInspection struct {
	Subcommand      string
	NormalizedArgs  []string
	WithinWorkspace bool
	Prompt          string
}

var supportedOTSubcommands = map[string]struct{}{
	"exec":     {},
	"list":     {},
	"patch":    {},
	"pointer":  {},
	"read":     {},
	"search":   {},
	"subagent": {},
	"write":    {},
}

func NewOTRunner() *OTRunner {
	return &OTRunner{}
}

func NewOTRunnerWithScriptEnvPreparer(preparer scriptEnvPreparer) *OTRunner {
	return &OTRunner{prepareScriptEnv: preparer}
}

func (r *OTRunner) SetExtraEnvResolver(resolver extraEnvResolver) {
	r.resolveExtraEnv = resolver
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

	scriptPath := resolveOTScriptPath(workspaceRoot, env, inspection.Subcommand)
	if _, err := os.Stat(scriptPath); err != nil {
		return "", fmt.Errorf("resolve ot subcommand %s: %w", inspection.Subcommand, err)
	}

	scriptRequest := domain.ExecRequest{
		Command:    "bash",
		Args:       append([]string{scriptPath}, inspection.NormalizedArgs...),
		Cwd:        request.Cwd,
		TimeoutSec: request.TimeoutSec,
		Stdin:      request.Stdin,
	}

	scriptEnv := append([]string(nil), env...)
	if r.resolveExtraEnv != nil {
		scriptEnv = mergeEnvValues(scriptEnv, r.resolveExtraEnv())
	}
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

func mergeEnvValues(base []string, overlay map[string]string) []string {
	next := append([]string(nil), base...)
	for key, value := range overlay {
		next = upsertEnvValue(next, key, value)
	}
	return next
}

func upsertEnvValue(env []string, key string, value string) []string {
	next := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		currentKey, _, ok := strings.Cut(entry, "=")
		if ok && currentKey == key {
			if !replaced {
				next = append(next, key+"="+value)
				replaced = true
			}
			continue
		}
		next = append(next, entry)
	}
	if !replaced {
		next = append(next, key+"="+value)
	}
	return next
}

func requiresEmbeddedOTHelpers(subcommand string) bool {
	switch strings.TrimSpace(subcommand) {
	case "patch", "search":
		return true
	default:
		return false
	}
}
