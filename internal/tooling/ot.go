package tooling

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/session"
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

func inspectOTRequest(workspaceRoot string, record domain.RunRecord, request domain.ExecRequest) (otInspection, error) {
	if len(request.Args) == 0 {
		return otInspection{}, fmt.Errorf("ot requires a subcommand")
	}

	subcommand := strings.TrimSpace(request.Args[0])
	if subcommand == "" {
		return otInspection{}, fmt.Errorf("ot subcommand is required")
	}
	if _, ok := supportedOTSubcommands[subcommand]; !ok {
		return otInspection{}, fmt.Errorf("ot %s is not supported", subcommand)
	}

	args := request.Args[1:]
	switch subcommand {
	case "read":
		return inspectOTRead(workspaceRoot, record, args)
	case "pointer":
		return inspectOTPointer(args)
	case "list":
		return inspectOTList(workspaceRoot, record, args)
	case "search":
		return inspectOTSearch(workspaceRoot, record, args)
	case "subagent":
		return inspectOTSubagent(args)
	case "write":
		return inspectOTWrite(workspaceRoot, record, args)
	default:
		return otInspection{
			Subcommand:      subcommand,
			NormalizedArgs:  append([]string(nil), args...),
			WithinWorkspace: true,
		}, nil
	}
}

func (r *OTRunner) runPointer(workspaceRoot string, record domain.RunRecord, inspection otInspection) (string, error) {
	pointer, err := session.ParseOTPointer(inspection.Prompt)
	if err != nil {
		return "", err
	}
	sessionID := strings.TrimSpace(pointer.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(record.SessionID)
	}
	if sessionID == "" {
		return "", fmt.Errorf("ot pointer requires an active session")
	}

	repoRoot, err := resolveSubagentRepoRoot(workspaceRoot)
	if err != nil {
		return "", err
	}
	path := filepath.Join(repoRoot, ".orch", "sessions", sessionID+".jsonl")
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pointer session file: %w", err)
	}
	defer file.Close()

	lineSet := make(map[int64]struct{}, len(pointer.Lines))
	for _, line := range pointer.Lines {
		lineSet[line] = struct{}{}
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 4*1024*1024)
	var output []string
	var lineNo int64
	for scanner.Scan() {
		lineNo++
		if _, ok := lineSet[lineNo]; !ok {
			continue
		}
		output = append(output, fmt.Sprintf("%d:%s", lineNo, scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan pointer session file: %w", err)
	}
	if len(output) == 0 {
		return "", fmt.Errorf("ot pointer did not resolve any lines")
	}
	return strings.Join(output, "\n"), nil
}

func (r *OTRunner) runSubagent(
	ctx context.Context,
	workspaceRoot string,
	record domain.RunRecord,
	env []string,
	inspection otInspection,
) (string, error) {
	title := strings.TrimSpace(inspection.Prompt)
	if len(title) > 72 {
		title = title[:72]
	}
	return r.RunDelegateTask(ctx, workspaceRoot, record, env, domain.SubagentTask{
		ID:       fmt.Sprintf("task-%d", time.Now().UnixNano()),
		Title:    title,
		Contract: inspection.Prompt,
	}, true)
}

func (r *OTRunner) RunDelegateTask(
	ctx context.Context,
	workspaceRoot string,
	record domain.RunRecord,
	env []string,
	task domain.SubagentTask,
	wait bool,
) (string, error) {
	if subagentDepth(env) > 0 {
		return "", fmt.Errorf("nested ot subagent runs are not allowed")
	}
	if strings.TrimSpace(task.Contract) == "" {
		return "", fmt.Errorf("subagent task contract is required")
	}

	repoRoot, err := resolveSubagentRepoRoot(workspaceRoot)
	if err != nil {
		return "", err
	}

	executable, err := resolveOrchExecutable()
	if err != nil {
		return "", err
	}

	parentSessionID := strings.TrimSpace(record.SessionID)
	if parentSessionID == "" {
		parentSessionID = subagentPlaceholder
	}
	parentRunID := strings.TrimSpace(record.RunID)
	if parentRunID == "" {
		parentRunID = subagentPlaceholder
	}
	encodedTask, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshal subagent task: %w", err)
	}

	request := domain.ExecRequest{
		Command: executable,
		Args: []string{
			"__subagent-run",
			repoRoot,
			parentSessionID,
			parentRunID,
			string(encodedTask),
		},
		Cwd: requestCwdOrWorkspace(record),
	}

	nextEnv := append([]string(nil), env...)
	nextEnv = append(nextEnv, fmt.Sprintf("%s=%d", subagentDepthEnv, subagentDepth(env)+1))
	if wait {
		return runExternal(ctx, workspaceRoot, record, nextEnv, request)
	}

	startFile, err := os.CreateTemp("", "orch-subagent-start-*.json")
	if err != nil {
		return "", fmt.Errorf("create async start file: %w", err)
	}
	startPath := startFile.Name()
	if err := startFile.Close(); err != nil {
		return "", fmt.Errorf("close async start file: %w", err)
	}
	if err := os.Remove(startPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("prepare async start file: %w", err)
	}
	defer os.Remove(startPath)

	task.StartFilePath = startPath
	encodedTask, err = json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("marshal async subagent task: %w", err)
	}

	command := exec.Command(executable,
		"__subagent-run",
		repoRoot,
		parentSessionID,
		parentRunID,
		string(encodedTask),
	)
	command.Dir = request.Cwd
	command.Env = append([]string(nil), nextEnv...)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return "", fmt.Errorf("open devnull: %w", err)
	}
	defer devNull.Close()
	command.Stdout = devNull
	command.Stderr = devNull
	if err := command.Start(); err != nil {
		return "", fmt.Errorf("start async subagent: %w", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- command.Wait()
	}()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	for {
		data, err := os.ReadFile(startPath)
		if err == nil && len(data) > 0 {
			return strings.TrimSpace(string(data)), nil
		}
		if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("read async start file: %w", err)
		}

		select {
		case err := <-waitDone:
			if err != nil {
				return "", fmt.Errorf("async subagent exited before start handshake: %w", err)
			}
			return "", fmt.Errorf("async subagent exited before start handshake")
		case <-ctx.Done():
			_ = command.Process.Kill()
			return "", ctx.Err()
		case <-timeout.C:
			_ = command.Process.Kill()
			return "", fmt.Errorf("timed out waiting for async subagent start handshake")
		case <-ticker.C:
		}
	}
}

