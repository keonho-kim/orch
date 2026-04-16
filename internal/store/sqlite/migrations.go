package sqlite

var migrationStatements = []string{
	`
	CREATE TABLE IF NOT EXISTS schema_version (
		id INTEGER PRIMARY KEY,
		version INTEGER NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		prompt TEXT NOT NULL,
		created_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS runs (
		run_id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL DEFAULT '',
		mode TEXT NOT NULL DEFAULT 'react',
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		prompt TEXT NOT NULL,
		current_task TEXT NOT NULL,
		status TEXT NOT NULL,
		workspace_path TEXT NOT NULL,
		current_cwd TEXT NOT NULL DEFAULT '',
		ralph_iteration INTEGER NOT NULL DEFAULT 0,
		final_output TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS run_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS plan_cache (
		id INTEGER PRIMARY KEY,
		source_run_id TEXT NOT NULL,
		content TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS sessions (
		session_id TEXT PRIMARY KEY,
		workspace_path TEXT NOT NULL,
		parent_session_id TEXT NOT NULL DEFAULT '',
		parent_run_id TEXT NOT NULL DEFAULT '',
		parent_task_id TEXT NOT NULL DEFAULT '',
		worker_role TEXT NOT NULL DEFAULT '',
		task_title TEXT NOT NULL DEFAULT '',
		task_contract TEXT NOT NULL DEFAULT '',
		task_status TEXT NOT NULL DEFAULT '',
		task_summary TEXT NOT NULL DEFAULT '',
		task_changed_paths TEXT NOT NULL DEFAULT '[]',
		task_checks_run TEXT NOT NULL DEFAULT '[]',
		task_evidence_pointers TEXT NOT NULL DEFAULT '[]',
		task_followups TEXT NOT NULL DEFAULT '[]',
		task_error_kind TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL,
		model TEXT NOT NULL,
		title TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		started_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		last_sequence INTEGER NOT NULL DEFAULT 0,
		last_compacted_seq INTEGER NOT NULL DEFAULT 0,
		tokens_since_compact INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		finalize_pending INTEGER NOT NULL DEFAULT 0,
		finalized_at TEXT,
		last_run_id TEXT NOT NULL DEFAULT ''
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		seq INTEGER NOT NULL,
		run_id TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL,
		content TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		tool_name TEXT NOT NULL DEFAULT '',
		tool_call_id TEXT NOT NULL DEFAULT '',
		through_seq INTEGER NOT NULL DEFAULT 0,
		context_snapshot TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		usage_prompt_tokens INTEGER NOT NULL DEFAULT 0,
		usage_completion_tokens INTEGER NOT NULL DEFAULT 0,
		usage_total_tokens INTEGER NOT NULL DEFAULT 0,
		UNIQUE(session_id, seq)
	)
	`,
	`
	CREATE INDEX IF NOT EXISTS messages_session_created_idx ON messages(session_id, created_at)
	`,
	`
	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		session_id UNINDEXED,
		run_id UNINDEXED,
		seq UNINDEXED,
		type UNINDEXED,
		title,
		content
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS task_outcomes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		task_id TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		changed_paths TEXT NOT NULL DEFAULT '[]',
		checks_run TEXT NOT NULL DEFAULT '[]',
		evidence_pointers TEXT NOT NULL DEFAULT '[]',
		followups TEXT NOT NULL DEFAULT '[]',
		error_kind TEXT NOT NULL DEFAULT '',
		fingerprint TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)
	`,
	`
	CREATE INDEX IF NOT EXISTS task_outcomes_fingerprint_idx ON task_outcomes(fingerprint, created_at)
	`,
	`
	CREATE TABLE IF NOT EXISTS memory_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		kind TEXT NOT NULL,
		workspace_path TEXT NOT NULL,
		source_session_id TEXT NOT NULL DEFAULT '',
		source_run_id TEXT NOT NULL DEFAULT '',
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		evidence_pointers TEXT NOT NULL DEFAULT '[]',
		status TEXT NOT NULL DEFAULT 'active',
		fingerprint TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	CREATE INDEX IF NOT EXISTS memory_entries_kind_idx ON memory_entries(kind, updated_at)
	`,
	`
	CREATE VIRTUAL TABLE IF NOT EXISTS memory_entries_fts USING fts5(
		memory_id UNINDEXED,
		kind UNINDEXED,
		workspace_path UNINDEXED,
		title,
		content
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS memory_links (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		memory_id INTEGER NOT NULL,
		session_id TEXT NOT NULL DEFAULT '',
		run_id TEXT NOT NULL DEFAULT '',
		link_kind TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS skills (
		skill_id TEXT PRIMARY KEY,
		workspace_path TEXT NOT NULL,
		name TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'draft',
		source_memory_id INTEGER NOT NULL DEFAULT 0,
		fingerprint TEXT NOT NULL DEFAULT '',
		version INTEGER NOT NULL DEFAULT 1,
		replay_count INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	CREATE VIRTUAL TABLE IF NOT EXISTS skills_fts USING fts5(
		skill_id UNINDEXED,
		workspace_path UNINDEXED,
		status UNINDEXED,
		name,
		summary,
		content
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS skill_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id TEXT NOT NULL,
		version INTEGER NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		source_memory_id INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	)
	`,
	`
	CREATE TABLE IF NOT EXISTS promotion_jobs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		job_type TEXT NOT NULL,
		status TEXT NOT NULL,
		memory_id INTEGER NOT NULL DEFAULT 0,
		skill_id TEXT NOT NULL DEFAULT '',
		fingerprint TEXT NOT NULL DEFAULT '',
		replay_count INTEGER NOT NULL DEFAULT 0,
		required_evidence_count INTEGER NOT NULL DEFAULT 0,
		notes TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)
	`,
	`
	ALTER TABLE runs ADD COLUMN mode TEXT NOT NULL DEFAULT 'react'
	`,
	`
	ALTER TABLE runs ADD COLUMN current_cwd TEXT NOT NULL DEFAULT ''
	`,
	`
	ALTER TABLE runs ADD COLUMN ralph_iteration INTEGER NOT NULL DEFAULT 0
	`,
	`
	ALTER TABLE runs ADD COLUMN session_id TEXT NOT NULL DEFAULT ''
	`,
}
