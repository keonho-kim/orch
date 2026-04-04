package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

func runExec(repoRoot string, configFile string, prompt string, mode domain.RunMode, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	app, err := newApp(repoRoot, configFile, orchestrator.BootOptions{})
	if err != nil {
		return err
	}
	defer app.close()

	settings := app.service.Snapshot().Settings
	if settings.DefaultProvider == "" {
		return fmt.Errorf("default provider is not configured; run `orch` or edit orch.toml first")
	}
	if err := settings.ProviderConfigError(settings.DefaultProvider); err != nil {
		return fmt.Errorf("%w; run `orch` or edit orch.toml first", err)
	}

	runID, err := app.service.SubmitPromptMode(prompt, mode)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(stderr, "orch exec: provider=%s mode=%s run=%s\n", settings.DefaultProvider.DisplayName(), mode.String(), runID); err != nil {
		return err
	}
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
		writtenNow, err := writePendingExecOutput(writer, service.RunOutput(runID), written)
		if err != nil {
			return err
		}
		written = writtenNow
		snapshot := service.Snapshot()
		handledApproval, err = handleExecApproval(service, runID, writer, reader, snapshot.PendingApproval, stdinTTY, handledApproval)
		if err != nil {
			return err
		}

		record, ok := service.RunRecord(runID)
		if ok && !service.RunActive(runID) && isExecTerminalStatus(record.Status) {
			return execTerminalError(runID, record.Status)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func writePendingExecOutput(writer io.Writer, output string, written int) (int, error) {
	if len(output) <= written {
		return written, nil
	}
	if _, err := io.WriteString(writer, output[written:]); err != nil {
		return written, fmt.Errorf("write exec output: %w", err)
	}
	return len(output), nil
}

func handleExecApproval(
	service *orchestrator.Service,
	runID string,
	writer io.Writer,
	reader *bufio.Reader,
	request *domain.ApprovalRequest,
	stdinTTY bool,
	handled bool,
) (bool, error) {
	if request == nil || request.RunID != runID {
		return false, nil
	}
	if handled {
		return true, nil
	}
	if !stdinTTY {
		_ = service.ResolveApproval(runID, false)
		return false, fmt.Errorf("approval required for %s but stdin is not a TTY", request.Call.Name)
	}
	if err := promptApproval(writer, request); err != nil {
		return false, err
	}
	approved, err := readApprovalDecision(reader, writer)
	if err != nil {
		return false, err
	}
	if err := service.ResolveApproval(runID, approved); err != nil {
		return false, err
	}
	return true, nil
}

func execTerminalError(runID string, status domain.RunStatus) error {
	if status == domain.StatusCompleted {
		return nil
	}
	return fmt.Errorf("run %s ended with status %s", runID, status)
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
