package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/apiserver"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/orchestrator"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
	"github.com/keonho-kim/orch/internal/tooling"
	"github.com/keonho-kim/orch/internal/tui"
)

type command struct {
	name             string
	prompt           string
	mode             domain.RunMode
	repoRoot         string
	configCommand    configCommandState
	restoreSessionID string
	showHistory      bool
	restoreLatest    bool
	finalizeSession  string
	subagentTask     string
	parentSessionID  string
	parentRunID      string
}

func Run(args []string) error {
	command, err := parseCommand(args)
	if err != nil {
		return err
	}

	switch command.name {
	case "interactive":
		return runTUI(command.repoRoot, command.restoreSessionID, command.showHistory, command.restoreLatest)
	case "exec":
		return runExec(command.repoRoot, command.prompt, command.mode, os.Stdin, os.Stdout, os.Stderr)
	case "config-list":
		return runConfigList(command.repoRoot, command.configCommand, os.Stdout)
	case "config-set":
		return runConfigUpdate(command.repoRoot, command.configCommand)
	case "__finalize-session":
		return runFinalizeSession(command.repoRoot, command.finalizeSession)
	case "__subagent-run":
		return runSubagent(command.repoRoot, command.parentSessionID, command.parentRunID, command.subagentTask, os.Stdout)
	default:
		return fmt.Errorf("unsupported command %q", command.name)
	}
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return command{name: "interactive", repoRoot: "."}, nil
	}
	if args[0] == "--workspace" {
		repoRoot, rest, err := parseWorkspaceFlag(args, ".")
		if err != nil {
			return command{}, err
		}
		if len(rest) != 0 {
			return command{}, fmt.Errorf("unsupported command %q", rest[0])
		}
		return command{name: "interactive", repoRoot: repoRoot}, nil
	}

	switch args[0] {
	case "exec":
		repoRoot, rest, err := parseWorkspaceFlag(args[1:], ".")
		if err != nil {
			return command{}, err
		}
		mode := domain.RunModeReact
		if len(rest) >= 2 && rest[0] == "--mode" {
			parsedMode, err := domain.ParseRunMode(rest[1])
			if err != nil {
				return command{}, err
			}
			mode = parsedMode
			rest = rest[2:]
		}
		if len(rest) == 0 || strings.TrimSpace(strings.Join(rest, " ")) == "" {
			return command{}, fmt.Errorf("usage: orch exec [--workspace <path>] [--mode react|plan] \"<request>\"")
		}
		return command{name: "exec", prompt: strings.Join(rest, " "), mode: mode, repoRoot: repoRoot}, nil
	case "history":
		repoRoot, rest, err := parseWorkspaceFlag(args[1:], ".")
		if err != nil {
			return command{}, err
		}
		switch {
		case len(rest) == 0:
			return command{name: "interactive", repoRoot: repoRoot, showHistory: true}, nil
		case rest[0] == "--latest":
			return command{name: "interactive", repoRoot: repoRoot, restoreLatest: true}, nil
		default:
			return command{name: "interactive", repoRoot: repoRoot, restoreSessionID: rest[0]}, nil
		}
	case "config":
		return parseConfigCommand(args[1:])
	case "__finalize-session":
		if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
			return command{}, fmt.Errorf("usage: orch __finalize-session <session-id> [workspace]")
		}
		repoRoot := "."
		if len(args) > 2 {
			repoRoot = args[2]
		}
		return command{name: "__finalize-session", finalizeSession: args[1], repoRoot: repoRoot}, nil
	case "__subagent-run":
		if len(args) != 5 || strings.TrimSpace(args[1]) == "" || strings.TrimSpace(args[4]) == "" {
			return command{}, fmt.Errorf("usage: orch __subagent-run <repo-root> <parent-session-id|-> <parent-run-id|-> <task-json>")
		}
		return command{
			name:            "__subagent-run",
			repoRoot:        args[1],
			parentSessionID: hiddenValue(args[2]),
			parentRunID:     hiddenValue(args[3]),
			subagentTask:    args[4],
		}, nil
	default:
		return command{}, fmt.Errorf("unsupported command %q", args[0])
	}
}

