package tooling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	TerminalStatus   domain.RunStatus
	TerminalMessage  string
}

func NewExecutor() *Executor {
	return &Executor{ot: NewOTRunner()}
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

	switch strings.TrimSpace(request.Op) {
	case "read":
		return e.executeRead(ctx, workspaceRoot, record, env, request)
	case "list":
		return e.executeList(ctx, workspaceRoot, record, env, request)
	case "search":
		return e.executeSearch(ctx, workspaceRoot, record, env, request)
	case "write":
		return e.executeWrite(ctx, workspaceRoot, record, env, request)
	case "patch":
		return e.executePatch(ctx, workspaceRoot, record, env, request)
	case "check":
		return e.executeCheck(ctx, workspaceRoot, record, env, request)
	case "delegate":
		output, err := e.ot.RunDelegateTask(ctx, workspaceRoot, record, env, domain.SubagentTask{
			ID:       normalizeTaskID(request),
			Title:    strings.TrimSpace(request.TaskTitle),
			Contract: strings.TrimSpace(request.TaskContract),
		})
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "complete":
		message := strings.TrimSpace(request.Message)
		if message == "" {
			message = "Worker task completed."
		}
		return Execution{
			Output:          message,
			TerminalStatus:  domain.StatusCompleted,
			TerminalMessage: message,
		}, nil
	case "fail":
		message := strings.TrimSpace(request.Message)
		if message == "" {
			message = "Worker task failed."
		}
		return Execution{
			Output:          message,
			TerminalStatus:  domain.StatusFailed,
			TerminalMessage: message,
		}, nil
	default:
		return Execution{}, fmt.Errorf("unsupported ot op %q", request.Op)
	}
}

func decodeOTRequest(call domain.ToolCall) (domain.OTRequest, error) {
	if call.Name != "ot" {
		return domain.OTRequest{}, fmt.Errorf("unsupported tool %q", call.Name)
	}

	var request domain.OTRequest
	if err := json.Unmarshal([]byte(call.Arguments), &request); err != nil {
		return domain.OTRequest{}, fmt.Errorf("decode ot request: %w", err)
	}
	request.Op = strings.TrimSpace(strings.ToLower(request.Op))
	if request.Op == "" {
		return domain.OTRequest{}, fmt.Errorf("ot.op is required")
	}
	if request.StartLine < 0 || request.EndLine < 0 {
		return domain.OTRequest{}, fmt.Errorf("line ranges must be >= 0")
	}
	return request, nil
}

func validateOTRequest(record domain.RunRecord, request domain.OTRequest) error {
	role := normalizeRecordRole(record)
	op := strings.TrimSpace(request.Op)
	if op == "" {
		return fmt.Errorf("ot.op is required")
	}

	if record.Mode == domain.RunModePlan {
		switch op {
		case "read", "list", "search":
			return validatePathRequest(op, request)
		default:
			return fmt.Errorf("plan mode only allows read, list, and search operations")
		}
	}

	switch role {
	case domain.AgentRoleWorker:
		switch op {
		case "read", "list", "search":
			return validatePathRequest(op, request)
		case "write":
			if strings.TrimSpace(request.Path) == "" {
				return fmt.Errorf("write requires path")
			}
			if request.Content == "" {
				return fmt.Errorf("write requires content")
			}
			return nil
		case "patch":
			if strings.TrimSpace(request.Patch) == "" {
				return fmt.Errorf("patch requires patch content")
			}
			return nil
		case "check":
			switch strings.TrimSpace(request.Check) {
			case "go_test", "go_vet", "golangci_lint":
				return nil
			default:
				return fmt.Errorf("unsupported check %q", request.Check)
			}
		case "complete", "fail":
			return nil
		default:
			return fmt.Errorf("worker role does not allow ot op %q", request.Op)
		}
	default:
		switch op {
		case "delegate":
			if strings.TrimSpace(request.TaskContract) == "" {
				return fmt.Errorf("delegate requires task_contract")
			}
			if strings.TrimSpace(request.TaskTitle) == "" {
				return fmt.Errorf("delegate requires task_title")
			}
			return nil
		case "read", "list", "search":
			return validatePathRequest(op, request)
		default:
			return fmt.Errorf("gateway role does not allow ot op %q", request.Op)
		}
	}
}

func validatePathRequest(op string, request domain.OTRequest) error {
	if op != "list" && strings.TrimSpace(request.Path) == "" {
		return fmt.Errorf("%s requires path", op)
	}
	if op == "search" && strings.TrimSpace(request.NamePattern) == "" && strings.TrimSpace(request.ContentPattern) == "" {
		return fmt.Errorf("search requires name_pattern or content_pattern")
	}
	if op == "read" && request.EndLine > 0 && request.StartLine > request.EndLine {
		return fmt.Errorf("start_line must be <= end_line")
	}
	return nil
}

func classifyOTApproval(record domain.RunRecord, settings domain.Settings, request domain.OTRequest) (bool, string, error) {
	switch request.Op {
	case "read", "list", "search", "delegate", "complete", "fail":
		return false, "", nil
	case "write":
		return true, "ot write requires approval.", nil
	case "patch":
		return true, "ot patch requires approval.", nil
	case "check":
		if settings.SelfDrivingMode && normalizeRecordRole(record) == domain.AgentRoleWorker {
			return false, "", nil
		}
		return true, "ot check requires approval.", nil
	default:
		return false, "", fmt.Errorf("unsupported ot op %q", request.Op)
	}
}

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
	output, err := e.ot.Run(ctx, workspaceRoot, record, env, domain.ExecRequest{Command: "ot", Args: args})
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
	output, err := e.ot.Run(ctx, workspaceRoot, record, env, domain.ExecRequest{Command: "ot", Args: args})
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
	output, err := e.ot.Run(ctx, workspaceRoot, record, env, domain.ExecRequest{Command: "ot", Args: args})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func (e *Executor) executeWrite(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	output, err := e.ot.Run(ctx, workspaceRoot, record, env, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"write", "--path", normalizeWorkspacePath(request.Path), "--from-stdin"},
		Stdin:   request.Content,
	})
	if err != nil {
		return Execution{}, err
	}
	return Execution{Output: output}, nil
}

func (e *Executor) executePatch(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, request domain.OTRequest) (Execution, error) {
	output, err := e.ot.Run(ctx, workspaceRoot, record, env, domain.ExecRequest{
		Command: "ot",
		Args:    []string{"patch", "--from-stdin"},
		Stdin:   request.Patch,
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

func normalizeWorkspacePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return strings.TrimSpace(path)
}

func normalizeWorkspaceRelativePath(workspaceRoot string, record domain.RunRecord, path string) (string, error) {
	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), normalizeWorkspacePath(path))
	if err != nil {
		return "", err
	}
	return displayRelativePath(workspaceRoot, resolved), nil
}

func normalizeTaskID(request domain.OTRequest) string {
	if strings.TrimSpace(request.TaskID) != "" {
		return strings.TrimSpace(request.TaskID)
	}
	title := strings.TrimSpace(request.TaskTitle)
	if title == "" {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "/", "-")
	return fmt.Sprintf("%s-%d", slug, time.Now().UnixNano())
}

func normalizeRecordRole(record domain.RunRecord) domain.AgentRole {
	role, err := domain.ParseAgentRole(record.AgentRole.String())
	if err != nil {
		return domain.AgentRoleGateway
	}
	return role
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

func truncateOutput(value string) string {
	if len(value) <= maxCommandOutputBytes {
		return value
	}
	return value[len(value)-maxCommandOutputBytes:]
}
