package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

type Context struct {
	Summary string
	Records []domain.SessionRecord
}

type Service struct {
	manager *Manager
}

func NewService(manager *Manager) *Service {
	return &Service{manager: manager}
}

func (s *Service) AppendContextSnapshot(meta domain.SessionMetadata, runID string, snapshot domain.ContextSnapshot) (domain.SessionMetadata, error) {
	return s.appendRecord(meta, domain.SessionRecord{
		RunID:           runID,
		Type:            domain.SessionRecordContext,
		ContextSnapshot: &snapshot,
	})
}

func (s *Service) LatestContextSnapshot(sessionID string, runID string) (domain.ContextSnapshot, error) {
	records, err := s.manager.LoadRecords(sessionID)
	if err != nil {
		return domain.ContextSnapshot{}, err
	}
	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		if record.Type != domain.SessionRecordContext || record.ContextSnapshot == nil {
			continue
		}
		if strings.TrimSpace(runID) != "" && record.RunID != runID {
			continue
		}
		return *record.ContextSnapshot, nil
	}
	return domain.ContextSnapshot{}, fmt.Errorf("no context snapshot found")
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

func (s *Service) AppendUser(meta domain.SessionMetadata, runID string, prompt string) (domain.SessionMetadata, error) {
	return s.appendRecord(meta, domain.SessionRecord{
		RunID:   runID,
		Type:    domain.SessionRecordUser,
		Content: prompt,
	})
}

func (s *Service) AppendAssistant(meta domain.SessionMetadata, runID string, content string, usage domain.UsageStats) (domain.SessionMetadata, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return meta, nil
	}

	return s.appendRecord(meta, domain.SessionRecord{
		RunID:   runID,
		Type:    domain.SessionRecordAssistant,
		Content: content,
		Usage:   usage,
	})
}

func (s *Service) AppendTool(meta domain.SessionMetadata, runID string, result domain.ToolResult) (domain.SessionMetadata, error) {
	return s.appendRecord(meta, domain.SessionRecord{
		RunID:      runID,
		Type:       domain.SessionRecordTool,
		Content:    result.Content,
		ToolName:   result.Name,
		ToolCallID: result.ToolCallID,
	})
}

func (s *Service) appendRecord(meta domain.SessionMetadata, record domain.SessionRecord) (domain.SessionMetadata, error) {
	meta.LastSequence++
	record.Seq = meta.LastSequence
	record.SessionID = meta.SessionID
	record.CreatedAt = time.Now()
	meta.UpdatedAt = record.CreatedAt
	if record.RunID != "" {
		meta.LastRunID = record.RunID
	}
	switch record.Type {
	case domain.SessionRecordCompact:
		meta.LastCompactedSeq = record.ThroughSeq
	case domain.SessionRecordTitle:
		if strings.TrimSpace(record.Title) != "" {
			meta.Title = strings.TrimSpace(record.Title)
		}
	}
	if record.Usage.TotalTokens > 0 {
		meta.TotalTokens += record.Usage.TotalTokens
		meta.TokensSinceCompact += record.Usage.TotalTokens
	}
	if err := s.manager.AppendRecord(meta.SessionID, record); err != nil {
		return meta, err
	}
	if err := s.manager.SaveMetadata(meta); err != nil {
		return meta, err
	}
	return meta, nil
}
