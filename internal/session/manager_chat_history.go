package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const chatHistoryFileName = "chatHistory.md"

func (m *Manager) ChatHistoryPath() string {
	return filepath.Join(filepath.Dir(m.root), chatHistoryFileName)
}

func (m *Manager) ReadChatHistory() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.ChatHistoryPath())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read chat history: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (m *Manager) AppendChatHistory(entry ChatHistoryEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	summary := strings.TrimSpace(entry.Summary)
	if summary == "" {
		return fmt.Errorf("chat history summary is required")
	}
	if entry.Speaker != ChatHistorySpeakerUser && entry.Speaker != ChatHistorySpeakerAssistant {
		return fmt.Errorf("unsupported chat history speaker %q", entry.Speaker)
	}
	pointer := strings.TrimSpace(entry.Pointer)

	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	path := m.ChatHistoryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create chat history dir: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open chat history: %w", err)
	}
	defer file.Close()

	if _, err := file.WriteString(formatChatHistoryEntry(createdAt, entry.SessionID, entry.RunID, entry.Speaker, summary, pointer)); err != nil {
		return fmt.Errorf("append chat history: %w", err)
	}
	return nil
}

func formatChatHistoryEntry(
	createdAt time.Time,
	sessionID string,
	runID string,
	speaker ChatHistorySpeaker,
	summary string,
	pointer string,
) string {
	var builder strings.Builder
	builder.WriteString("## ")
	builder.WriteString(createdAt.UTC().Format(time.RFC3339))
	builder.WriteString(" | ")
	builder.WriteString(string(speaker))
	builder.WriteString("\n")
	if strings.TrimSpace(sessionID) != "" {
		builder.WriteString("- session_id: ")
		builder.WriteString(strings.TrimSpace(sessionID))
		builder.WriteString("\n")
	}
	if strings.TrimSpace(runID) != "" {
		builder.WriteString("- run_id: ")
		builder.WriteString(strings.TrimSpace(runID))
		builder.WriteString("\n")
	}
	builder.WriteString("- summary: ")
	builder.WriteString(summary)
	if pointer != "" {
		builder.WriteString("\n- pointer: ")
		builder.WriteString(pointer)
	}
	builder.WriteString("\n\n")
	return builder.String()
}
