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
- The TUI preserves prompt history on `Up` and `Down`, uses mouse wheel plus `PgUp`/`PgDn` and `Home`/`End` for in-app timeline scrolling, labels `Ctrl+T` as `Show Think` while toggling active-run reasoning visibility, uses `Ctrl+S` for settings, uses `Shift+Tab` to toggle the sticky composer mode, and uses `/exit` for controlled shutdown.
- Settings allow toggling `self_driving_mode`, toggling `auto_translate`, and editing `react_ralph_iter` plus `plan_ralph_iter`.
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
  - only allows `cd` and `ot read`
  - does not execute builds, writes, patches, or custom scripts

## Ralph And Plan Cache

- On every Ralph iteration, orch re-reads and re-injects:
  - `PRODUCT.md`
  - `AGENTS.md`
  - `bootstrap/USER.md`
  - `bootstrap/SKILLS.md`
- Plan mode also re-injects the current draft plan each iteration.
- A successful Plan run becomes the active cached plan.
- Later ReAct runs automatically receive the active cached plan until another successful Plan run replaces it.
- auto-translate runs once per submitted request, before the first Ralph iteration, and falls back to the original prompt if detect or translation fails.

## Tool Policy

- Orch exposes one model-facing tool: `exec`.
- ReAct mode prefers `ot read --path <path>` for inspection, uses `rg` or `find` for search and discovery, may use `bash tools/<script>.sh ...` for custom scripts, and may use direct toolchain commands when task execution requires them.
- Plan mode allows only:
  - `cd <path>`
  - `ot read --path <path>`
- Approval policy:
  - `ot read` is always allowed
  - `ot write` requires user approval
  - every other command requires user approval unless self-driving mode is enabled
  - self-driving mode auto-approves all commands except `rm` and `mv`, which still require approval

## Configuration Contracts

- The root [AGENTS.md](/Users/khkim/dev/orch/AGENTS.md) remains the repository development guide.
- Runtime bootstrap authoring lives under [runtime-asset/bootstrap/AGENTS.md](/Users/khkim/dev/orch/runtime-asset/bootstrap/AGENTS.md), [runtime-asset/bootstrap/USER.md](/Users/khkim/dev/orch/runtime-asset/bootstrap/USER.md), [runtime-asset/bootstrap/SKILLS.md](/Users/khkim/dev/orch/runtime-asset/bootstrap/SKILLS.md), and `runtime-asset/bootstrap/skills/<skill-name>/...`.
- Repository-local launch settings live in [orch.settings.json](/Users/khkim/dev/orch/orch.settings.json).
- SQLite stores the default provider, prompt history, runs, run events, and the active plan cache in the user config directory resolved by the standard library.

## Runtime Model

- The TUI routes user actions into the agent service.
- The agent service provisions the workspace, runs the Ralph loop, refreshes bootstrap context, enforces mode restrictions, persists run state, and updates the active plan cache.
- The tooling layer owns `exec`, `ot`, approval classification, mode restrictions, command allowlisting, and workspace path confinement.
- The provider layer owns provider HTTP requests and stream parsing.

## Verification Handoff

Validation was not executed during this update.

- `gofmt -w .`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run ./...`
