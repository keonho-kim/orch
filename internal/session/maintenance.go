package session

import (
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

type TitleInput struct {
	Records    []domain.SessionRecord
	Transcript string
}

type CompactInput struct {
	Records    []domain.SessionRecord
	ThroughSeq int64
	Transcript string
}

type RunStartUpdate struct {
	Provider   domain.Provider
	Model      string
	RunID      string
	WorkerRole domain.AgentRole
	TaskStatus string
	UpdatedAt  time.Time
}

func (s *Service) LoadTitleInput(meta domain.SessionMetadata) (TitleInput, error) {
	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return TitleInput{}, err
	}
	return TitleInput{
		Records:    records,
		Transcript: BuildTranscript(records, 0),
	}, nil
}

func (s *Service) LoadCompactInput(meta domain.SessionMetadata) (CompactInput, error) {
	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return CompactInput{}, err
	}
	throughSeq := int64(0)
	if len(records) > 0 {
		throughSeq = records[len(records)-1].Seq
	}
	return CompactInput{
		Records:    records,
		ThroughSeq: throughSeq,
		Transcript: BuildTranscriptWithPointers(records, throughSeq),
	}, nil
}

func (s *Service) ApplyTitle(meta domain.SessionMetadata, title string) (domain.SessionMetadata, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return meta, nil
	}
	return s.appendRecord(meta, domain.SessionRecord{
		Type:  domain.SessionRecordTitle,
		Title: title,
	})
}

func (s *Service) ApplyCompactSummary(meta domain.SessionMetadata, throughSeq int64, summary string) (domain.SessionMetadata, error) {
	if strings.TrimSpace(meta.SessionID) == "" {
		return meta, nil
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return meta, nil
	}

	updated, err := s.appendRecord(meta, domain.SessionRecord{
		Type:       domain.SessionRecordCompact,
		Content:    summary,
		ThroughSeq: throughSeq,
	})
	if err != nil {
		return meta, err
	}
	updated.Summary = summary
	updated.TokensSinceCompact = 0
	updated.UpdatedAt = time.Now()
	if err := s.manager.SaveMetadata(updated); err != nil {
		return meta, err
	}
	return updated, nil
}

func (s *Service) MarkFinalizeStarted(meta domain.SessionMetadata) (domain.SessionMetadata, error) {
	if strings.TrimSpace(meta.SessionID) == "" {
		return meta, nil
	}
	meta.FinalizePending = true
	meta.UpdatedAt = time.Now()
	return meta, s.manager.SaveMetadata(meta)
}

func (s *Service) MarkFinalized(meta domain.SessionMetadata, finalizedAt time.Time) (domain.SessionMetadata, error) {
	if strings.TrimSpace(meta.SessionID) == "" {
		return meta, nil
	}
	meta.FinalizePending = false
	meta.FinalizedAt = &finalizedAt
	meta.UpdatedAt = finalizedAt
	return meta, s.manager.SaveMetadata(meta)
}

func (s *Service) MarkRunStarted(meta domain.SessionMetadata, update RunStartUpdate) (domain.SessionMetadata, error) {
	if update.Provider != "" {
		meta.Provider = update.Provider
	}
	if trimmed := strings.TrimSpace(update.Model); trimmed != "" {
		meta.Model = trimmed
	}
	if trimmed := strings.TrimSpace(update.RunID); trimmed != "" {
		meta.LastRunID = trimmed
	}
	if update.WorkerRole != "" {
		meta.WorkerRole = update.WorkerRole
	}
	if trimmed := strings.TrimSpace(update.TaskStatus); trimmed != "" {
		meta.TaskStatus = trimmed
	}
	if update.UpdatedAt.IsZero() {
		update.UpdatedAt = time.Now()
	}
	meta.UpdatedAt = update.UpdatedAt
	return meta, s.manager.SaveMetadata(meta)
}

func FallbackSessionTitle(records []domain.SessionRecord, currentTitle string) string {
	for _, record := range records {
		if record.Type != domain.SessionRecordUser {
			continue
		}
		value := strings.TrimSpace(record.Content)
		if value == "" {
			continue
		}
		if len(value) > 72 {
			value = value[:72]
		}
		return value
	}
	return currentTitle
}
