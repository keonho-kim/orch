package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

func runExec(repoRoot string, prompt string, mode domain.RunMode, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	app, err := newApp(repoRoot, orchestrator.BootOptions{})
	if err != nil {
		return err
	}
	defer app.close()

	settings := app.service.Snapshot().Settings
	if settings.DefaultProvider == "" {
		return fmt.Errorf("default provider is not configured; run `orch config --scope global --provider=<provider> --model=<name>` or edit orch.env.toml first")
	}
	if err := settings.ProviderConfigError(settings.DefaultProvider); err != nil {
		return fmt.Errorf("%w; run `orch config` or edit orch.env.toml first", err)
	}

	runID, err := app.service.SubmitPromptMode(prompt, mode)
	if err != nil {
		return err
	}

	fmt.Fprintf(stderr, "orch exec: provider=%s mode=%s run=%s\n", settings.DefaultProvider.DisplayName(), mode.String(), runID)
	return streamExecRun(app.ctx, app.service, runID, stdin, stdout)
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
