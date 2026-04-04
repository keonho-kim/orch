package session

import (
	"time"

	"github.com/keonho-kim/orch/domain"
)

type ChatHistorySpeaker string

const (
	ChatHistorySpeakerUser      ChatHistorySpeaker = "user"
	ChatHistorySpeakerAssistant ChatHistorySpeaker = "assistant"
	ChatHistoryFallbackLimit                       = 480
)

type ChatHistoryEntry struct {
	CreatedAt time.Time
	SessionID string
	RunID     string
	Speaker   ChatHistorySpeaker
	Summary   string
	Pointer   string
}

func (s *Service) ReadChatHistory() (string, error) {
	return s.manager.ReadChatHistory()
}

func (s *Service) ReadChatHistoryRecent(limitEntries int, maxBytes int) (string, error) {
	return s.manager.ReadChatHistoryRecent(limitEntries, maxBytes)
}

func (s *Service) AppendChatHistory(entry ChatHistoryEntry) error {
	return s.manager.AppendChatHistory(entry)
}

func (s *Service) AppendChatHistoryUser(meta domain.SessionMetadata, runID string, summary string) error {
	return s.appendChatHistory(meta, runID, ChatHistorySpeakerUser, summary, domain.SessionRecordUser)
}

func (s *Service) AppendChatHistoryAssistant(meta domain.SessionMetadata, runID string, summary string) error {
	return s.appendChatHistory(meta, runID, ChatHistorySpeakerAssistant, summary, domain.SessionRecordAssistant)
}

func (s *Service) appendChatHistory(
	meta domain.SessionMetadata,
	runID string,
	speaker ChatHistorySpeaker,
	summary string,
	recordType domain.SessionRecordType,
) error {
	return s.AppendChatHistory(ChatHistoryEntry{
		CreatedAt: time.Now(),
		SessionID: meta.SessionID,
		RunID:     runID,
		Speaker:   speaker,
		Summary:   summary,
		Pointer:   s.pointerForRun(meta.SessionID, runID, recordType),
	})
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
