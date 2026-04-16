package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

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