func inspectOTSubagent(args []string) (otInspection, error) {
	prompt, err := parseOTSubagentArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	return otInspection{
		Subcommand:      "subagent",
		NormalizedArgs:  []string{"--prompt", prompt},
		WithinWorkspace: true,
		Prompt:          prompt,
	}, nil
}

func inspectOTPointer(args []string) (otInspection, error) {
	value, err := parseOTPointerArgs(args)
	if err != nil {
		return otInspection{}, err
	}
	return otInspection{
		Subcommand:      "pointer",
		NormalizedArgs:  []string{"--value", value},
		WithinWorkspace: true,
		Prompt:          value,
	}, nil
}

func inspectOTRead(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, start, end, err := parseOTReadArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, info, withinWorkspace, err := inspectReadOnlyTarget(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}
	if info.IsDir() && (start != "" || end != "") {
		return otInspection{}, fmt.Errorf("ot read line ranges are only supported for files")
	}

	normalized := normalizedReadOnlyArgs(workspaceRoot, resolved, withinWorkspace)
	if start != "" {
		normalized = append(normalized, "--start", start)
	}
	if end != "" {
		normalized = append(normalized, "--end", end)
	}
	return otInspection{
		Subcommand:      "read",
		NormalizedArgs:  normalized,
		WithinWorkspace: withinWorkspace,
	}, nil
}

