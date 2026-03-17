# Session Model

## Artifacts

| Artifact | Purpose | Raw Content |
| --- | --- | --- |
| `.orch/sessions/<session-id>.jsonl` | append-only transcript store | yes |
| compact record inside session JSONL | reinjection compression | no |
| `.orch/chatHistory.md` | rolling whole-conversation digest for sLLMs | no |
| `ot-pointer://current?...` | reference back to current-session JSONL lines | no |

## JSONL

JSONL is the canonical transcript source of truth.

It stores:

- user messages
- assistant messages
- tool results
- compact records
- title updates

It does not store streamed reasoning.

## Compact

Compact is a reinjection optimization, not a raw transcript replacement.

Current compact pipeline:

1. collect transcript input up to the compact cut line
2. extract major topics with ordered line anchors
3. summarize topic groups in parallel
4. merge summaries in chronological order
5. persist a compact record and reset `TokensSinceCompact`

Compact output may contain paragraph-level `ot-pointer` references.

## chatHistory

`chatHistory.md` is separate from compact.

It exists to help weaker sLLMs maintain continuity by injecting a rolling digest on every provider call.

Current behavior:

- user summary appended in the background after prompt submission
- assistant summary appended in the background after terminal completion
- summary-only, no raw transcript duplication
- may include paragraph-level `ot-pointer` references

## Pointers

Pointer format:

```text
ot-pointer://current?lines=12,13,14
```

Properties:

- current-session only
- line-oriented
- deterministic for the same compact input
- resolvable through `ot pointer --value <ot-pointer>`

## Session Switching

### `/clear`

`/clear` now means:

1. open a fresh session
2. clear the visible conversation in the TUI
3. finalize the previous session in the background

Constraint:

- it does not open a new session while a run is active

### Restore

`orch history`, `orch history --workspace <path> --latest`, and explicit session restore still load:

- the latest compact summary
- raw records after the compact point

Older pre-compact records remain on disk but are not reinjected.

## Child Sessions

`ot subagent` creates child sessions with:

- `parent_session_id`
- `parent_run_id`

Child sessions may inherit parent compact context plus later restorable turns, but not copied raw transcript records.
