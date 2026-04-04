package session

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

type TaskMetadataUpdate struct {
	WorkerRole       domain.AgentRole
	Status           string
	Summary          string
	ChangedPaths     []string
	ChecksRun        []string
	EvidencePointers []string
	Followups        []string
	ErrorKind        string
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

func (s *Service) UpdateTaskMetadata(meta domain.SessionMetadata, update TaskMetadataUpdate) (domain.SessionMetadata, error) {
	if meta.WorkerRole == "" && update.WorkerRole != "" {
		meta.WorkerRole = update.WorkerRole
	}
	if trimmed := strings.TrimSpace(update.Status); trimmed != "" {
		meta.TaskStatus = trimmed
	}
	if trimmed := strings.TrimSpace(update.Summary); trimmed != "" && strings.TrimSpace(meta.TaskSummary) == "" {
		meta.TaskSummary = trimmed
	}
	if len(update.ChangedPaths) > 0 && len(meta.TaskChangedPaths) == 0 {
		meta.TaskChangedPaths = normalizeTaskValues(update.ChangedPaths)
	}
	if len(update.ChecksRun) > 0 && len(meta.TaskChecksRun) == 0 {
		meta.TaskChecksRun = normalizeTaskValues(update.ChecksRun)
	}
	if len(update.EvidencePointers) > 0 && len(meta.TaskEvidencePointers) == 0 {
		normalized := normalizeTaskValues(update.EvidencePointers)
		if err := s.validateTaskEvidencePointers(normalized); err != nil {
			return meta, err
		}
		meta.TaskEvidencePointers = normalized
	}
	if len(update.Followups) > 0 && len(meta.TaskFollowups) == 0 {
		meta.TaskFollowups = normalizeTaskValues(update.Followups)
	}
	if trimmed := strings.TrimSpace(update.ErrorKind); trimmed != "" && strings.TrimSpace(meta.TaskErrorKind) == "" {
		meta.TaskErrorKind = trimmed
	}
	meta.UpdatedAt = time.Now()
	return meta, s.manager.SaveMetadata(meta)
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

func normalizeTaskValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func (s *Service) validateTaskEvidencePointers(values []string) error {
	for _, value := range values {
		pointer, err := ParseOTPointer(value)
		if err != nil {
			return err
		}
		if strings.TrimSpace(pointer.SessionID) == "" {
			continue
		}
		if _, err := os.Stat(s.manager.recordsPath(pointer.SessionID)); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("evidence pointer session %s not found", pointer.SessionID)
			}
			return fmt.Errorf("stat evidence pointer session %s: %w", pointer.SessionID, err)
		}
	}
	return nil
}
