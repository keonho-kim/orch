# TUI

## Overview

The TUI is the primary operator interface for `orch`. It uses the same orchestrator service as `orch exec`, but adds session browsing, modal approvals, provider setup, and a persistent timeline view.

It is session-oriented, not shell-oriented.

## Layout Model

| Area | Responsibility |
| --- | --- |
| timeline | rendered run history, streamed output, active-run reasoning |
| composer | prompt entry, slash command discovery, mode-aware submission |
| command meta | current mode, provider, and model |
| status line | inline state, warnings, and feedback |
| modal views | approvals, settings, session history, exit confirmation |

## First-Run Settings Gate

If the current settings do not define a usable default provider and model, the TUI opens directly into setup flow.

Current first-run behavior:

- provider is chosen first
- choosing `Ollama` enters URL selection and live model discovery
- choosing `vLLM` switches to the manual settings form
- choosing other cloud providers switches to the manual settings form
- the setup flow saves into the `user` scope
- the user cannot start runs until the default provider has a configured model

The status line reflects that requirement until settings are saved.

## Composer Modes

The composer has two sticky modes:

- `ReAct Deep Agent`
- `Plan`

`Shift+Tab` toggles between them.

The active mode is shown in the command meta chip at the right side of the composer row.

## Slash Commands

When the current input starts with `/`, the TUI shows a slash-command dropdown above the composer.

Current behavior:

- `Up` / `Down` moves through the dropdown
- `Tab` or `Enter` can complete a selected slash command
- `Enter` executes the command once the full command is selected

Current slash commands:

| Command | Behavior |
| --- | --- |
| `/clear` | open a new session and clear the visible conversation |
| `/compact` | force session compaction |
| `/exit` | quit the application |

## Approval Modal

When a tool call requires approval, the normal dashboard view is replaced by an approval page.

The modal shows:

- run id
- tool name
- approval reason
- raw tool arguments

Controls:

- `Enter` or `Y` to approve
- `Esc` or `N` to deny

## Session History Picker

`orch history` launches the same TUI runtime and opens a session-history picker immediately.

Picker behavior:

- `Up` / `Down` moves selection
- `Enter` restores the selected session
- `Esc` closes the picker

The picker lists saved sessions by session id and title.

## Settings Modal

`Ctrl+S` opens settings.

Current settings UI supports:

- scope tabs for `Effective`, `Managed`, `User`, `Project`, and `Local`
- default provider selection
- `self_driving_mode`
- Ollama base URL and model
- vLLM base URL, model, and API key env name
- Gemini base URL, model, and API key env name
- Vertex base URL, model, and API key env name
- Bedrock base URL, model, and API key env name
- Claude base URL, model, and API key env name
- Azure base URL, deployment name, and API key env name
- ChatGPT base URL, model, and API key env name
- `react_ralph_iter`
- `plan_ralph_iter`
- `compact_threshold_k`

The `Effective` and `Managed` tabs are read-only. The editable tabs show scope-local values and use inherited placeholders for lower-scope values. `Ctrl+U` unsets the focused field in the active editable scope so the effective value falls back to a lower layer. If a field is enforced by managed settings, lower scopes show it as locked.

The first-run setup flow still keeps Ollama discovery, but all cloud providers enter the manual settings form directly and the first saved settings go to the `user` scope.

Changing providers inside the full form requires an explicit confirmation step.

The same provider/model state can also be inspected and updated outside the TUI:

- `orch config --list`
- `orch config --list --scope <managed|user|project|local|effective>`
- `orch config --list --show-origin`
- `orch config --scope <user|project|local> --provider=ollama --model=<name>`
- `orch config --scope <user|project|local> --ollama-base-url=<url>`
- `orch config --scope <user|project|local> --ollama-model=<name>`
- `orch config --scope <user|project|local> --vllm-base-url=<url>`
- `orch config --scope <user|project|local> --vllm-model=<name>`
- `orch config --scope <user|project|local> --vllm-api-key-env=<env>`
- `orch config --scope <user|project|local> --gemini-base-url=<url>`
- `orch config --scope <user|project|local> --gemini-model=<name>`
- `orch config --scope <user|project|local> --gemini-api-key-env=<env>`
- `orch config --scope <user|project|local> --vertex-base-url=<url>`
- `orch config --scope <user|project|local> --vertex-model=<name>`
- `orch config --scope <user|project|local> --vertex-api-key-env=<env>`
- `orch config --scope <user|project|local> --bedrock-base-url=<url>`
- `orch config --scope <user|project|local> --bedrock-model=<name>`
- `orch config --scope <user|project|local> --bedrock-api-key-env=<env>`
- `orch config --scope <user|project|local> --claude-base-url=<url>`
- `orch config --scope <user|project|local> --claude-model=<name>`
- `orch config --scope <user|project|local> --claude-api-key-env=<env>`
- `orch config --scope <user|project|local> --azure-base-url=<url>`
- `orch config --scope <user|project|local> --azure-model=<deployment>`
- `orch config --scope <user|project|local> --azure-api-key-env=<env>`
- `orch config --scope <user|project|local> --chatgpt-base-url=<url>`
- `orch config --scope <user|project|local> --chatgpt-model=<name>`
- `orch config --scope <user|project|local> --chatgpt-api-key-env=<env>`
- `orch config --scope <user|project|local> --approval-policy=<policy>`
- `orch config --scope <user|project|local> --self-driving-mode=<true|false>`
- `orch config --scope <user|project|local> --react-ralph-iter=<n>`
- `orch config --scope <user|project|local> --plan-ralph-iter=<n>`
- `orch config --scope <user|project|local> --compact-threshold-k=<n>`
- `orch config --scope <user|project|local> --unset <key>`

## Thinking Visibility

`Ctrl+T` toggles reasoning visibility for the active run.

Rendering modes:

- expanded bordered `THINK` block
- collapsed `THINKING ...` placeholder

Reasoning is visible in the TUI only. It is not written into the session transcript files.

## Navigation

| Key | Behavior |
| --- | --- |
| `Up` / `Down` | prompt history recall when slash menu is hidden |
| `PgUp` / `PgDn` | page timeline |
| `Home` / `End` | jump to top or bottom |
| mouse wheel | scroll timeline |
| `Ctrl+S` | open settings |
| `Ctrl+T` | toggle active-run reasoning |
| `Ctrl+C` | quit immediately |

## `/clear` And `/exit`

`/clear` means:

1. create a fresh session
2. switch the UI to that session
3. clear the visible conversation
4. finalize the old session in the background

Constraint:

- it does not open a new session while a run is active

`/exit` is the controlled shutdown path. If a run is active, the TUI shows a confirmation modal before quitting.

## Rendering Notes

The timeline renders:

- the `ORCH` wordmark and version
- one section per run
- `USER` and `ORCH` content blocks
- a separate reasoning block for the active run

Assistant output supports lightweight inline terminal styling for:

- `**bold**`
- `*italic*`
- `__underline__`