func parseWorkspaceFlag(args []string, defaultRepoRoot string) (string, []string, error) {
	repoRoot := defaultRepoRoot
	rest := make([]string, 0, len(args))
	seen := false

	for index := 0; index < len(args); index++ {
		if args[index] != "--workspace" {
			rest = append(rest, args[index])
			continue
		}
		if seen {
			return "", nil, fmt.Errorf("--workspace may only be provided once")
		}
		if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
			return "", nil, fmt.Errorf("--workspace requires a path")
		}
		repoRoot = args[index+1]
		seen = true
		index++
	}

	return repoRoot, rest, nil
}

func hiddenValue(value string) string {
	if strings.TrimSpace(value) == "-" {
		return ""
	}
	return value
}

func runTUI(repoRoot string, restoreSessionID string, showHistory bool, restoreLatest bool) error {
	app, err := newApp(repoRoot, orchestrator.BootOptions{RestoreSessionID: restoreSessionID})
	if err != nil {
		return err
	}
	defer app.close()

	if restoreLatest {
		sessionID, err := app.service.LatestSessionID()
		if err != nil {
			return err
		}
		if err := app.service.RestoreSession(sessionID); err != nil {
			return err
		}
	}

	if isTTY(os.Stdout) {
		if _, err := io.WriteString(os.Stdout, "\r\n"); err != nil {
			return fmt.Errorf("prepare tui output: %w", err)
		}
	}

	model := tui.New(app.service)
	apiStatus, apiEvent := startAttachedAPIServer(app, os.Stderr)
	if strings.TrimSpace(apiStatus) != "" {
		model.SetStatusMessage(apiStatus)
	}
	if apiEvent.Type != "" {
		app.service.EmitEvent(apiEvent)
	}
	if showHistory {
		model.OpenHistoryPicker()
	}

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(app.ctx),
	)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}

func runExec(repoRoot string, prompt string, mode domain.RunMode, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	app, err := newApp(repoRoot, orchestrator.BootOptions{})
	if err != nil {
		return err
	}
	defer app.close()

	settings := app.service.Snapshot().Settings
	if settings.DefaultProvider == "" {
		return fmt.Errorf("default provider is not configured; run `orch` or edit orch.settings.json first")
	}
	if err := settings.ProviderConfigError(settings.DefaultProvider); err != nil {
		return fmt.Errorf("%w; run `orch` or edit orch.settings.json first", err)
	}

	runID, err := app.service.SubmitPromptMode(prompt, mode)
	if err != nil {
		return err
	}

	fmt.Fprintf(stderr, "orch exec: provider=%s mode=%s run=%s\n", settings.DefaultProvider.DisplayName(), mode.String(), runID)
	return streamExecRun(app.ctx, app.service, runID, stdin, stdout)
}

type app struct {
	ctx          context.Context
	cancel       context.CancelFunc
	store        *sqlitestore.Store
	service      *orchestrator.Service
	paths        config.Paths
	api          *apiserver.Server
	skipFinalize bool
}

func newApp(repoRoot string, options orchestrator.BootOptions) (*app, error) {
	paths, err := resolveAppPaths(repoRoot)
	if err != nil {
		return nil, err
	}
	if err := config.EnsureRuntimePaths(paths); err != nil {
		return nil, err
	}

	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	service, err := orchestrator.NewService(ctx, store, tooling.NewExecutor(), paths, options)
	if err != nil {
		cancel()
		_ = store.Close()
		return nil, err
	}

	return &app{
		ctx:     ctx,
		cancel:  cancel,
		store:   store,
		service: service,
		paths:   paths,
	}, nil
}

