package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/session"
)

func (s *Store) UpsertSession(ctx context.Context, meta domain.SessionMetadata) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (
			session_id, workspace_path, parent_session_id, parent_run_id, parent_task_id,
			worker_role, task_title, task_contract, task_status, task_summary,
			task_changed_paths, task_checks_run, task_evidence_pointers, task_followups, task_error_kind,
			provider, model, title, summary, started_at, updated_at, last_sequence, last_compacted_seq,
			tokens_since_compact, total_tokens, finalize_pending, finalized_at, last_run_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id) DO UPDATE SET
			workspace_path = excluded.workspace_path,
			parent_session_id = excluded.parent_session_id,
			parent_run_id = excluded.parent_run_id,
			parent_task_id = excluded.parent_task_id,
			worker_role = excluded.worker_role,
			task_title = excluded.task_title,
			task_contract = excluded.task_contract,
			task_status = excluded.task_status,
			task_summary = excluded.task_summary,
			task_changed_paths = excluded.task_changed_paths,
			task_checks_run = excluded.task_checks_run,
			task_evidence_pointers = excluded.task_evidence_pointers,
			task_followups = excluded.task_followups,
			task_error_kind = excluded.task_error_kind,
			provider = excluded.provider,
			model = excluded.model,
			title = excluded.title,
			summary = excluded.summary,
			started_at = excluded.started_at,
			updated_at = excluded.updated_at,
			last_sequence = excluded.last_sequence,
			last_compacted_seq = excluded.last_compacted_seq,
			tokens_since_compact = excluded.tokens_since_compact,
			total_tokens = excluded.total_tokens,
			finalize_pending = excluded.finalize_pending,
			finalized_at = excluded.finalized_at,
			last_run_id = excluded.last_run_id
	`,
		meta.SessionID,
		meta.WorkspacePath,
		meta.ParentSessionID,
		meta.ParentRunID,
		meta.ParentTaskID,
		meta.WorkerRole.String(),
		meta.TaskTitle,
		meta.TaskContract,
		meta.TaskStatus,
		meta.TaskSummary,
		encodeStringSlice(meta.TaskChangedPaths),
		encodeStringSlice(meta.TaskChecksRun),
		encodeStringSlice(meta.TaskEvidencePointers),
		encodeStringSlice(meta.TaskFollowups),
		meta.TaskErrorKind,
		meta.Provider.String(),
		meta.Model,
		meta.Title,
		meta.Summary,
		meta.StartedAt.Format(sqliteTimeLayout),
		meta.UpdatedAt.Format(sqliteTimeLayout),
		meta.LastSequence,
		meta.LastCompactedSeq,
		meta.TokensSinceCompact,
		meta.TotalTokens,
		boolToInt(meta.FinalizePending),
		nullableSQLiteTime(meta.FinalizedAt),
		meta.LastRunID,
	)
	if err != nil {
		return fmt.Errorf("upsert session %s: %w", meta.SessionID, err)
	}
	return nil
}

func (s *Store) AppendSessionMessage(ctx context.Context, record domain.SessionRecord) error {
	contextSnapshot := ""
	if record.ContextSnapshot != nil {
		contextSnapshot = encodeJSON(record.ContextSnapshot)
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO messages (
			session_id, seq, run_id, type, content, title, tool_name, tool_call_id,
			through_seq, context_snapshot, created_at, usage_prompt_tokens,
			usage_completion_tokens, usage_total_tokens
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.SessionID,
		record.Seq,
		record.RunID,
		string(record.Type),
		record.Content,
		record.Title,
		record.ToolName,
		record.ToolCallID,
		record.ThroughSeq,
		contextSnapshot,
		record.CreatedAt.Format(sqliteTimeLayout),
		record.Usage.PromptTokens,
		record.Usage.CompletionTokens,
		record.Usage.TotalTokens,
	)
	if err != nil {
		return fmt.Errorf("append session message: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("session message rows affected: %w", err)
	}
	if rows == 0 {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO messages_fts (session_id, run_id, seq, type, title, content)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		record.SessionID,
		record.RunID,
		record.Seq,
		string(record.Type),
		record.Title,
		record.Content,
	); err != nil {
		return fmt.Errorf("append messages fts: %w", err)
	}
	return nil
}

func (s *Store) SearchSessionMessages(
	ctx context.Context,
	workspacePath string,
	query string,
	limit int,
) ([]domain.SessionSearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.session_id, m.run_id, m.seq, m.type, m.title,
		       snippet(messages_fts, 5, '[', ']', '...', 18),
		       m.created_at
		FROM messages_fts
		JOIN messages m
		  ON m.session_id = messages_fts.session_id
		 AND m.seq = CAST(messages_fts.seq AS INTEGER)
		JOIN sessions sess
		  ON sess.session_id = m.session_id
		WHERE messages_fts MATCH ?
		  AND (? = '' OR sess.workspace_path = ?)
		ORDER BY bm25(messages_fts), m.created_at DESC
		LIMIT ?
	`, ftsPhraseQuery(query), strings.TrimSpace(workspacePath), strings.TrimSpace(workspacePath), limit)
	if err != nil {
		return nil, fmt.Errorf("search session messages: %w", err)
	}
	defer rows.Close()

	results := make([]domain.SessionSearchResult, 0, limit)
	for rows.Next() {
		var item domain.SessionSearchResult
		var createdAt string
		if err := rows.Scan(
			&item.SessionID,
			&item.RunID,
			&item.RecordSeq,
			&item.RecordType,
			&item.Title,
			&item.Snippet,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan session search result: %w", err)
		}
		item.SelectionReason = "matched session transcript"
		item.EvidencePointer = session.FormatOTPointerForSession(item.SessionID, []int64{item.RecordSeq})
		item.CreatedAt = parseSQLiteTime(createdAt)
		results = append(results, item)
	}
	return results, rows.Err()
}

func (s *Store) LoadSessionContextSnapshot(
	ctx context.Context,
	sessionID string,
	runID string,
) (domain.ContextSnapshot, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `
		SELECT context_snapshot
		FROM messages
		WHERE session_id = ?
		  AND (? = '' OR run_id = ?)
		  AND type = ?
		  AND context_snapshot <> ''
		ORDER BY seq DESC
		LIMIT 1
	`, sessionID, strings.TrimSpace(runID), strings.TrimSpace(runID), string(domain.SessionRecordContext)).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.ContextSnapshot{}, fmt.Errorf("no context snapshot found")
		}
		return domain.ContextSnapshot{}, fmt.Errorf("load context snapshot: %w", err)
	}

	var snapshot domain.ContextSnapshot
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return domain.ContextSnapshot{}, fmt.Errorf("decode context snapshot: %w", err)
	}
	return snapshot, nil
}

const sqliteTimeLayout = time.RFC3339Nano

func nullableSQLiteTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(sqliteTimeLayout)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
