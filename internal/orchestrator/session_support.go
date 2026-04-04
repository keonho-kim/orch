package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/prompts"
	"github.com/keonho-kim/orch/internal/session"
)

func (s *Service) sessionContextMessages() ([]adapters.Message, error) {
	s.mu.RLock()
	meta := s.currentSession
	inherited := s.inheritedCtx
	s.mu.RUnlock()

	contextState, err := s.sessions.Context(meta)
	if err != nil {
		return nil, err
	}

	messages := make([]adapters.Message, 0, len(inherited.Records)+len(contextState.Records)+2)
	messages = append(messages, buildContextMessages(inherited)...)
	messages = append(messages, buildContextMessages(contextState)...)
	return messages, nil
}

func buildContextMessages(contextState session.Context) []adapters.Message {
	messages := make([]adapters.Message, 0, len(contextState.Records)+1)
	if strings.TrimSpace(contextState.Summary) != "" {
		messages = append(messages, adapters.Message{
			Role:    "system",
			Content: "Session compact summary:\n" + strings.TrimSpace(contextState.Summary),
		})
	}

	for _, record := range contextState.Records {
		switch record.Type {
		case domain.SessionRecordUser:
			messages = append(messages, adapters.Message{Role: "user", Content: record.Content})
		case domain.SessionRecordAssistant:
			messages = append(messages, adapters.Message{Role: "assistant", Content: record.Content})
		case domain.SessionRecordTool:
			messages = append(messages, adapters.Message{
				Role:       "tool",
				Name:       record.ToolName,
				ToolCallID: record.ToolCallID,
				Content:    record.Content,
			})
		}
	}

	return messages
}

func (s *Service) appendSessionUser(runID string, prompt string) error {
	s.mu.Lock()
	meta := s.currentSession
	s.mu.Unlock()

	updated, err := s.sessions.AppendUser(meta, runID, prompt)
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) appendSessionAssistant(runID string, result adapters.ChatResult) error {
	s.mu.Lock()
	meta := s.currentSession
	s.mu.Unlock()

	updated, err := s.sessions.AppendAssistant(meta, runID, result.Content, result.Usage)
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) appendSessionTool(runID string, result domain.ToolResult) error {
	s.mu.Lock()
	meta := s.currentSession
	s.mu.Unlock()

	updated, err := s.sessions.AppendTool(meta, runID, result)
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) appendSessionContextSnapshot(runID string, snapshot domain.ContextSnapshot) error {
	s.mu.Lock()
	meta := s.currentSession
	s.mu.Unlock()

	updated, err := s.sessions.AppendContextSnapshot(meta, runID, snapshot)
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) updateCurrentSessionTaskMetadata(
	status string,
	summary string,
	changedPaths []string,
	checksRun []string,
	evidencePointers []string,
	followups []string,
	errorKind string,
) error {
	s.mu.RLock()
	meta := s.currentSession
	s.mu.RUnlock()

	updated, err := s.sessions.UpdateTaskMetadata(meta, session.TaskMetadataUpdate{
		WorkerRole:       s.agentRole,
		Status:           status,
		Summary:          summary,
		ChangedPaths:     changedPaths,
		ChecksRun:        checksRun,
		EvidencePointers: evidencePointers,
		Followups:        followups,
		ErrorKind:        errorKind,
	})
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) setCurrentSessionIfActive(meta domain.SessionMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentSession.SessionID == meta.SessionID {
		s.currentSession = meta
	}
}

func (s *Service) forceCompactCurrentSession() error {
	if s.ActiveRunCount() > 0 {
		return fmt.Errorf("cannot compact while a run is active")
	}

	s.mu.RLock()
	meta := s.currentSession
	s.mu.RUnlock()

	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	summary, throughSeq, err := s.generateCompactSummary(context.Background(), meta)
	if err != nil {
		return err
	}
	updated, err := s.sessions.ApplyCompactSummary(meta, throughSeq, summary)
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) ForceCompact() error {
	return s.forceCompactCurrentSession()
}

func (s *Service) finalizeSessionByID(sessionID string) error {
	meta, err := s.sessionManager.LoadMetadata(sessionID)
	if err != nil {
		return err
	}

	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	updated, err := s.sessions.MarkFinalizeStarted(meta)
	if err != nil {
		return err
	}

	title, err := s.deriveSessionTitle(context.Background(), updated)
	if err != nil {
		return err
	}
	updated, err = s.sessions.ApplyTitle(updated, title)
	if err != nil {
		return err
	}

	summary, throughSeq, err := s.generateCompactSummary(context.Background(), updated)
	if err != nil {
		return err
	}
	updated, err = s.sessions.ApplyCompactSummary(updated, throughSeq, summary)
	if err != nil {
		return err
	}

	updated, err = s.sessions.MarkFinalized(updated, time.Now())
	if err != nil {
		return err
	}
	s.setCurrentSessionIfActive(updated)
	return nil
}

func (s *Service) FinalizeCurrentSession() error {
	s.mu.RLock()
	sessionID := s.currentSession.SessionID
	s.mu.RUnlock()
	return s.finalizeSessionByID(sessionID)
}

