package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func (s *Store) SaveTaskOutcome(ctx context.Context, outcome domain.TaskOutcome) (domain.TaskOutcome, error) {
	now := time.Now()
	if outcome.CreatedAt.IsZero() {
		outcome.CreatedAt = now
	}
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO task_outcomes (
			session_id, run_id, task_id, title, status, summary, changed_paths,
			checks_run, evidence_pointers, followups, error_kind, fingerprint, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		outcome.SessionID,
		outcome.RunID,
		outcome.TaskID,
		outcome.Title,
		outcome.Status,
		outcome.Summary,
		encodeStringSlice(outcome.ChangedPaths),
		encodeStringSlice(outcome.ChecksRun),
		encodeStringSlice(outcome.EvidencePointers),
		encodeStringSlice(outcome.Followups),
		outcome.ErrorKind,
		outcome.Fingerprint,
		outcome.CreatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.TaskOutcome{}, fmt.Errorf("save task outcome: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.TaskOutcome{}, fmt.Errorf("task outcome last insert id: %w", err)
	}
	outcome.ID = id
	return outcome, nil
}

func (s *Store) ListTaskOutcomesByFingerprint(
	ctx context.Context,
	fingerprint string,
	limit int,
) ([]domain.TaskOutcome, error) {
	if strings.TrimSpace(fingerprint) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, session_id, run_id, task_id, title, status, summary, changed_paths,
		       checks_run, evidence_pointers, followups, error_kind, fingerprint, created_at
		FROM task_outcomes
		WHERE fingerprint = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, fingerprint, limit)
	if err != nil {
		return nil, fmt.Errorf("list task outcomes by fingerprint: %w", err)
	}
	defer rows.Close()

	outcomes := make([]domain.TaskOutcome, 0, limit)
	for rows.Next() {
		var item domain.TaskOutcome
		var changedPaths string
		var checksRun string
		var evidencePointers string
		var followups string
		var createdAt string
		if err := rows.Scan(
			&item.ID,
			&item.SessionID,
			&item.RunID,
			&item.TaskID,
			&item.Title,
			&item.Status,
			&item.Summary,
			&changedPaths,
			&checksRun,
			&evidencePointers,
			&followups,
			&item.ErrorKind,
			&item.Fingerprint,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan task outcome: %w", err)
		}
		item.ChangedPaths, _ = decodeStringSlice(changedPaths)
		item.ChecksRun, _ = decodeStringSlice(checksRun)
		item.EvidencePointers, _ = decodeStringSlice(evidencePointers)
		item.Followups, _ = decodeStringSlice(followups)
		item.CreatedAt = parseSQLiteTime(createdAt)
		outcomes = append(outcomes, item)
	}
	return outcomes, rows.Err()
}

