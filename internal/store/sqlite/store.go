package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"orch/domain"
)

const (
	defaultProviderKey = "default_provider"
	schemaVersion      = 3
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	for _, statement := range migrationStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			if isIgnorableMigrationError(err) {
				continue
			}
			return fmt.Errorf("run migration: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_version (id, version, updated_at)
		VALUES (1, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET version = excluded.version, updated_at = excluded.updated_at
	`, schemaVersion); err != nil {
		return fmt.Errorf("upsert schema version: %w", err)
	}

	return tx.Commit()
}

func isIgnorableMigrationError(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "duplicate column name")
}

func (s *Store) LoadSettings(ctx context.Context) (domain.Settings, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, defaultProviderKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Settings{}, nil
	}
	if err != nil {
		return domain.Settings{}, fmt.Errorf("load settings: %w", err)
	}

	provider, err := domain.ParseProvider(raw)
	if err != nil {
		return domain.Settings{}, fmt.Errorf("parse default provider: %w", err)
	}

	return domain.Settings{DefaultProvider: provider}, nil
}

func (s *Store) SaveDefaultProvider(ctx context.Context, provider domain.Provider) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, defaultProviderKey, provider.String())
	if err != nil {
		return fmt.Errorf("save default provider: %w", err)
	}

	return nil
}

func (s *Store) AddHistory(ctx context.Context, prompt string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO history (prompt, created_at)
		VALUES (?, CURRENT_TIMESTAMP)
	`, prompt)
	if err != nil {
		return fmt.Errorf("insert history: %w", err)
	}

	return nil
}

func (s *Store) ListHistory(ctx context.Context, limit int) ([]domain.HistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, prompt, created_at
		FROM history
		ORDER BY id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var entries []domain.HistoryEntry
	for rows.Next() {
		var entry domain.HistoryEntry
		var createdAt string
		if err := rows.Scan(&entry.ID, &entry.Prompt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		entry.CreatedAt = parseSQLiteTime(createdAt)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

func (s *Store) NextRunID(ctx context.Context) (string, error) {
	var latest sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT run_id
		FROM runs
		ORDER BY created_at DESC, rowid DESC
		LIMIT 1
	`).Scan(&latest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("load latest run id: %w", err)
	}
	if !latest.Valid {
		return "R1", nil
	}

	var number int
	if _, err := fmt.Sscanf(latest.String, "R%d", &number); err != nil {
		return "", fmt.Errorf("parse latest run id %q: %w", latest.String, err)
	}

	return fmt.Sprintf("R%d", number+1), nil
}

func (s *Store) CreateRun(ctx context.Context, record domain.RunRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO runs (
			run_id, mode, provider, model, prompt, current_task, status, workspace_path,
			current_cwd, ralph_iteration, final_output, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`,
		record.RunID,
		record.Mode.String(),
		record.Provider.String(),
		record.Model,
		record.Prompt,
		record.CurrentTask,
		string(record.Status),
		record.WorkspacePath,
		record.CurrentCwd,
		record.RalphIteration,
		record.FinalOutput,
	)
	if err != nil {
		return fmt.Errorf("insert run %s: %w", record.RunID, err)
	}

	return nil
}

func (s *Store) UpdateRun(ctx context.Context, record domain.RunRecord) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE runs
		SET mode = ?, provider = ?, model = ?, prompt = ?, current_task = ?, status = ?,
		    workspace_path = ?, current_cwd = ?, ralph_iteration = ?, final_output = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE run_id = ?
	`,
		record.Mode.String(),
		record.Provider.String(),
		record.Model,
		record.Prompt,
		record.CurrentTask,
		string(record.Status),
		record.WorkspacePath,
		record.CurrentCwd,
		record.RalphIteration,
		record.FinalOutput,
		record.RunID,
	)
	if err != nil {
		return fmt.Errorf("update run %s: %w", record.RunID, err)
	}

	return nil
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]domain.RunRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT run_id, mode, provider, model, prompt, current_task, status,
		       workspace_path, current_cwd, ralph_iteration, final_output, created_at, updated_at
		FROM runs
		ORDER BY created_at DESC, rowid DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var records []domain.RunRecord
	for rows.Next() {
		var record domain.RunRecord
		var modeRaw string
		var providerRaw string
		var createdAt string
		var updatedAt string
		if err := rows.Scan(
			&record.RunID,
			&modeRaw,
			&providerRaw,
			&record.Model,
			&record.Prompt,
			&record.CurrentTask,
			&record.Status,
			&record.WorkspacePath,
			&record.CurrentCwd,
			&record.RalphIteration,
			&record.FinalOutput,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}

		mode, err := domain.ParseRunMode(modeRaw)
		if err != nil {
			return nil, fmt.Errorf("parse run mode: %w", err)
		}
		provider, err := domain.ParseProvider(providerRaw)
		if err != nil {
			return nil, fmt.Errorf("parse run provider: %w", err)
		}

		record.Mode = mode
		record.Provider = provider
		if record.CurrentCwd == "" {
			record.CurrentCwd = record.WorkspacePath
		}
		record.CreatedAt = parseSQLiteTime(createdAt)
		record.UpdatedAt = parseSQLiteTime(updatedAt)
		records = append(records, record)
	}

	return records, rows.Err()
}

func (s *Store) AppendRunEvent(ctx context.Context, event domain.RunEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO run_events (run_id, kind, message, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`, event.RunID, event.Kind, event.Message)
	if err != nil {
		return fmt.Errorf("insert run event: %w", err)
	}

	return nil
}

func (s *Store) LoadPlanCache(ctx context.Context) (domain.PlanCache, error) {
	var cache domain.PlanCache
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT source_run_id, content, updated_at
		FROM plan_cache
		WHERE id = 1
	`).Scan(&cache.SourceRunID, &cache.Content, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PlanCache{}, nil
	}
	if err != nil {
		return domain.PlanCache{}, fmt.Errorf("load plan cache: %w", err)
	}
	cache.UpdatedAt = parseSQLiteTime(updatedAt)
	return cache, nil
}

func (s *Store) SavePlanCache(ctx context.Context, cache domain.PlanCache) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plan_cache (id, source_run_id, content, updated_at)
		VALUES (1, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			source_run_id = excluded.source_run_id,
			content = excluded.content,
			updated_at = excluded.updated_at
	`, cache.SourceRunID, cache.Content)
	if err != nil {
		return fmt.Errorf("save plan cache: %w", err)
	}
	return nil
}

func parseSQLiteTime(value string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05Z07:00",
	}

	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}

	return time.Now()
}
