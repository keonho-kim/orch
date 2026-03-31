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

## Continuity

Default reinjection uses:

- compact summary
- raw records after the compact point

`chatHistory.md` is no longer injected as a full file. The runtime loads bounded recent excerpts instead.
