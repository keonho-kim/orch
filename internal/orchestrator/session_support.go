package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/session"
)

type maintenanceRunner struct {
	clients  map[domain.Provider]adapters.Client
	settings func(domain.Provider) domain.ProviderSettings
}

func (r maintenanceRunner) Run(ctx context.Context, provider domain.Provider, model string, systemPrompt string, userPrompt string) (string, error) {
	client, ok := r.clients[provider]
	if !ok {
		return "", fmt.Errorf("provider %s is not configured for maintenance", provider)
	}
	if r.settings == nil {
		return "", fmt.Errorf("provider settings resolver is not configured")
	}

	result, err := client.Chat(ctx, r.settings(provider), adapters.ChatRequest{
		Model: model,
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

	updated, err := s.sessions.Compact(context.Background(), meta)
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
	meta, err := s.sessions.LoadMetadata(sessionID)
	if err != nil {
		return err
	}
	updated, err := s.sessions.Finalize(context.Background(), meta)
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
	meta, err := s.sessions.LoadMetadata(sessionID)
	if err != nil {
		return
	}

	title, err := s.sessions.DeriveTitle(context.Background(), meta)
	if err == nil && strings.TrimSpace(title) != "" {
		if updated, updateErr := s.sessions.UpdateTitle(meta, title); updateErr == nil {
			meta = updated
			s.setCurrentSessionIfActive(updated)
			s.publish(UIEvent{Message: "Session title updated."})
		}
	}

	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if s.sessions.ShouldCompact(settings, meta) {
		if updated, compactErr := s.sessions.Compact(context.Background(), meta); compactErr == nil {
			s.setCurrentSessionIfActive(updated)
			s.publish(UIEvent{Message: "Session compacted."})
		}
	}
}

func (s *Service) runChatHistoryUserSummary(meta domain.SessionMetadata, runID string, prompt string) {
	if strings.TrimSpace(prompt) == "" {
		return
	}
	if err := s.sessions.AppendChatHistoryUserSummary(context.Background(), meta, runID, prompt); err != nil {
		_ = s.appendRunEvent(runID, "chat_history", fmt.Sprintf("Could not append user chatHistory summary: %v", err))
	}
}

func (s *Service) runChatHistoryAssistantSummary(sessionID string, runID string, content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	meta, err := s.sessions.LoadMetadata(sessionID)
	if err != nil {
		_ = s.appendRunEvent(runID, "chat_history", fmt.Sprintf("Could not load session metadata for chatHistory: %v", err))
		return
	}
	if err := s.sessions.AppendChatHistoryAssistantSummary(context.Background(), meta, runID, content); err != nil {
		_ = s.appendRunEvent(runID, "chat_history", fmt.Sprintf("Could not append assistant chatHistory summary: %v", err))
	}
}
