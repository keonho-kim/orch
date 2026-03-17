package tooling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

const maxCommandOutputBytes = 64000

type Executor struct {
	ot *OTRunner
}

type Execution struct {
	Output           string
	RequiresApproval bool
	Reason           string
	NextCwd          string
}

func NewExecutor() *Executor {
	return &Executor{ot: NewOTRunner()}
}

func (e *Executor) Review(workspaceRoot string, record domain.RunRecord, env []string, settings domain.Settings, call domain.ToolCall) (Execution, error) {
	request, err := decodeExecRequest(call)
	if err != nil {
		return Execution{}, err
	}

	requiresApproval, reason, err := classifyApproval(workspaceRoot, record, env, settings, request)
	if err != nil {
		return Execution{}, err
	}
	return Execution{
		RequiresApproval: requiresApproval,
		Reason:           reason,
	}, nil
}

func (e *Executor) Execute(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, call domain.ToolCall) (Execution, error) {
	request, err := decodeExecRequest(call)
	if err != nil {
		return Execution{}, err
	}

	if err := validateModeCommand(workspaceRoot, record, request); err != nil {
		return Execution{}, err
	}

	if request.Command == "cd" {
		nextCwd, err := resolveCommandPath(workspaceRoot, baseCwd(record), firstArg(request.Args))
		if err != nil {
			return Execution{}, err
		}
		return Execution{
			Output:  fmt.Sprintf("Changed directory to %s", displayRelativePath(workspaceRoot, nextCwd)),
			NextCwd: nextCwd,
		}, nil
	}

	switch request.Command {
	case "ot":
		output, err := e.ot.Run(ctx, workspaceRoot, record, env, request)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "bash":
		if err := validateCustomScript(workspaceRoot, request); err != nil {
			return Execution{}, err
		}
		output, err := runExternal(ctx, workspaceRoot, record, env, request)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	default:
		if !allowedDirectCommands()[request.Command] {
			return Execution{}, fmt.Errorf("command %q is not allowlisted", request.Command)
		}
		output, err := runExternal(ctx, workspaceRoot, record, env, request)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	}
}

func classifyApproval(workspaceRoot string, record domain.RunRecord, env []string, settings domain.Settings, request domain.ExecRequest) (bool, string, error) {
	if err := validateModeCommand(workspaceRoot, record, request); err != nil {
		return false, "", err
	}

	if record.Mode == domain.RunModePlan {
		return false, "", nil
	}

	if isBlockedDestructiveCommand(request) {
		return true, "rm and mv always require approval.", nil
	}

	switch request.Command {
	case "ot":
		return classifyOTApproval(workspaceRoot, record, env, settings, request)
	case "bash":
		if err := validateCustomScript(workspaceRoot, request); err != nil {
			return false, "", err
		}
		if settings.SelfDrivingMode {
			return false, "", nil
		}
		return true, "Custom tools/*.sh scripts require approval.", nil
	default:
		if !allowedDirectCommands()[request.Command] {
			return false, "", fmt.Errorf("command %q is not allowlisted", request.Command)
		}
		if settings.SelfDrivingMode {
			return false, "", nil
		}
		return true, fmt.Sprintf("Command %s requires approval.", request.Command), nil
	}
}

func validateModeCommand(workspaceRoot string, record domain.RunRecord, request domain.ExecRequest) error {
	if record.Mode != domain.RunModePlan {
		return nil
	}

	switch request.Command {
	case "cd":
		if len(request.Args) != 1 {
			return fmt.Errorf("plan mode cd requires exactly one path argument")
		}
		_, err := resolveCommandPath(workspaceRoot, baseCwd(record), request.Args[0])
		return err
	case "ot":
		if len(request.Args) == 0 {
			return fmt.Errorf("plan mode only allows ot read, ot list, and ot search")
		}
		switch strings.TrimSpace(request.Args[0]) {
		case "read", "list", "search":
			return nil
		default:
			return fmt.Errorf("plan mode only allows ot read, ot list, and ot search")
		}
	default:
		return fmt.Errorf("plan mode only allows cd, ot read, ot list, and ot search")
	}
}

func classifyOTApproval(workspaceRoot string, record domain.RunRecord, env []string, settings domain.Settings, request domain.ExecRequest) (bool, string, error) {
	inspection, err := inspectOTRequest(workspaceRoot, record, request)
	if err != nil {
		return false, "", err
	}

	switch inspection.Subcommand {
	case "read", "list", "search":
		if inspection.WithinWorkspace {
			return false, "", nil
		}
		return true, fmt.Sprintf(otSearchReasonOutside, inspection.Subcommand), nil
	case "pointer":
		return false, "", nil
	case "write":
		return true, "ot write requires approval.", nil
	case "subagent":
		if subagentDepth(env) > 0 {
			return false, "", fmt.Errorf("nested ot subagent runs are not allowed")
		}
		if settings.SelfDrivingMode {
			return false, "", nil
		}
		return true, "ot subagent requires approval.", nil
	default:
		if settings.SelfDrivingMode {
			return false, "", nil
		}
		return true, fmt.Sprintf("ot %s requires approval.", inspection.Subcommand), nil
	}
}

