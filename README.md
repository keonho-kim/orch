# orch

`orch` is a local repository agent system optimized for smaller LLMs.

The runtime now centers on:

- a gateway agent that interprets requests and delegates execution work
- worker agents that do only the assigned task contract
- an OT-only model-facing tool contract
- small role-specific system prompts loaded from bootstrap assets
- dynamic context loading instead of large fixed prompt injection

## Current Model

| Area | Behavior |
| --- | --- |
| Primary interfaces | `orch`, `orch exec`, `orch config`, `orch history`, standalone `ot` |
| Roles | gateway and worker |
| Providers | `ollama`, `vllm`, `gemini`, `vertex`, `bedrock`, `claude`, `azure`, `chatgpt` |
| Workspace | provisioned `test-workspace/` |
| Session storage | `.orch/sessions/*.jsonl` + `.meta.json` |
| Attached API | localhost-only HTTP + SSE during interactive TUI sessions |
| Model-facing tool | `ot` only |
| Continuity | compact summary + post-compact records |

## Configuration

`orch` now resolves settings from four JSON scopes:

1. `managed`
2. `local`
3. `project`
4. `user`
5. built-in defaults

`managed` is machine policy and is read-only from the CLI and TUI. CLI flags apply only to the current command and sit above the persisted scopes.

Scope files:

- `managed`: `/Library/Application Support/orch/managed-settings.json` on macOS, `/etc/orch/managed-settings.json` on Linux/WSL, `%ProgramData%/orch/managed-settings.json` on Windows, or `ORCH_MANAGED_SETTINGS=<path>` for test and development overrides
- `user`: `${os.UserConfigDir()}/orch/settings.json`
- `project`: `<repo>/orch.settings.json`
- `local`: `<repo>/.orch/settings.local.json`

Use the TUI settings flow for interactive setup, or use the CLI for inspection and updates:

```bash
orch config --list
orch config --list --show-origin
orch config --list --scope project
orch config --scope user --provider=ollama --model=qwen3.5:35b
orch config --scope project --ollama-base-url=http://localhost:11434/v1 --self-driving-mode=true
orch config --scope local --chatgpt-model=gpt-4.1 --chatgpt-api-key-env=OPENAI_API_KEY
orch config --scope local --unset providers.chatgpt.model
orch config --scope project --azure-base-url=https://example.openai.azure.com --azure-model=my-deployment --azure-api-key-env=AZURE_OPENAI_API_KEY
```

`orch config --list` prints the effective normalized runtime settings as flat `key=value` lines. Use `--scope <managed|user|project|local|effective>` to inspect a single layer, `--show-origin` to append the source scope and file for each effective key, and `--unset <key>` on editable scopes to fall back to lower layers.

Scope-less writes still target the `project` scope for backward compatibility.

Supported write flags:

- `--provider`
- `--model` when paired with `--provider`
- `--ollama-base-url`
- `--ollama-model`
- `--vllm-base-url`
- `--vllm-model`
- `--vllm-api-key-env`
- `--gemini-base-url`
- `--gemini-model`
- `--gemini-api-key-env`
- `--vertex-base-url`
- `--vertex-model`
- `--vertex-api-key-env`
- `--bedrock-base-url`
- `--bedrock-model`
- `--bedrock-api-key-env`
- `--claude-base-url`
- `--claude-model`
- `--claude-api-key-env`
- `--azure-base-url`
- `--azure-model`
- `--azure-api-key-env`
- `--chatgpt-base-url`
- `--chatgpt-model`
- `--chatgpt-api-key-env`
- `--approval-policy`
- `--self-driving-mode`
- `--react-ralph-iter`
- `--plan-ralph-iter`
- `--compact-threshold-k`

Provider notes:

- `azure.model` is the Azure deployment name.
- `vertex` uses Vertex AI Express Mode API-key auth.
- `bedrock.base_url` must point at the regional Bedrock Mantle `/v1` endpoint.

## Key Runtime Changes

- `PRODUCT.md` is no longer provisioned into `test-workspace/` and is no longer injected into provider calls.
- `runtime-asset/bootstrap/AGENTS.md` is now a small shared charter.
- role-specific prompts live under `bootstrap/system-prompt/gateway/AGENTS.md` and `bootstrap/system-prompt/worker/AGENTS.md`
- `bootstrap/TOOLS.md` is the canonical OT tool guide for prompting
- the model no longer receives the generic `exec` tool
- gateway and worker receive different OT capabilities
- `bootstrap/USER.md` is shared user memory across sessions in the workspace
- `.orch/chatHistory.md` is shared conversation memory with session-tagged entries and bounded prompt loading
- delegated child worker sessions are exposed as first-class tasks derived from session metadata
- each run persists a context snapshot so the active prompt inputs can be inspected later
- interactive `orch` sessions now expose a local attached HTTP API server with repo-local discovery files under `.orch/api/`

## Local API

Interactive TUI launches now start an attached local HTTP API server in the same `orch` process.

- bind: `127.0.0.1:<ephemeral-port>`
- auth: `Authorization: Bearer <token>`
- discovery files:
  - `.orch/api/<session-id>.json`
  - `.orch/api/current.json`
- lifecycle: the attached API server exits when the interactive `orch` session exits

Core endpoint mapping:

- `orch exec` -> `POST /v1/exec`, `GET /v1/exec/{run_id}`, `GET /v1/exec/{run_id}/events`, `POST /v1/exec/{run_id}/approval`
- `orch history` -> `GET /v1/history`, `GET /v1/history/latest`, `POST /v1/history/restore`
- `orch config` -> `GET /v1/config`, `PATCH /v1/config`
- session/process status -> `GET /v1/status`
- global event stream -> `GET /v1/events`

## OT Contract

Gateway OT operations:

- `context`
- `task_list`
- `task_get`
- `delegate`
- `read`
- `list`
- `search`

Worker OT operations:

- `context`
- `task_list`
- `task_get`
- `read`
- `list`
- `search`
- `write`
- `patch`
- `check`
- `complete`
- `fail`

Plan mode remains read-only and allows only:

- `context`
- `task_list`
- `task_get`
- `read`
- `list`
- `search`

## Bootstrap Layout

Runtime bootstrap inputs:

```text
AGENTS.md
bootstrap/TOOLS.md
bootstrap/USER.md
bootstrap/SKILLS.md
bootstrap/skills/**
bootstrap/system-prompt/gateway/AGENTS.md
bootstrap/system-prompt/worker/AGENTS.md
tools/**
```

`bootstrap/USER.md` is preserved across reprovisioning and acts as shared durable user memory.

## Validation

```bash
go test ./...
```
