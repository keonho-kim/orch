package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/keonho-kim/orch/domain"
)

type Manager struct {
	root string
	mu   sync.RWMutex
}

func NewManager(root string) *Manager {
	return &Manager{root: root}
}

func NewSessionID(now time.Time) string {
	return now.Format("20060102-150405.000")
}

func (m *Manager) Create(
	workspacePath string,
	provider domain.Provider,
	model string,
	startedAt time.Time,
	parentSessionID string,
	parentRunID string,
	parentTaskID string,
	workerRole domain.AgentRole,
	taskTitle string,
	taskContract string,
	taskStatus string,
) (domain.SessionMetadata, error) {
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return domain.SessionMetadata{}, fmt.Errorf("create sessions dir: %w", err)
	}

	meta := domain.SessionMetadata{
		SessionID:       NewSessionID(startedAt),
		WorkspacePath:   workspacePath,
		ParentSessionID: strings.TrimSpace(parentSessionID),
		ParentRunID:     strings.TrimSpace(parentRunID),
		ParentTaskID:    strings.TrimSpace(parentTaskID),
		WorkerRole:      workerRole,
		TaskTitle:       strings.TrimSpace(taskTitle),
		TaskContract:    strings.TrimSpace(taskContract),
		TaskStatus:      strings.TrimSpace(taskStatus),
		Provider:        provider,
		Model:           model,
		Title:           "Untitled session",
		StartedAt:       startedAt,
		UpdatedAt:       startedAt,
	}

	if err := m.SaveMetadata(meta); err != nil {
		return domain.SessionMetadata{}, err
	}
	return meta, nil
}

func (m *Manager) AppendRecord(sessionID string, record domain.SessionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.recordsPath(sessionID)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open session record file: %w", err)
	}
	defer file.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal session record: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("append session record: %w", err)
	}
	return nil
}

func (m *Manager) LoadRecords(sessionID string) ([]domain.SessionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := m.recordsPath(sessionID)
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open session record file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 4*1024*1024)

	records := make([]domain.SessionRecord, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var record domain.SessionRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("decode session record: %w", err)
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan session records: %w", err)
	}

	return records, nil
}

func (m *Manager) SaveMetadata(meta domain.SessionMetadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return fmt.Errorf("create sessions dir: %w", err)
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session metadata: %w", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(m.metadataPath(meta.SessionID), data, 0o644); err != nil {
		return fmt.Errorf("write session metadata: %w", err)
	}
	return nil
}

func (m *Manager) LoadMetadata(sessionID string) (domain.SessionMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.metadataPath(sessionID))
	if err != nil {
		return domain.SessionMetadata{}, fmt.Errorf("read session metadata: %w", err)
	}

	var meta domain.SessionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return domain.SessionMetadata{}, fmt.Errorf("decode session metadata: %w", err)
	}
	return meta, nil
}

func (m *Manager) ListSessions(limit int) ([]domain.SessionMetadata, error) {
	return m.listMetadata(limit, false)
}

func (m *Manager) ListMetadata(limit int) ([]domain.SessionMetadata, error) {
	return m.listMetadata(limit, true)
}

func (m *Manager) listMetadata(limit int, includeMetadataOnly bool) ([]domain.SessionMetadata, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	sessions := make([]domain.SessionMetadata, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}

		sessionID := strings.TrimSuffix(entry.Name(), ".meta.json")
		if !includeMetadataOnly {
			recordInfo, err := os.Stat(m.recordsPath(sessionID))
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("stat session record file: %w", err)
			}
			if recordInfo.Size() == 0 {
				continue
			}
		}
		meta, err := m.LoadMetadata(sessionID)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, meta)
	}

	sort.Slice(sessions, func(i int, j int) bool {
		if sessions[i].UpdatedAt.Equal(sessions[j].UpdatedAt) {
			return sessions[i].SessionID > sessions[j].SessionID
		}
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	if limit > 0 && len(sessions) > limit {
		sessions = sessions[:limit]
	}
	return sessions, nil
}

func (m *Manager) LatestSessionID() (string, error) {
	sessions, err := m.ListSessions(1)
	if err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", fmt.Errorf("no saved sessions")
	}
	return sessions[0].SessionID, nil
}

func (m *Manager) recordsPath(sessionID string) string {
	return filepath.Join(m.root, sessionID+".jsonl")
}

func (m *Manager) metadataPath(sessionID string) string {
	return filepath.Join(m.root, sessionID+".meta.json")
}
