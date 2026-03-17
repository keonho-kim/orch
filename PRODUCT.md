# Product Direction

## Vision

`orch` is a Go-based command center for one local repository-focused AI execution loop with two operator-selected modes: execution and planning.

## Supported Behavior

- The application starts as a Bubble Tea console chat UI with one scrollable conversation timeline, one sticky command row, one sticky status line, and modal overlays.
- The timeline starts with the ASCII `ORCH` wordmark and version metadata as scrollable content instead of a fixed dashboard header.
- Each run appears inline in the timeline with a centered separator, a subtle run meta line, a `USER` prompt block, and an `ORCH` response block.
- The TUI renders lightweight inline terminal styling for `**bold**`, `*italic*`, and `__underline__` spans in assistant output.
- Streamed response text wraps to the active pane width instead of truncating at the viewport edge.
- Streamed reasoning for the active run appears in a faint bordered `THINK` box and can be collapsed to a `THINKING ...` placeholder.
- `orch exec [--mode react|plan] "<request>"` runs the same workflow without entering the TUI.
- Supported providers are Ollama and vLLM only.
- The first usable launch requires a configured default provider and model in [orch.settings.json](/Users/khkim/dev/orch/orch.settings.json).
- The shared workspace lives in `./test-workspace`.
- The workspace syncs `PRODUCT.md`, `AGENTS.md`, `bootstrap/SKILLS.md`, `bootstrap/skills/**`, and `tools/**` from repo assets and preserves `bootstrap/USER.md`.
- `auto_translate` detects one language bucket (`kor`, `en`, or `ch`), translates the input into the other two languages in parallel, and feeds the combined three-language request into the main agent loop without using bootstrap context.
- The first detected language can be stored once as a hidden machine-managed preference block inside `bootstrap/USER.md`.
- The visible UI continues to show only the original user input. Translated variants stay internal and are never rendered as part of the request view.
- The TUI preserves message-history recall on `Up` and `Down`, uses mouse wheel plus `PgUp`/`PgDn` and `Home`/`End` for in-app timeline scrolling, labels `Ctrl+T` as `Show Think` while toggling active-run reasoning visibility, uses `Ctrl+S` for settings, shows a slash-command dropdown above the composer when the user types `/`, allows `Up`/`Down` to move through slash commands, uses `Shift+Tab` to toggle the sticky composer mode, uses `/clear` to open a new session and clear the visible conversation, uses `/compact` to force session compaction, and uses `/exit` for controlled shutdown.
- Settings allow toggling `self_driving_mode`, toggling `auto_translate`, and editing `react_ralph_iter`, `plan_ralph_iter`, plus `compact_threshold_k`.
- Each app session is persisted under `./.orch/sessions` in the current workspace/repo root.
- Saved session transcript records include user messages, assistant messages, tool records, compact records, and title updates, but never reasoning text.
- `.orch/chatHistory.md` stores a rolling summary-only conversation digest used for sLLM prompt reinjection and does not replace JSONL or compact records.
- `ot subagent --prompt "<task>"` is available only in ReAct mode. It creates a child session, runs one synchronous child ReAct request in a hidden backend process, and returns structured child result JSON to the parent tool call.
- Child session metadata stores `parent_session_id` and `parent_run_id`.
- `orch history` opens Session History for the current workspace, `orch history --workspace <path> --latest` restores the most recent saved session for that workspace, and `orch history <session-id> --workspace <path>` restores a specific session.
- First-launch setup is provider-aware:
  - provider choice comes first
  - `vLLM` continues with the existing manual settings flow
  - `Ollama` offers default or custom URL selection, performs a connection check, loads local models from the Ollama API, and saves the selected model

## Mode Model

- `ReAct Deep Agent`
  - default mode
  - runs a Ralph reflect-and-act loop
  - uses `react_ralph_iter`
  - may execute the full approved command surface
- `Plan`
  - planning-only mode
  - runs a Ralph loop with `plan_ralph_iter`
  - only allows `cd`, `ot read`, `ot list`, and `ot search`
  - does not execute builds, writes, patches, or custom scripts

## Ralph And Plan Cache

- On every Ralph iteration, orch re-reads and re-injects:
  - `PRODUCT.md`
  - `AGENTS.md`
  - `bootstrap/USER.md`
  - `bootstrap/SKILLS.md`