func inspectOTList(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, err := parseOTListArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, _, withinWorkspace, err := inspectReadOnlyTarget(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}

	return otInspection{
		Subcommand:      "list",
		NormalizedArgs:  normalizedReadOnlyArgs(workspaceRoot, resolved, withinWorkspace),
		WithinWorkspace: withinWorkspace,
	}, nil
}

func inspectOTSearch(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, name, content, err := parseOTSearchArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, _, withinWorkspace, err := inspectReadOnlyTarget(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}

	normalized := normalizedReadOnlyArgs(workspaceRoot, resolved, withinWorkspace)
	if name != "" {
		normalized = append(normalized, "--name", name)
	}
	if content != "" {
		normalized = append(normalized, "--content", content)
	}

	return otInspection{
		Subcommand:      "search",
		NormalizedArgs:  normalized,
		WithinWorkspace: withinWorkspace,
	}, nil
}

func inspectOTWrite(workspaceRoot string, record domain.RunRecord, args []string) (otInspection, error) {
	path, fromStdin, err := parseOTWriteArgs(args)
	if err != nil {
		return otInspection{}, err
	}

	resolved, err := resolveCommandPath(workspaceRoot, baseCwd(record), path)
	if err != nil {
		return otInspection{}, err
	}
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return otInspection{}, fmt.Errorf("ot write requires a file path, not a directory")
	} else if err != nil && !os.IsNotExist(err) {
		return otInspection{}, fmt.Errorf("stat write path %q: %w", path, err)
	}

	normalized := []string{"--path", displayRelativePath(workspaceRoot, resolved)}
	if fromStdin {
		normalized = append(normalized, "--from-stdin")
	}
	return otInspection{
		Subcommand:      "write",
		NormalizedArgs:  normalized,
		WithinWorkspace: true,
	}, nil
}

func inspectReadOnlyTarget(workspaceRoot string, base string, rawPath string) (string, os.FileInfo, bool, error) {
	resolved, err := resolveInspectablePath(base, rawPath)
	if err != nil {
		return "", nil, false, err
	}

	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, false, fmt.Errorf("stat path %q: %w", rawPath, err)
	}

	withinWorkspace := isPathInsideWorkspace(workspaceRoot, resolved)
	if !withinWorkspace && pathContainsHiddenSegment(resolved) {
		return "", nil, false, fmt.Errorf("path %q contains hidden segments outside the workspace", rawPath)
	}
	return resolved, info, withinWorkspace, nil
}

func normalizedReadOnlyArgs(workspaceRoot string, resolved string, withinWorkspace bool) []string {
	scope := otScopeOutside
	display := otDisplayAbsolute
	if withinWorkspace {
		scope = otScopeInside
		display = otDisplayRelative
	}

	return []string{
		"--target", filepath.Clean(resolved),
		"--scope", scope,
		"--display", display,
		"--workspace-root", filepath.Clean(workspaceRoot),
	}
}

func parseOTReadArgs(args []string) (string, string, string, error) {
	path := ""
	start := ""
	end := ""

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--path is required")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		case "--start":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--start requires a value")
			}
			start = strings.TrimSpace(args[index+1])
			index += 2
		case "--end":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--end requires a value")
			}
			end = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", "", "", fmt.Errorf("unknown ot read arg: %s", args[index])
		}
	}

	if path == "" {
		return "", "", "", fmt.Errorf("--path is required")
	}
	return path, start, end, nil
}

func parseOTListArgs(args []string) (string, error) {
	path := "."

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", fmt.Errorf("--path requires a value")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", fmt.Errorf("unknown ot list arg: %s", args[index])
		}
	}

	if path == "" {
		path = "."
	}
	return path, nil
}

func parseOTSearchArgs(args []string) (string, string, string, error) {
	path := "."
	name := ""
	content := ""

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--path requires a value")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		case "--name":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--name requires a value")
			}
			name = strings.TrimSpace(args[index+1])
			index += 2
		case "--content":
			if index+1 >= len(args) {
				return "", "", "", fmt.Errorf("--content requires a value")
			}
			content = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", "", "", fmt.Errorf("unknown ot search arg: %s", args[index])
		}
	}

	if path == "" {
		path = "."
	}
	if name == "" && content == "" {
		return "", "", "", fmt.Errorf("ot search requires --name or --content")
	}
	return path, name, content, nil
}

