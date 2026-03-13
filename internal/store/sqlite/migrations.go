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
	ALTER TABLE runs ADD COLUMN mode TEXT NOT NULL DEFAULT 'react'
	`,
	`
	ALTER TABLE runs ADD COLUMN current_cwd TEXT NOT NULL DEFAULT ''
	`,
	`
	ALTER TABLE runs ADD COLUMN ralph_iteration INTEGER NOT NULL DEFAULT 0
	`,
}
