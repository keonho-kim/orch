package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"orch/domain"
	"orch/internal/config"
	"orch/internal/orchestrator"
	sqlitestore "orch/internal/store/sqlite"
	"orch/internal/tooling"
	"orch/internal/tui"
)

type command struct {
	name   string
	prompt string
	mode   domain.RunMode
}

func run(args []string) error {
	command, err := parseCommand(args)
	if err != nil {
		return err
	}

	switch command.name {
	case "tui":
		return runTUI()
	case "exec":
		return runExec(command.prompt, command.mode, os.Stdin, os.Stdout, os.Stderr)
	default:
		return fmt.Errorf("unsupported command %q", command.name)
	}
}

func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return command{name: "tui"}, nil
	}

	switch args[0] {
	case "tui":
		return command{name: "tui"}, nil
	case "exec":
		mode := domain.RunModeReact
		rest := args[1:]
		if len(rest) >= 2 && rest[0] == "--mode" {
			parsedMode, err := domain.ParseRunMode(rest[1])
			if err != nil {
				return command{}, err
			}
			mode = parsedMode
			rest = rest[2:]
		}
		if len(rest) == 0 || strings.TrimSpace(strings.Join(rest, " ")) == "" {
			return command{}, fmt.Errorf("usage: orch exec [--mode react|plan] \"<request>\"")
		}
		return command{name: "exec", prompt: strings.Join(rest, " "), mode: mode}, nil
	default:
		return command{}, fmt.Errorf("usage: orch [tui] | orch exec [--mode react|plan] \"<request>\"")
	}
}

func runTUI() error {
	app, err := newApp()
	if err != nil {
		return err
	}
	defer app.close()

	if isTTY(os.Stdout) {
		// Start from a fresh shell line before Bubble Tea takes over. Some
		// terminals visually retain the command prompt on the first frame
		// otherwise.
		if _, err := io.WriteString(os.Stdout, "\r\n"); err != nil {
			return fmt.Errorf("prepare tui output: %w", err)
		}
	}

	program := tea.NewProgram(
		tui.New(app.service),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithContext(app.ctx),
	)

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run tui: %w", err)
	}

	return nil
}

func runExec(prompt string, mode domain.RunMode, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	app, err := newApp()
	if err != nil {
		return err
	}
	defer app.close()

	settings := app.service.Snapshot().Settings
	if settings.DefaultProvider == "" {
		return fmt.Errorf("default provider is not configured; run `orch` or edit orch.settings.json first")
	}
	if !settings.HasProviderModel(settings.DefaultProvider) {
		return fmt.Errorf("model is not configured for %s; edit orch.settings.json first", settings.DefaultProvider.DisplayName())
	}

	runID, err := app.service.SubmitPromptMode(prompt, mode)
	if err != nil {
		return err
	}

	fmt.Fprintf(stderr, "orch exec: provider=%s mode=%s run=%s\n", settings.DefaultProvider.DisplayName(), mode.String(), runID)
	return streamExecRun(app.ctx, app.service, runID, stdin, stdout)
}

type app struct {
	ctx     context.Context
	cancel  context.CancelFunc
	store   *sqlitestore.Store
	service *orchestrator.Service
}

func newApp() (*app, error) {
	repoRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}

	paths, err := config.ResolvePaths(repoRoot)
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
	service, err := orchestrator.NewService(ctx, store, tooling.NewExecutor(), paths)
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
	}, nil
}

func (a *app) close() {
	a.cancel()
	if a.store != nil {
		_ = a.store.Close()
	}
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
