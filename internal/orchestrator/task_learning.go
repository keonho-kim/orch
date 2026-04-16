package orchestrator

import (
	"context"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/knowledge"
)

func (s *Service) persistTaskOutcome(record domain.RunRecord) (domain.SessionMetadata, domain.TaskOutcome, error) {
	meta, err := s.sessions.LoadMetadata(record.SessionID)
	if err != nil {
		return domain.SessionMetadata{}, domain.TaskOutcome{}, err
	}
	if record.AgentRole != domain.AgentRoleWorker {
		return meta, domain.TaskOutcome{}, nil
	}

	outcome := domain.TaskOutcome{
		SessionID:        meta.SessionID,
		RunID:            record.RunID,
		TaskID:           meta.ParentTaskID,
		Title:            strings.TrimSpace(meta.TaskTitle),
		Status:           inferOutcomeStatus(meta, record),
		Summary:          taskOutcomeSummary(meta, record),
		ChangedPaths:     append([]string(nil), meta.TaskChangedPaths...),
		ChecksRun:        append([]string(nil), meta.TaskChecksRun...),
		EvidencePointers: append([]string(nil), meta.TaskEvidencePointers...),
		Followups:        append([]string(nil), meta.TaskFollowups...),
		ErrorKind:        strings.TrimSpace(meta.TaskErrorKind),
		Fingerprint:      knowledge.OutcomeFingerprint(meta.TaskTitle, meta.TaskChangedPaths, meta.TaskChecksRun),
	}
	if s.store != nil {
		saved, err := s.store.SaveTaskOutcome(context.Background(), outcome)
		if err != nil {
			return meta, domain.TaskOutcome{}, err
		}
		outcome = saved
	}
	return meta, outcome, nil
}

func inferOutcomeStatus(meta domain.SessionMetadata, record domain.RunRecord) string {
	if trimmed := strings.TrimSpace(meta.TaskStatus); trimmed != "" {
		return trimmed
	}
	switch record.Status {
	case domain.StatusCompleted:
		return domain.TaskStatusCompleted
	case domain.StatusCancelled:
		return domain.TaskStatusCancelled
	default:
		return domain.TaskStatusFailed
	}
}

func taskOutcomeSummary(meta domain.SessionMetadata, record domain.RunRecord) string {
	if trimmed := strings.TrimSpace(meta.TaskSummary); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(record.FinalOutput); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(record.CurrentTask)
}
