package session

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

type ChatHistorySpeaker string

const (
	ChatHistorySpeakerUser      ChatHistorySpeaker = "user"
	ChatHistorySpeakerAssistant ChatHistorySpeaker = "assistant"
)

type ChatHistoryEntry struct {
	CreatedAt time.Time
	SessionID string
	RunID     string
	Speaker   ChatHistorySpeaker
	Summary   string
	Pointer   string
}

const (
	chatHistoryUserPrompt      = "Summarize this user request for a rolling conversation digest in 1-3 sentences."
	chatHistoryAssistantPrompt = "Summarize this assistant response for a rolling conversation digest in 1-3 sentences."
	chatHistoryFallbackLimit   = 480
)

func (s *Service) ReadChatHistory() (string, error) {
	return s.manager.ReadChatHistory()
}

func (s *Service) AppendChatHistory(entry ChatHistoryEntry) error {
	return s.manager.AppendChatHistory(entry)
}

func (s *Service) SummarizeChatHistoryUser(ctx context.Context, meta domain.SessionMetadata, content string) (string, error) {
	return s.summarizeChatHistory(ctx, meta, chatHistoryUserPrompt, content)
}

func (s *Service) SummarizeChatHistoryAssistant(ctx context.Context, meta domain.SessionMetadata, content string) (string, error) {
	return s.summarizeChatHistory(ctx, meta, chatHistoryAssistantPrompt, content)
}

func (s *Service) AppendChatHistoryUserSummary(
	ctx context.Context,
	meta domain.SessionMetadata,
	runID string,
	content string,
) error {
	summary, err := s.SummarizeChatHistoryUser(ctx, meta, content)
	if err != nil {
		return err
	}
	return s.AppendChatHistory(ChatHistoryEntry{
		CreatedAt: time.Now(),
		SessionID: meta.SessionID,
		RunID:     runID,
		Speaker:   ChatHistorySpeakerUser,
		Summary:   summary,
		Pointer:   s.pointerForRun(meta.SessionID, runID, domain.SessionRecordUser),
	})
}

func (s *Service) AppendChatHistoryAssistantSummary(
	ctx context.Context,
	meta domain.SessionMetadata,
	runID string,
	content string,
) error {
	summary, err := s.SummarizeChatHistoryAssistant(ctx, meta, content)
	if err != nil {
		return err
	}
	return s.AppendChatHistory(ChatHistoryEntry{
		CreatedAt: time.Now(),
		SessionID: meta.SessionID,
		RunID:     runID,
		Speaker:   ChatHistorySpeakerAssistant,
		Summary:   summary,
		Pointer:   s.pointerForRun(meta.SessionID, runID, domain.SessionRecordAssistant),
	})
}

func (s *Service) summarizeChatHistory(
	ctx context.Context,
	meta domain.SessionMetadata,
	systemPrompt string,
	content string,
) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", fmt.Errorf("chat history content is required")
	}

	if s.runner == nil {
		return clipCompactLineLimit(trimmed, chatHistoryFallbackLimit), nil
	}

	summary, err := s.runner.Run(ctx, meta.Provider, meta.Model, systemPrompt, trimmed)
	if err != nil {
		return "", err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return clipCompactLineLimit(trimmed, chatHistoryFallbackLimit), nil
	}
	return summary, nil
}

func clipCompactLineLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func (s *Service) pointerForRun(sessionID string, runID string, recordType domain.SessionRecordType) string {
	records, err := s.manager.LoadRecords(sessionID)
	if err != nil {
		return ""
	}

	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		if record.RunID != runID || record.Type != recordType {
			continue
		}
		return FormatOTPointer([]int64{record.Seq})
	}
	return ""
}