func parseOTWriteArgs(args []string) (string, bool, error) {
	path := ""
	fromStdin := false

	for index := 0; index < len(args); {
		switch args[index] {
		case "--path":
			if index+1 >= len(args) {
				return "", false, fmt.Errorf("--path is required")
			}
			path = strings.TrimSpace(args[index+1])
			index += 2
		case "--from-stdin":
			fromStdin = true
			index++
		default:
			return "", false, fmt.Errorf("unknown ot write arg: %s", args[index])
		}
	}

	if path == "" {
		return "", false, fmt.Errorf("--path is required")
	}
	if !fromStdin {
		return "", false, fmt.Errorf("--from-stdin is required")
	}
	return path, fromStdin, nil
}

func parseOTSubagentArgs(args []string) (string, error) {
	prompt := ""

	for index := 0; index < len(args); {
		switch args[index] {
		case "--prompt":
			if index+1 >= len(args) {
				return "", fmt.Errorf("--prompt requires a value")
			}
			prompt = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", fmt.Errorf("unknown ot subagent arg: %s", args[index])
		}
	}

	if prompt == "" {
		return "", fmt.Errorf("ot subagent requires --prompt")
	}
	return prompt, nil
}

func parseOTPointerArgs(args []string) (string, error) {
	value := ""
	for index := 0; index < len(args); {
		switch args[index] {
		case "--value":
			if index+1 >= len(args) {
				return "", fmt.Errorf("--value requires a value")
			}
			value = strings.TrimSpace(args[index+1])
			index += 2
		default:
			return "", fmt.Errorf("unknown ot pointer arg: %s", args[index])
		}
	}
	if value == "" {
		return "", fmt.Errorf("ot pointer requires --value")
	}
	return value, nil
}

func resolveInspectablePath(base string, raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" || cleaned == "." {
		return filepath.Clean(base), nil
	}

	if filepath.IsAbs(cleaned) {
		return filepath.Clean(cleaned), nil
	}
	return filepath.Clean(filepath.Join(base, cleaned)), nil
}

func isPathInsideWorkspace(workspaceRoot string, path string) bool {
	rel, err := filepath.Rel(workspaceRoot, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func pathContainsHiddenSegment(path string) bool {
	cleaned := filepath.Clean(path)
	for _, segment := range strings.Split(cleaned, string(filepath.Separator)) {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
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

func subagentDepth(env []string) int {
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key != subagentDepthEnv {
			continue
		}

		depth := 0
		if _, err := fmt.Sscanf(value, "%d", &depth); err == nil && depth > 0 {
			return depth
		}
	}
	return 0
}

func resolveOrchExecutable() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable: %w", err)
	}

	if filepath.Base(executable) != "ot" {
		return executable, nil
	}

	sibling := filepath.Join(filepath.Dir(executable), "orch")
	if _, err := os.Stat(sibling); err == nil {
		return sibling, nil
	}

	if lookedUp, err := exec.LookPath("orch"); err == nil {
		return lookedUp, nil
	}
	return "", fmt.Errorf("resolve orch executable from %s", executable)
}

func resolveSubagentRepoRoot(workspaceRoot string) (string, error) {
	current := filepath.Clean(workspaceRoot)
	for {
		if config.LooksLikeRepoRoot(current) {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("resolve repo root from workspace %s", workspaceRoot)
}

func requiresEmbeddedOTHelpers(subcommand string) bool {
	switch strings.TrimSpace(subcommand) {
	case "patch", "search":
		return true
	default:
		return false
	}
}

func requestCwdOrWorkspace(record domain.RunRecord) string {
	if strings.TrimSpace(record.CurrentCwd) != "" {
		return record.CurrentCwd
	}
	return record.WorkspacePath
}
