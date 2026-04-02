package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	s.mu.Lock()
	meta := s.currentSession
	if meta.WorkerRole == "" {
		meta.WorkerRole = s.agentRole
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		meta.TaskStatus = trimmed
	}
	if trimmed := strings.TrimSpace(summary); trimmed != "" && strings.TrimSpace(meta.TaskSummary) == "" {
		meta.TaskSummary = trimmed
	}
	if len(changedPaths) > 0 && len(meta.TaskChangedPaths) == 0 {
		meta.TaskChangedPaths = normalizeTaskValues(changedPaths)
	}
	if len(checksRun) > 0 && len(meta.TaskChecksRun) == 0 {
		meta.TaskChecksRun = normalizeTaskValues(checksRun)
	}
	if len(evidencePointers) > 0 && len(meta.TaskEvidencePointers) == 0 {
		normalized := normalizeTaskValues(evidencePointers)
		if err := s.validateTaskEvidencePointers(normalized); err != nil {
			s.mu.Unlock()
			return err
		}
		meta.TaskEvidencePointers = normalized
	}
	if len(followups) > 0 && len(meta.TaskFollowups) == 0 {
		meta.TaskFollowups = normalizeTaskValues(followups)
	}
	if trimmed := strings.TrimSpace(errorKind); trimmed != "" && strings.TrimSpace(meta.TaskErrorKind) == "" {
		meta.TaskErrorKind = trimmed
	}
	meta.UpdatedAt = time.Now()
	s.currentSession = meta
	s.mu.Unlock()

	return s.sessions.SaveMetadata(meta)
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
			s.publish(UIEvent{Type: "snapshot", SessionID: updated.SessionID, Message: "Session title updated."})
		}
	}

	s.mu.RLock()
	settings := s.settings
	s.mu.RUnlock()
	if s.sessions.ShouldCompact(settings, meta) {
		if updated, compactErr := s.sessions.Compact(context.Background(), meta); compactErr == nil {
			s.setCurrentSessionIfActive(updated)
			s.publish(UIEvent{Type: "snapshot", SessionID: updated.SessionID, Message: "Session compacted."})
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
		pointer, err := session.ParseOTPointer(value)
		if err != nil {
			return err
		}
		if strings.TrimSpace(pointer.SessionID) == "" {
			continue
		}
		path := filepath.Join(s.paths.SessionsDir, pointer.SessionID+".jsonl")
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("evidence pointer session %s not found", pointer.SessionID)
			}
			return fmt.Errorf("stat evidence pointer session %s: %w", pointer.SessionID, err)
		}
	}
	return nil
}