- On every provider call, orch also injects a concise text summary of currently available tools for the active mode.
- User prompt references `@filename` and `#dir-name` are resolved against the current workspace and injected as markdown-style links.
- Plan mode also re-injects the current draft plan each iteration.
- A successful Plan run becomes the active cached plan.
- Later ReAct runs automatically receive the active cached plan until another successful Plan run replaces it.
- auto-translate runs once per submitted request, before the first Ralph iteration, and falls back to the original prompt if detect or translation fails.
- Restored sessions inject the latest compact summary plus later raw conversation records back into live model context. Older pre-compact history remains visible but is not reinjected.
- `.orch/chatHistory.md` is injected on every provider call as a rolling whole-conversation digest for weaker sLLMs.
- New child sessions inherit only the parent compact summary plus later raw conversation records for their initial run. That inherited parent context is not copied into the child transcript log.

## Tool Policy

- Orch exposes one model-facing tool: `exec`.
- Every provider call sends both the structured tool schema and a concise text tool summary.
- ReAct mode prefers `ot read --path <path>` for file content and quick directory inspection, prefers `ot list [--path <path>]` for long directory listings, prefers `ot search [--path <path>] [--name <glob>] [--content <pattern>]` for curated name/content search, may use `ot pointer --value <ot-pointer>` to inspect current-session transcript lines from compact or `chatHistory.md`, may use `ot subagent --prompt "<task>"` for a synchronous child ReAct run in a child session, uses direct `rg` or `find` only when task execution needs behavior outside the curated OT commands, may use `bash tools/<script>.sh ...` for custom scripts, and may use direct toolchain commands when task execution requires them.
- Plan mode allows only:
  - `cd <path>`
  - `ot read --path <path>`
  - `ot list [--path <path>]`
  - `ot search [--path <path>] [--name <glob>] [--content <pattern>]`
- `ot read`, `ot list`, and `ot search` may inspect paths inside or outside the workspace.
- Inside the workspace, those OT inspection commands include hidden files and directories.
- Outside the workspace, those OT inspection commands reject explicitly hidden target paths and must not expose or traverse hidden descendants.
- Approval policy:
  - `ot read`, `ot list`, and `ot search` are auto-allowed for workspace paths
  - `ot read`, `ot list`, and `ot search` require user approval for paths outside the workspace
  - `ot subagent` requires user approval unless self-driving mode is enabled
  - `ot write` requires user approval
  - every other command requires user approval unless self-driving mode is enabled
  - self-driving mode auto-approves all commands except `rm`, `mv`, and external read-only OT access, which still require approval

## Configuration Contracts

- The root [AGENTS.md](/Users/khkim/dev/orch/AGENTS.md) remains the repository development guide.
- Runtime bootstrap authoring lives under [runtime-asset/bootstrap/AGENTS.md](/Users/khkim/dev/orch/runtime-asset/bootstrap/AGENTS.md), [runtime-asset/bootstrap/USER.md](/Users/khkim/dev/orch/runtime-asset/bootstrap/USER.md), [runtime-asset/bootstrap/SKILLS.md](/Users/khkim/dev/orch/runtime-asset/bootstrap/SKILLS.md), and `runtime-asset/bootstrap/skills/<skill-name>/...`.
- Users may explicitly request a skill by mentioning `$<skill-name>`.
- Explicitly requested skills are injected every provider call alongside the canonical skill index.
- Users may explicitly reference workspace files and directories with `@filename` and `#dir-name`.
- Repository-local launch settings live in [orch.settings.json](/Users/khkim/dev/orch/orch.settings.json).
- SQLite stores the default provider, message history recall entries, runs, run events, and the active plan cache in the user config directory resolved by the standard library.
- Workspace-local `.orch/sessions` stores session transcript JSONL files and sidecar metadata used for restore, compact, and history listing.
- Workspace-local `.orch/chatHistory.md` stores the rolling summary-only digest used for prompt reinjection.
- Compact output and `chatHistory.md` may include `ot-pointer` references back to exact JSONL lines in the current session only.

## Runtime Model

- The TUI routes user actions into the agent service.
- The agent service provisions the workspace, runs the Ralph loop, refreshes bootstrap context, enforces mode restrictions, persists run state, and updates the active plan cache.
- The agent service owns coordination of the active session, session restore, compact/title maintenance triggers, and shutdown finalization handoff.
- The agent service also owns child-session bootstrapping, inherited parent context loading, and the hidden backend child-run path used by `ot subagent`.
- The session layer owns transcript persistence, `chatHistory.md` rolling-summary persistence, pointer-aware compact generation, live-context reconstruction, parent-child linkage metadata, title generation, and finalize state transitions.
- The tooling layer owns `exec`, `ot`, `ot subagent`, approval classification, mode restrictions, command allowlisting, and workspace path confinement.
- The provider layer owns provider HTTP requests, stream parsing, and usage accounting. Ollama chat uses the native API stream; vLLM uses the OpenAI-compatible stream with streamed usage enabled.

## Verification Handoff

Validation was not executed during this update.

- `gofmt -w .`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run ./...`
