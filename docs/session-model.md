# Session Model

## Stored Artifacts

- `.orch/sessions/<session-id>.jsonl`
- `.orch/sessions/<session-id>.meta.json`
- shared `.orch/chatHistory.md`

## Metadata

Session metadata now includes worker lineage fields such as:

- `parent_session_id`
- `parent_run_id`
- `parent_task_id`
- `worker_role`
- `task_title`
- `task_contract`
- `task_status`
- `task_summary`
- `task_changed_paths`
- `task_checks_run`
- `task_evidence_pointers`
- `task_followups`
- `task_error_kind`

## Tasks

Delegated worker sessions are now treated as first-class tasks derived from session metadata.

Each child worker session keeps:

- task identity via `parent_task_id`
- parent linkage via `parent_session_id` and `parent_run_id`
- task lifecycle state: `queued`, `running`, `completed`, `failed`, `cancelled`
- structured task outcome fields for summaries, changed paths, checks, evidence, and followups

The runtime does not use a separate task database. Task views are derived from the session files already stored under `.orch/sessions/`.

## Context Snapshots

Each run now persists a `context_snapshot` session record before each provider call.

The snapshot captures the effective runtime context used for that iteration, including:

- provider and model
- workspace path and current cwd
- compact-summary and inherited-context presence
- post-compact and inherited record counts
- selected skills
- resolved reference count
- user-memory presence
- chat-history excerpt size
- plan-cache presence

## Continuity

Default reinjection uses:

- compact summary
- raw records after the compact point

`chatHistory.md` is no longer injected as a full file. The runtime loads bounded recent excerpts instead.
