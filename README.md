# orch

`orch` is a Go-based local agent runner for repository work. It uses Ollama or vLLM for inference, provisions one shared runtime workspace at `./test-workspace`, and exposes one model-facing tool: `exec`.

## Supported Behavior

- `orch` and `orch tui` start the Bubble Tea console chat UI.
- The conversation timeline starts with an ASCII `ORCH` wordmark and version metadata as scrollable content.
- `orch exec [--mode react|plan] "<request>"` runs one request without the TUI and streams output to stdout.
- The TUI has two sticky composer modes:
  - `ReAct Deep Agent`
  - `Plan`
- Dashboard `Shift+Tab` toggles the composer between those two modes.
- Prompt history is available on `Up` and `Down`.
- Timeline history is available through in-app scrolling with mouse wheel, `PgUp`/`PgDn`, and `Home`/`End`.
- `Ctrl+T` is labeled `Show Think` and toggles active-run reasoning visibility.
- `Ctrl+S` opens settings. `/exit` requests shutdown and cancels an active run before quitting.
- `auto_translate` detects one language bucket (`kor`, `en`, or `ch`), translates the input into the other two languages in parallel, and sends the combined three-language request bundle into the main agent loop.
- The first detected language can be stored as a hidden machine-managed preference block inside `test-workspace/bootstrap/USER.md`.
- The TUI continues to show only the original user input; translated variants are internal preprocessing data and are not rendered on screen.
- The active run shows streamed reasoning in a subtle `THINK` box separate from the `ORCH` response, or as `THINKING ...` when collapsed.
- The TUI renders lightweight inline terminal styling for `**bold**`, `*italic*`, and `__underline__` text in streamed output.
- Streamed `Thinking` and `Response` content wraps to the current pane width, including narrow `tmux` splits.
- The runtime workspace syncs `PRODUCT.md`, `AGENTS.md`, `bootstrap/SKILLS.md`, `bootstrap/skills/**`, and `tools/**` into `./test-workspace` while preserving `bootstrap/USER.md`.
- `test-workspace` is provisioned output, not source-of-truth content. No `.claude` mirror is part of the supported workspace contract.
- First-launch setup is guided:
  - choose `Ollama` or `vLLM`
  - `vLLM` enters the manual settings form
  - `Ollama` enters a guided setup flow with default-or-custom URL selection, connection check, local model discovery, and final model selection before saving

## Ralph Loop

- ReAct runs use the Ralph loop with `react_ralph_iter`.
- Plan runs use the Ralph loop with `plan_ralph_iter`.
- On every Ralph iteration, orch re-reads and re-injects:
  - `PRODUCT.md`
  - `AGENTS.md`
  - `bootstrap/USER.md`
  - `bootstrap/SKILLS.md`
- Plan runs also re-inject the current draft plan each iteration.
- A successful Plan run becomes the active cached plan and is automatically re-injected into later ReAct runs until replaced.

## Tooling Model

- The model sees one callable tool named `exec`.
- ReAct mode:
  - prefers `ot read --path <path>` for file and directory inspection
  - uses `rg` or `find` for search and discovery
  - may use `bash tools/<script>.sh ...` for custom scripts
  - may use direct toolchain commands when task execution requires them
- Plan mode:
  - allows only `cd <path>`
  - allows only `ot read --path <path>`
- Approval policy:
  - `ot read` is always allowed
  - `ot write` requires approval
  - every other command requires approval unless self-driving mode is enabled
  - self-driving mode auto-approves everything except `rm` and `mv`

## Configuration

- Repository-local settings live in [orch.settings.json](/Users/khkim/dev/orch/orch.settings.json).
- Supported providers are `ollama` and `vllm`.
- Each provider has a `base_url` and `model`. vLLM also uses `api_key_env` for bearer auth.
- Ralph settings:
  - `react_ralph_iter`
  - `plan_ralph_iter`
- `self_driving_mode` affects ReAct mode only.
- `auto_translate` controls the multilingual preprocessing path.

## Layout

- [cmd/cli.go](/Users/khkim/dev/orch/cmd/cli.go): main CLI entrypoints and `orch exec`
- [cmd/ot/main.go](/Users/khkim/dev/orch/cmd/ot/main.go): curated tool CLI
- [domain/types.go](/Users/khkim/dev/orch/domain/types.go): provider, mode, settings, run, and plan-cache models
- [internal/orchestrator/service.go](/Users/khkim/dev/orch/internal/orchestrator/service.go): run lifecycle, plan cache, and persistence coordination
- [internal/orchestrator/loop.go](/Users/khkim/dev/orch/internal/orchestrator/loop.go): Ralph loop and bootstrap refresh
- [internal/tooling/executor.go](/Users/khkim/dev/orch/internal/tooling/executor.go): mode-aware command policy and execution
- [internal/adapters/ollama.go](/Users/khkim/dev/orch/internal/adapters/ollama.go): Ollama URL normalization and local model discovery
- [internal/tui/settings_state.go](/Users/khkim/dev/orch/internal/tui/settings_state.go), [internal/tui/settings_update.go](/Users/khkim/dev/orch/internal/tui/settings_update.go), and [internal/tui/settings_view.go](/Users/khkim/dev/orch/internal/tui/settings_view.go): settings modal state, guided setup flow, and settings rendering

## Validation Handoff

Validation was not executed during this update.

- `gofmt -w .`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run ./...`