func isBlockedDestructiveCommand(request domain.ExecRequest) bool {
	switch request.Command {
	case "rm", "mv":
		return true
	case "ot":
		if len(request.Args) == 0 {
			return false
		}
		if strings.TrimSpace(request.Args[0]) != "exec" || len(request.Args) < 2 {
			return false
		}
		command := strings.TrimSpace(request.Args[1])
		return command == "rm" || command == "mv"
	default:
		return false
	}
}

func decodeExecRequest(call domain.ToolCall) (domain.ExecRequest, error) {
	if call.Name != "exec" {
		return domain.ExecRequest{}, fmt.Errorf("unsupported tool %q", call.Name)
	}

	var request domain.ExecRequest
	if err := json.Unmarshal([]byte(call.Arguments), &request); err != nil {
		return domain.ExecRequest{}, fmt.Errorf("decode exec request: %w", err)
	}
	request.Command = strings.TrimSpace(request.Command)
	if request.Command == "" {
		return domain.ExecRequest{}, fmt.Errorf("exec.command is required")
	}
	if len(strings.Fields(request.Command)) > 1 {
		return domain.ExecRequest{}, fmt.Errorf("exec.command must be a bare executable name; put flags and paths in exec.args")
	}
	if request.TimeoutSec < 0 {
		return domain.ExecRequest{}, fmt.Errorf("exec.timeout_sec must be >= 0")
	}
	return request, nil
}

func runExternal(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.ExecRequest) (string, error) {
	cwd, err := resolveExecutionCwd(workspaceRoot, record, request)
	if err != nil {
		return "", err
	}

	runCtx := ctx
	cancel := func() {}
	if request.TimeoutSec > 0 {
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(request.TimeoutSec)*time.Second)
	}
	defer cancel()

	command := exec.CommandContext(runCtx, request.Command, request.Args...)
	command.Dir = cwd
	command.Env = baseEnv(workspaceRoot, env)
	if request.Stdin != "" {
		command.Stdin = strings.NewReader(request.Stdin)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		combined := truncateOutput(stdout.String() + stderr.String())
		if combined == "" {
			return "", fmt.Errorf("run %s: %w", request.Command, err)
		}
		return "", fmt.Errorf("run %s: %w: %s", request.Command, err, combined)
	}

	return truncateOutput(stdout.String() + stderr.String()), nil
}

func validateCustomScript(workspaceRoot string, request domain.ExecRequest) error {
	if request.Command != "bash" {
		return nil
	}
	if len(request.Args) == 0 {
		return fmt.Errorf("bash requires a script path")
	}
	if strings.HasPrefix(request.Args[0], "-") {
		return fmt.Errorf("bash flags are not allowed")
	}

	scriptPath, err := resolveCommandPath(workspaceRoot, workspaceRoot, request.Args[0])
	if err != nil {
		return err
	}
	if filepath.Ext(scriptPath) != ".sh" {
		return fmt.Errorf("custom script must be a .sh file")
	}

	toolsRoot := filepath.Join(workspaceRoot, "tools")
	rel, err := filepath.Rel(toolsRoot, scriptPath)
	if err != nil {
		return fmt.Errorf("compute script path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("custom bash script must live under tools/")
	}
	return nil
}

func resolveExecutionCwd(workspaceRoot string, record domain.RunRecord, request domain.ExecRequest) (string, error) {
	raw := strings.TrimSpace(request.Cwd)
	if raw == "" {
		return baseCwd(record), nil
	}
	return resolveCommandPath(workspaceRoot, baseCwd(record), raw)
}

func resolveCommandPath(workspaceRoot string, base string, raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" || cleaned == "." {
		return filepath.Clean(base), nil
	}

	var candidate string
	if filepath.IsAbs(cleaned) {
		candidate = filepath.Clean(cleaned)
	} else {
		candidate = filepath.Clean(filepath.Join(base, cleaned))
	}

	rel, err := filepath.Rel(workspaceRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("compute relative path for %s: %w", candidate, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace root", raw)
	}
	return candidate, nil
}

func baseCwd(record domain.RunRecord) string {
	if strings.TrimSpace(record.CurrentCwd) != "" {
		return record.CurrentCwd
	}
	return record.WorkspacePath
}

func displayRelativePath(workspaceRoot string, path string) string {
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func truncateOutput(value string) string {
	if len(value) <= maxCommandOutputBytes {
		return value
	}
	return value[len(value)-maxCommandOutputBytes:]
}

func allowedDirectCommands() map[string]bool {
	return map[string]bool{
		"bash":   true,
		"find":   true,
		"git":    true,
		"go":     true,
		"mv":     true,
		"node":   true,
		"npm":    true,
		"ot":     true,
		"pytest": true,
		"python": true,
		"rg":     true,
		"rm":     true,
		"uv":     true,
	}
}
