package tooling

import (
	"context"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func executeCompleteOperation(_ *Executor, _ context.Context, _ string, _ domain.RunRecord, _ []string, request domain.OTRequest) (Execution, error) {
	return terminalExecution(request, domain.StatusCompleted, "Worker task completed."), nil
}

func executeFailOperation(_ *Executor, _ context.Context, _ string, _ domain.RunRecord, _ []string, request domain.OTRequest) (Execution, error) {
	return terminalExecution(request, domain.StatusFailed, "Worker task failed."), nil
}

func terminalExecution(request domain.OTRequest, status domain.RunStatus, fallback string) Execution {
	message := terminalTaskMessage(request, fallback)
	return Execution{
		Output:               renderTaskOutcomeOutput(request, message),
		TerminalStatus:       status,
		TerminalMessage:      message,
		TaskSummary:          strings.TrimSpace(request.Summary),
		TaskChangedPaths:     append([]string(nil), request.ChangedPaths...),
		TaskChecksRun:        append([]string(nil), request.ChecksRun...),
		TaskEvidencePointers: append([]string(nil), request.EvidencePointers...),
		TaskFollowups:        append([]string(nil), request.Followups...),
		TaskErrorKind:        strings.TrimSpace(request.ErrorKind),
	}
}
