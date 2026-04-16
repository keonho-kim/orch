package session

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func (s *Service) ListSessions(limit int) ([]domain.SessionMetadata, error) {
	return s.manager.ListSessions(limit)
}

func (s *Service) LatestSessionID() (string, error) {
	return s.manager.LatestSessionID()
}

func (s *Service) ListTasks(parentSessionID string, parentRunID string, statusFilter string) ([]domain.TaskView, error) {
	metadata, err := s.manager.ListMetadata(0)
	if err != nil {
		return nil, err
	}

	tasks := make([]domain.TaskView, 0)
	for _, meta := range metadata {
		if strings.TrimSpace(meta.ParentSessionID) != strings.TrimSpace(parentSessionID) {
			continue
		}
		if strings.TrimSpace(parentRunID) != "" && strings.TrimSpace(meta.ParentRunID) != strings.TrimSpace(parentRunID) {
			continue
		}
		task := s.taskViewFromMetadata(meta)
		if strings.TrimSpace(statusFilter) != "" && task.Status != strings.TrimSpace(statusFilter) {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *Service) GetTask(parentSessionID string, taskID string) (domain.TaskView, error) {
	metadata, err := s.manager.ListMetadata(0)
	if err != nil {
		return domain.TaskView{}, err
	}

	for _, meta := range metadata {
		if strings.TrimSpace(meta.ParentSessionID) != strings.TrimSpace(parentSessionID) {
			continue
		}
		if strings.TrimSpace(meta.ParentTaskID) != strings.TrimSpace(taskID) {
			continue
		}
		return s.taskViewFromMetadata(meta), nil
	}
	return domain.TaskView{}, fmt.Errorf("task %s not found", strings.TrimSpace(taskID))
}

func (s *Service) Context(meta domain.SessionMetadata) (Context, error) {
	if strings.TrimSpace(meta.SessionID) == "" {
		return Context{}, nil
	}

	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return Context{}, err
	}

	contextRecords := make([]domain.SessionRecord, 0, len(records))
	for _, record := range records {
		if record.Seq <= meta.LastCompactedSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser, domain.SessionRecordAssistant, domain.SessionRecordTool:
			contextRecords = append(contextRecords, record)
		}
	}

	return Context{
		Summary: strings.TrimSpace(meta.Summary),
		Records: contextRecords,
	}, nil
}

func (s *Service) taskViewFromMetadata(meta domain.SessionMetadata) domain.TaskView {
	return domain.TaskView{
		TaskID:               strings.TrimSpace(meta.ParentTaskID),
		Title:                strings.TrimSpace(meta.TaskTitle),
		Status:               inferTaskStatus(meta),
		ParentSessionID:      strings.TrimSpace(meta.ParentSessionID),
		ParentRunID:          strings.TrimSpace(meta.ParentRunID),
		ChildSessionID:       strings.TrimSpace(meta.SessionID),
		ChildRunID:           strings.TrimSpace(meta.LastRunID),
		WorkerRole:           meta.WorkerRole.String(),
		Provider:             meta.Provider.String(),
		Model:                strings.TrimSpace(meta.Model),
		TaskSummary:          strings.TrimSpace(meta.TaskSummary),
		TaskChangedPaths:     append([]string(nil), meta.TaskChangedPaths...),
		TaskChecksRun:        append([]string(nil), meta.TaskChecksRun...),
		TaskEvidencePointers: append([]string(nil), meta.TaskEvidencePointers...),
		TaskFollowups:        append([]string(nil), meta.TaskFollowups...),
		TaskErrorKind:        strings.TrimSpace(meta.TaskErrorKind),
		FinalOutputExcerpt:   s.taskFinalOutputExcerpt(meta),
		StartedAt:            meta.StartedAt,
		UpdatedAt:            meta.UpdatedAt,
		FinalizedAt:          meta.FinalizedAt,
	}
}

func (s *Service) taskFinalOutputExcerpt(meta domain.SessionMetadata) string {
	if strings.TrimSpace(meta.TaskSummary) != "" {
		return clipCompactLineLimit(meta.TaskSummary, 240)
	}

	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return ""
	}
	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		switch record.Type {
		case domain.SessionRecordAssistant, domain.SessionRecordTool, domain.SessionRecordUser:
			if strings.TrimSpace(record.Content) == "" {
				continue
			}
			return clipCompactLineLimit(record.Content, 240)
		}
	}
	return ""
}

func inferTaskStatus(meta domain.SessionMetadata) string {
	status := strings.TrimSpace(meta.TaskStatus)
	if status != "" {
		return status
	}
	if meta.FinalizedAt != nil {
		return domain.TaskStatusCompleted
	}
	if strings.TrimSpace(meta.LastRunID) != "" {
		return domain.TaskStatusRunning
	}
	return domain.TaskStatusQueued
}