func (s *Service) runSessionMaintenance(sessionID string) {
	meta, err := s.sessionManager.LoadMetadata(sessionID)
	if err != nil {
		return
	}

	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	title, err := s.deriveSessionTitle(context.Background(), meta)
	if err == nil && strings.TrimSpace(title) != "" {
		if updated, updateErr := s.sessions.ApplyTitle(meta, title); updateErr == nil {
			meta = updated
			s.setCurrentSessionIfActive(updated)
			s.publish(ServiceEvent{Type: "snapshot", SessionID: updated.SessionID, Message: "Session title updated."})
		}
	}

	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if session.ShouldCompact(settings, meta) {
		summary, throughSeq, compactErr := s.generateCompactSummary(context.Background(), meta)
		if compactErr == nil {
			updated, applyErr := s.sessions.ApplyCompactSummary(meta, throughSeq, summary)
			if applyErr != nil {
				return
			}
			s.setCurrentSessionIfActive(updated)
			s.publish(ServiceEvent{Type: "snapshot", SessionID: updated.SessionID, Message: "Session compacted."})
		}
	}
}

func (s *Service) runChatHistoryUserSummary(meta domain.SessionMetadata, runID string, prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	if err := s.appendChatHistorySummary(context.Background(), meta, runID, prompt, session.ChatHistorySpeakerUser, prompts.ChatHistoryUserPrompt()); err != nil {
		_ = s.appendRunEvent(runID, "chat_history", fmt.Sprintf("Could not append user chatHistory summary: %v", err))
	}
}

func (s *Service) runChatHistoryAssistantSummary(sessionID string, runID string, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	meta, err := s.sessionManager.LoadMetadata(sessionID)
	if err != nil {
		_ = s.appendRunEvent(runID, "chat_history", fmt.Sprintf("Could not load session metadata for chatHistory: %v", err))
		return
	}
	if err := s.appendChatHistorySummary(context.Background(), meta, runID, content, session.ChatHistorySpeakerAssistant, prompts.ChatHistoryAssistantPrompt()); err != nil {
		_ = s.appendRunEvent(runID, "chat_history", fmt.Sprintf("Could not append assistant chatHistory summary: %v", err))
	}
}

func (s *Service) deriveSessionTitle(ctx context.Context, meta domain.SessionMetadata) (string, error) {
	input, err := s.sessions.LoadTitleInput(meta)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(input.Transcript) != "" {
		title, runErr := s.runMaintenancePrompt(ctx, meta, prompts.SessionTitlePrompt(), input.Transcript)
		if runErr == nil && strings.TrimSpace(title) != "" {
			return title, nil
		}
	}
	return session.FallbackSessionTitle(input.Records, meta.Title), nil
}

func (s *Service) generateCompactSummary(ctx context.Context, meta domain.SessionMetadata) (string, int64, error) {
	input, err := s.sessions.LoadCompactInput(meta)
	if err != nil {
		return "", 0, err
	}
	if len(input.Records) == 0 {
		return "", 0, nil
	}
	if strings.TrimSpace(input.Transcript) == "" {
		return session.BuildCompactSummary(input.Records, input.ThroughSeq), input.ThroughSeq, nil
	}

	topicsRaw, runErr := s.runMaintenancePrompt(ctx, meta, prompts.SessionTopicsPrompt(), input.Transcript)
	if runErr != nil {
		return session.BuildCompactSummary(input.Records, input.ThroughSeq), input.ThroughSeq, nil
	}

	topics := session.ParseCompactTopics(topicsRaw)
	if len(topics) == 0 {
		topics = session.FallbackCompactTopics(input.Records, input.ThroughSeq)
	}
	if len(topics) == 0 {
		return session.BuildCompactSummary(input.Records, input.ThroughSeq), input.ThroughSeq, nil
	}

	parts := make([]string, 0, len(topics)+1)
	parts = append(parts, "Compact summary")
	for _, topic := range topics {
		summary, topicErr := s.runMaintenancePrompt(
			ctx,
			meta,
			prompts.SessionTopicSummaryPrompt(topic.Title, session.JoinCompactLines(topic.Lines)),
			input.Transcript,
		)
		summary = strings.TrimSpace(summary)
		if topicErr != nil || summary == "" {
			summary = session.FallbackCompactTopicSummary(input.Records, topic)
		}
		parts = append(parts, session.RenderPointerParagraph(summary, topic.Lines))
	}

	return strings.Join(parts, "\n\n"), input.ThroughSeq, nil
}

func (s *Service) appendChatHistorySummary(
	ctx context.Context,
	meta domain.SessionMetadata,
	runID string,
	content string,
	speaker session.ChatHistorySpeaker,
	systemPrompt string,
) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	summary, err := s.runMaintenancePrompt(ctx, meta, systemPrompt, content)
	if err != nil {
		return err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = clipChatHistorySummary(content)
	}

	if speaker == session.ChatHistorySpeakerUser {
		return s.sessions.AppendChatHistoryUser(meta, runID, summary)
	}
	return s.sessions.AppendChatHistoryAssistant(meta, runID, summary)
}

func (s *Service) runMaintenancePrompt(ctx context.Context, meta domain.SessionMetadata, systemPrompt string, userPrompt string) (string, error) {
	client, ok := s.clients[meta.Provider]
	if !ok {
		return "", fmt.Errorf("provider %s is not configured for maintenance", meta.Provider)
	}

	s.mu.RLock()
	settings := s.settings.ConfigFor(meta.Provider)
	s.mu.RUnlock()

	result, err := client.Chat(ctx, settings, adapters.ChatRequest{
		Model: meta.Model,
		Messages: []adapters.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}, nil)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Content), nil
}

func clipChatHistorySummary(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= session.ChatHistoryFallbackLimit {
		return value
	}
	return value[:session.ChatHistoryFallbackLimit]
}