func (s *Store) SaveMemoryEntry(ctx context.Context, entry domain.MemoryEntry) (domain.MemoryEntry, error) {
	now := time.Now()
	if entry.Status == "" {
		entry.Status = domain.MemoryStatusActive
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	if strings.TrimSpace(entry.Fingerprint) != "" {
		existing, err := s.GetMemoryByFingerprint(ctx, entry.WorkspacePath, entry.Kind, entry.Fingerprint)
		if err == nil && existing.ID > 0 {
			entry.ID = existing.ID
		} else if err != nil && err != sql.ErrNoRows {
			return domain.MemoryEntry{}, err
		}
	}

	if entry.ID > 0 {
		_, err := s.db.ExecContext(ctx, `
			UPDATE memory_entries
			SET title = ?, content = ?, evidence_pointers = ?, status = ?, updated_at = ?,
			    source_session_id = ?, source_run_id = ?, fingerprint = ?, workspace_path = ?, kind = ?
			WHERE id = ?
		`,
			entry.Title,
			entry.Content,
			encodeStringSlice(entry.EvidencePointers),
			string(entry.Status),
			entry.UpdatedAt.Format(sqliteTimeLayout),
			entry.SourceSessionID,
			entry.SourceRunID,
			entry.Fingerprint,
			entry.WorkspacePath,
			string(entry.Kind),
			entry.ID,
		)
		if err != nil {
			return domain.MemoryEntry{}, fmt.Errorf("update memory entry: %w", err)
		}
	} else {
		result, err := s.db.ExecContext(ctx, `
			INSERT INTO memory_entries (
				kind, workspace_path, source_session_id, source_run_id, title, content,
				evidence_pointers, status, fingerprint, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			string(entry.Kind),
			entry.WorkspacePath,
			entry.SourceSessionID,
			entry.SourceRunID,
			entry.Title,
			entry.Content,
			encodeStringSlice(entry.EvidencePointers),
			string(entry.Status),
			entry.Fingerprint,
			entry.CreatedAt.Format(sqliteTimeLayout),
			entry.UpdatedAt.Format(sqliteTimeLayout),
		)
		if err != nil {
			return domain.MemoryEntry{}, fmt.Errorf("insert memory entry: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return domain.MemoryEntry{}, fmt.Errorf("memory entry last insert id: %w", err)
		}
		entry.ID = id
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM memory_entries_fts WHERE memory_id = ?`, entry.ID); err != nil {
		return domain.MemoryEntry{}, fmt.Errorf("delete memory fts: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_entries_fts (memory_id, kind, workspace_path, title, content)
		VALUES (?, ?, ?, ?, ?)
	`, entry.ID, string(entry.Kind), entry.WorkspacePath, entry.Title, entry.Content); err != nil {
		return domain.MemoryEntry{}, fmt.Errorf("insert memory fts: %w", err)
	}
	return entry, nil
}

func (s *Store) GetMemoryByFingerprint(
	ctx context.Context,
	workspacePath string,
	kind domain.MemoryKind,
	fingerprint string,
) (domain.MemoryEntry, error) {
	return s.scanMemoryEntryRow(s.db.QueryRowContext(ctx, `
		SELECT id, kind, workspace_path, source_session_id, source_run_id, title, content,
		       evidence_pointers, status, fingerprint, created_at, updated_at
		FROM memory_entries
		WHERE workspace_path = ? AND kind = ? AND fingerprint = ?
		LIMIT 1
	`, workspacePath, string(kind), fingerprint))
}

func (s *Store) SearchMemoryEntries(
	ctx context.Context,
	workspacePath string,
	query string,
	limit int,
) ([]domain.MemorySearchResult, error) {
	if limit <= 0 {
		limit = 5
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.kind, m.workspace_path, m.source_session_id, m.source_run_id, m.title,
		       snippet(memory_entries_fts, 4, '[', ']', '...', 18),
		       m.evidence_pointers, m.status, m.fingerprint, m.created_at, m.updated_at
		FROM memory_entries_fts
		JOIN memory_entries m
		  ON m.id = CAST(memory_entries_fts.memory_id AS INTEGER)
		WHERE memory_entries_fts MATCH ?
		  AND (? = '' OR m.workspace_path = ?)
		  AND m.status = ?
		ORDER BY bm25(memory_entries_fts), m.updated_at DESC
		LIMIT ?
	`, ftsPhraseQuery(query), strings.TrimSpace(workspacePath), strings.TrimSpace(workspacePath), string(domain.MemoryStatusActive), limit)
	if err != nil {
		return nil, fmt.Errorf("search memory entries: %w", err)
	}
	defer rows.Close()

	results := make([]domain.MemorySearchResult, 0, limit)
	for rows.Next() {
		var item domain.MemorySearchResult
		var kind string
		var evidencePointers string
		var status string
		var createdAt string
		var updatedAt string
		if err := rows.Scan(
			&item.Entry.ID,
			&kind,
			&item.Entry.WorkspacePath,
			&item.Entry.SourceSessionID,
			&item.Entry.SourceRunID,
			&item.Entry.Title,
			&item.Entry.Content,
			&evidencePointers,
			&status,
			&item.Entry.Fingerprint,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan memory search result: %w", err)
		}
		item.Entry.Kind = domain.MemoryKind(kind)
		item.Entry.Status = domain.MemoryStatus(status)
		item.Entry.EvidencePointers, _ = decodeStringSlice(evidencePointers)
		item.Entry.CreatedAt = parseSQLiteTime(createdAt)
		item.Entry.UpdatedAt = parseSQLiteTime(updatedAt)
		item.SelectionReason = "matched persistent memory"
		results = append(results, item)
	}
	return results, rows.Err()
}

func (s *Store) LinkMemory(ctx context.Context, memoryID int64, sessionID string, runID string, linkKind string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memory_links (memory_id, session_id, run_id, link_kind, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, memoryID, strings.TrimSpace(sessionID), strings.TrimSpace(runID), strings.TrimSpace(linkKind), time.Now().Format(sqliteTimeLayout))
	if err != nil {
		return fmt.Errorf("link memory: %w", err)
	}
	return nil
}

func (s *Store) SaveSkill(ctx context.Context, skill domain.SkillRecord) (domain.SkillRecord, error) {
	now := time.Now()
	if skill.Status == "" {
		skill.Status = domain.SkillStatusDraft
	}
	if skill.CreatedAt.IsZero() {
		skill.CreatedAt = now
	}
	skill.UpdatedAt = now

	var existingSummary string
	var existingContent string
	var existingVersion int
	err := s.db.QueryRowContext(ctx, `
		SELECT summary, content, version
		FROM skills
		WHERE skill_id = ?
	`, skill.SkillID).Scan(&existingSummary, &existingContent, &existingVersion)
	switch {
	case err == sql.ErrNoRows:
		if skill.Version <= 0 {
			skill.Version = 1
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO skills (
				skill_id, workspace_path, name, summary, content, status, source_memory_id,
				fingerprint, version, replay_count, created_at, updated_at
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			skill.SkillID,
			skill.WorkspacePath,
			skill.Name,
			skill.Summary,
			skill.Content,
			string(skill.Status),
			skill.SourceMemoryID,
			skill.Fingerprint,
			skill.Version,
			skill.ReplayCount,
			skill.CreatedAt.Format(sqliteTimeLayout),
			skill.UpdatedAt.Format(sqliteTimeLayout),
		)
		if err != nil {
			return domain.SkillRecord{}, fmt.Errorf("insert skill: %w", err)
		}
	case err != nil:
		return domain.SkillRecord{}, fmt.Errorf("load existing skill: %w", err)
	default:
		if skill.Version <= 0 {
			skill.Version = existingVersion
			if strings.TrimSpace(existingSummary) != strings.TrimSpace(skill.Summary) ||
				strings.TrimSpace(existingContent) != strings.TrimSpace(skill.Content) {
				skill.Version++
			}
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE skills
			SET workspace_path = ?, name = ?, summary = ?, content = ?, status = ?, source_memory_id = ?,
			    fingerprint = ?, version = ?, replay_count = ?, updated_at = ?
			WHERE skill_id = ?
		`,
			skill.WorkspacePath,
			skill.Name,
			skill.Summary,
			skill.Content,
			string(skill.Status),
			skill.SourceMemoryID,
			skill.Fingerprint,
			skill.Version,
			skill.ReplayCount,
			skill.UpdatedAt.Format(sqliteTimeLayout),
			skill.SkillID,
		)
		if err != nil {
			return domain.SkillRecord{}, fmt.Errorf("update skill: %w", err)
		}
	}

	if skill.Version <= 0 {
		skill.Version = 1
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO skill_versions (
			skill_id, version, summary, content, source_memory_id, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		skill.SkillID,
		skill.Version,
		skill.Summary,
		skill.Content,
		skill.SourceMemoryID,
		now.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.SkillRecord{}, fmt.Errorf("insert skill version: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `DELETE FROM skills_fts WHERE skill_id = ?`, skill.SkillID); err != nil {
		return domain.SkillRecord{}, fmt.Errorf("delete skill fts: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO skills_fts (skill_id, workspace_path, status, name, summary, content)
		VALUES (?, ?, ?, ?, ?, ?)
	`, skill.SkillID, skill.WorkspacePath, string(skill.Status), skill.Name, skill.Summary, skill.Content); err != nil {
		return domain.SkillRecord{}, fmt.Errorf("insert skill fts: %w", err)
	}
	return skill, nil
}

func (s *Store) GetSkill(ctx context.Context, skillID string) (domain.SkillRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT skill_id, workspace_path, name, summary, content, status, source_memory_id,
		       fingerprint, version, replay_count, created_at, updated_at
		FROM skills
		WHERE skill_id = ?
	`, strings.TrimSpace(skillID))
	return scanSkillRecord(row)
}

func (s *Store) GetSkillByFingerprint(ctx context.Context, workspacePath string, fingerprint string) (domain.SkillRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT skill_id, workspace_path, name, summary, content, status, source_memory_id,
		       fingerprint, version, replay_count, created_at, updated_at
		FROM skills
		WHERE workspace_path = ? AND fingerprint = ?
		LIMIT 1
	`, workspacePath, fingerprint)
	return scanSkillRecord(row)
}

func (s *Store) ListSkills(
	ctx context.Context,
	workspacePath string,
	status string,
	limit int,
) ([]domain.SkillRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT skill_id, workspace_path, name, summary, content, status, source_memory_id,
		       fingerprint, version, replay_count, created_at, updated_at
		FROM skills
		WHERE (? = '' OR workspace_path = ?)
		  AND (? = '' OR status = ?)
		ORDER BY updated_at DESC, skill_id ASC
		LIMIT ?
	`, strings.TrimSpace(workspacePath), strings.TrimSpace(workspacePath), strings.TrimSpace(status), strings.TrimSpace(status), limit)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer rows.Close()

	skills := make([]domain.SkillRecord, 0, limit)
	for rows.Next() {
		item, err := scanSkillRecord(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, item)
	}
	return skills, rows.Err()
}

func (s *Store) SearchSkills(
	ctx context.Context,
	workspacePath string,
	query string,
	status string,
	limit int,
) ([]domain.SkillRecord, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.skill_id, s.workspace_path, s.name, s.summary, s.content, s.status, s.source_memory_id,
		       s.fingerprint, s.version, s.replay_count, s.created_at, s.updated_at
		FROM skills_fts
		JOIN skills s ON s.skill_id = skills_fts.skill_id
		WHERE skills_fts MATCH ?
		  AND (? = '' OR s.workspace_path = ?)
		  AND (? = '' OR s.status = ?)
		ORDER BY bm25(skills_fts), s.updated_at DESC
		LIMIT ?
	`, ftsPhraseQuery(query), strings.TrimSpace(workspacePath), strings.TrimSpace(workspacePath), strings.TrimSpace(status), strings.TrimSpace(status), limit)
	if err != nil {
		return nil, fmt.Errorf("search skills: %w", err)
	}
	defer rows.Close()

	skills := make([]domain.SkillRecord, 0, limit)
	for rows.Next() {
		item, err := scanSkillRecord(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, item)
	}
	return skills, rows.Err()
}

func (s *Store) SavePromotionJob(ctx context.Context, job domain.PromotionJob) (domain.PromotionJob, error) {
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	if job.ID > 0 {
		_, err := s.db.ExecContext(ctx, `
			UPDATE promotion_jobs
			SET job_type = ?, status = ?, memory_id = ?, skill_id = ?, fingerprint = ?,
			    replay_count = ?, required_evidence_count = ?, notes = ?, updated_at = ?
			WHERE id = ?
		`,
			job.JobType,
			string(job.Status),
			job.MemoryID,
			job.SkillID,
			job.Fingerprint,
			job.ReplayCount,
			job.RequiredEvidenceCount,
			job.Notes,
			job.UpdatedAt.Format(sqliteTimeLayout),
			job.ID,
		)
		if err != nil {
			return domain.PromotionJob{}, fmt.Errorf("update promotion job: %w", err)
		}
		return job, nil
	}

	result, err := s.db.ExecContext(ctx, `
		INSERT INTO promotion_jobs (
			job_type, status, memory_id, skill_id, fingerprint, replay_count,
			required_evidence_count, notes, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.JobType,
		string(job.Status),
		job.MemoryID,
		job.SkillID,
		job.Fingerprint,
		job.ReplayCount,
		job.RequiredEvidenceCount,
		job.Notes,
		job.CreatedAt.Format(sqliteTimeLayout),
		job.UpdatedAt.Format(sqliteTimeLayout),
	)
	if err != nil {
		return domain.PromotionJob{}, fmt.Errorf("insert promotion job: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.PromotionJob{}, fmt.Errorf("promotion job last insert id: %w", err)
	}
	job.ID = id
	return job, nil
}

func scanSkillRecord(scanner interface{ Scan(dest ...any) error }) (domain.SkillRecord, error) {
	var item domain.SkillRecord
	var status string
	var createdAt string
	var updatedAt string
	err := scanner.Scan(
		&item.SkillID,
		&item.WorkspacePath,
		&item.Name,
		&item.Summary,
		&item.Content,
		&status,
		&item.SourceMemoryID,
		&item.Fingerprint,
		&item.Version,
		&item.ReplayCount,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.SkillRecord{}, err
	}
	item.Status = domain.SkillStatus(status)
	item.CreatedAt = parseSQLiteTime(createdAt)
	item.UpdatedAt = parseSQLiteTime(updatedAt)
	return item, nil
}

func (s *Store) scanMemoryEntryRow(scanner interface{ Scan(dest ...any) error }) (domain.MemoryEntry, error) {
	var item domain.MemoryEntry
	var kind string
	var evidencePointers string
	var status string
	var createdAt string
	var updatedAt string
	err := scanner.Scan(
		&item.ID,
		&kind,
		&item.WorkspacePath,
		&item.SourceSessionID,
		&item.SourceRunID,
		&item.Title,
		&item.Content,
		&evidencePointers,
		&status,
		&item.Fingerprint,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return domain.MemoryEntry{}, err
	}
	item.Kind = domain.MemoryKind(kind)
	item.Status = domain.MemoryStatus(status)
	item.EvidencePointers, _ = decodeStringSlice(evidencePointers)
	item.CreatedAt = parseSQLiteTime(createdAt)
	item.UpdatedAt = parseSQLiteTime(updatedAt)
	return item, nil
}