func (a *app) close() {
	if a.api != nil {
		_ = a.api.Close()
	}
	a.cancel()
	if a.service != nil && !a.skipFinalize {
		_ = spawnFinalizeProcess(a.service.Snapshot().CurrentSession.SessionID, a.paths.RepoRoot)
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

func runFinalizeSession(repoRoot string, sessionID string) error {
	app, err := newApp(repoRoot, orchestrator.BootOptions{RestoreSessionID: sessionID})
	if err != nil {
		return err
	}
	app.skipFinalize = true
	defer app.close()
	return app.service.FinalizeCurrentSession()
}

func runSubagent(repoRoot string, parentSessionID string, parentRunID string, encodedTask string, stdout io.Writer) error {
	var task domain.SubagentTask
	if err := json.Unmarshal([]byte(encodedTask), &task); err != nil {
		return fmt.Errorf("decode subagent task: %w", err)
	}
	if strings.TrimSpace(task.Contract) == "" {
		return fmt.Errorf("subagent task contract is required")
	}

	app, err := newApp(repoRoot, orchestrator.BootOptions{
		ParentSessionID:      parentSessionID,
		ParentRunID:          parentRunID,
		ParentTaskID:         task.ID,
		TaskTitle:            task.Title,
		TaskContract:         task.Contract,
		TaskStatus:           "queued",
		AgentRole:            domain.AgentRoleWorker,
		InheritParentContext: strings.TrimSpace(parentSessionID) != "",
	})
	if err != nil {
		return err
	}
	defer app.close()

	childSessionID := app.service.Snapshot().CurrentSession.SessionID
	runID, err := app.service.SubmitPromptMode(task.Contract, domain.RunModeReact)
	if err != nil {
		return err
	}
	if strings.TrimSpace(task.StartFilePath) != "" {
		if err := writeSubagentStartPayload(task.StartFilePath, buildTaskView(app.service.Snapshot().CurrentSession, task, runID)); err != nil {
			return err
		}
	}

	record, err := waitForRun(app.ctx, app.service, runID)
	if err != nil {
		return err
	}

	result := buildSubagentResult(childSessionID, task, app.service.Snapshot().CurrentSession, record)
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal subagent result: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(encoded)); err != nil {
		return fmt.Errorf("write subagent result: %w", err)
	}
	return nil
}

func startAttachedAPIServer(app *app, stderr io.Writer) (string, orchestrator.ServiceEvent) {
	server, err := apiserver.Start(app.ctx, app.service, app.paths)
	if err != nil {
		message := "Attached API server failed to start."
		_, _ = fmt.Fprintf(stderr, "orch api: %v\n", err)
		return message + " See stderr for details.", orchestrator.ServiceEvent{}
	}
	app.api = server

	discovery := server.Discovery()
	currentPath := filepath.Join(app.paths.APIDir, "current.json")
	_, _ = fmt.Fprintf(stderr, "orch api: ready at %s (details: %s)\n", discovery.BaseURL, currentPath)

	message := fmt.Sprintf("Local API ready at %s. See %s for connection details.", discovery.BaseURL, currentPath)
	return message, orchestrator.ServiceEvent{
		Type:      "api_server_ready",
		SessionID: discovery.SessionID,
		Message:   "API server ready.",
		Payload: map[string]any{
			"base_url":   discovery.BaseURL,
			"session_id": discovery.SessionID,
			"repo_root":  discovery.RepoRoot,
		},
	}
}

func waitForRun(ctx context.Context, service *orchestrator.Service, runID string) (domain.RunRecord, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		record, ok := service.RunRecord(runID)
		if ok && isExecTerminalStatus(record.Status) {
			return record, nil
		}

		select {
		case <-ctx.Done():
			return domain.RunRecord{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func buildSubagentResult(
	childSessionID string,
	task domain.SubagentTask,
	meta domain.SessionMetadata,
	record domain.RunRecord,
) domain.SubagentResult {
	finalOutput, truncated := truncateSubagentOutput(record.FinalOutput)
	result := domain.SubagentResult{
		ChildSessionID:       childSessionID,
		ChildRunID:           record.RunID,
		TaskID:               task.ID,
		TaskTitle:            task.Title,
		TaskStatus:           meta.TaskStatus,
		WorkerRole:           meta.WorkerRole.String(),
		Status:               string(record.Status),
		TaskSummary:          meta.TaskSummary,
		TaskChangedPaths:     append([]string(nil), meta.TaskChangedPaths...),
		TaskChecksRun:        append([]string(nil), meta.TaskChecksRun...),
		TaskEvidencePointers: append([]string(nil), meta.TaskEvidencePointers...),
		TaskFollowups:        append([]string(nil), meta.TaskFollowups...),
		TaskErrorKind:        meta.TaskErrorKind,
		FinalOutput:          finalOutput,
		Truncated:            truncated,
	}

	if record.Status != domain.StatusCompleted {
		result.Error = strings.TrimSpace(record.FinalOutput)
		if result.Error == "" {
			result.Error = strings.TrimSpace(record.CurrentTask)
		}
	}
	return result
}

func truncateSubagentOutput(value string) (string, bool) {
	const maxSubagentOutputBytes = 12000

	if len(value) <= maxSubagentOutputBytes {
		return value, false
	}
	return value[len(value)-maxSubagentOutputBytes:], true
}

func buildTaskView(meta domain.SessionMetadata, task domain.SubagentTask, runID string) domain.TaskView {
	return domain.TaskView{
		TaskID:               task.ID,
		Title:                task.Title,
		Status:               meta.TaskStatus,
		ParentSessionID:      meta.ParentSessionID,
		ParentRunID:          meta.ParentRunID,
		ChildSessionID:       meta.SessionID,
		ChildRunID:           runID,
		WorkerRole:           meta.WorkerRole.String(),
		Provider:             meta.Provider.String(),
		Model:                meta.Model,
		TaskSummary:          meta.TaskSummary,
		TaskChangedPaths:     append([]string(nil), meta.TaskChangedPaths...),
		TaskChecksRun:        append([]string(nil), meta.TaskChecksRun...),
		TaskEvidencePointers: append([]string(nil), meta.TaskEvidencePointers...),
		TaskFollowups:        append([]string(nil), meta.TaskFollowups...),
		TaskErrorKind:        meta.TaskErrorKind,
		StartedAt:            meta.StartedAt,
		UpdatedAt:            meta.UpdatedAt,
		FinalizedAt:          meta.FinalizedAt,
	}
}

func writeSubagentStartPayload(path string, task domain.TaskView) error {
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal subagent start payload: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write subagent start payload: %w", err)
	}
	return nil
}

func streamExecRun(ctx context.Context, service *orchestrator.Service, runID string, stdin io.Reader, writer io.Writer) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	reader := bufio.NewReader(stdin)
	written := 0
	handledApproval := false
	stdinTTY := isTTY(stdin)

	for {
		output := service.RunOutput(runID)
		if len(output) > written {
			if _, err := io.WriteString(writer, output[written:]); err != nil {
				return fmt.Errorf("write exec output: %w", err)
			}
			written = len(output)
		}

		snapshot := service.Snapshot()
		if snapshot.PendingApproval != nil && snapshot.PendingApproval.RunID == runID {
			if !handledApproval {
				if !stdinTTY {
					_ = service.ResolveApproval(runID, false)
					return fmt.Errorf("approval required for %s but stdin is not a TTY", snapshot.PendingApproval.Call.Name)
				}
				if err := promptApproval(writer, snapshot.PendingApproval); err != nil {
					return err
				}
				approved, err := readApprovalDecision(reader, writer)
				if err != nil {
					return err
				}
				if err := service.ResolveApproval(runID, approved); err != nil {
					return err
				}
				handledApproval = true
			}
		} else {
			handledApproval = false
		}

		record, ok := service.RunRecord(runID)
		if ok && !service.RunActive(runID) && isExecTerminalStatus(record.Status) {
			if record.Status == domain.StatusCompleted {
				return nil
			}
			return fmt.Errorf("run %s ended with status %s", runID, record.Status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func promptApproval(writer io.Writer, request *domain.ApprovalRequest) error {
	if _, err := fmt.Fprintf(writer, "\n[approval] tool=%s reason=%s\n%s\nApprove? [y/N]: ", request.Call.Name, request.Reason, request.Call.Arguments); err != nil {
		return fmt.Errorf("write approval prompt: %w", err)
	}
	return nil
}

func readApprovalDecision(reader *bufio.Reader, writer io.Writer) (bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("read approval response: %w", err)
	}

	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		if _, err := io.WriteString(writer, "Denied.\n"); err != nil {
			return false, fmt.Errorf("write approval denial: %w", err)
		}
		return false, nil
	}
}

func spawnFinalizeProcess(sessionID string, repoRoot string) error {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}

	command := exec.Command(executable, "__finalize-session", sessionID, repoRoot)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.Stdin = nil
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func isExecTerminalStatus(status domain.RunStatus) bool {
	switch status {
	case domain.StatusCompleted, domain.StatusCancelled, domain.StatusFailed:
		return true
	default:
		return false
	}
}

func isTTY(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
