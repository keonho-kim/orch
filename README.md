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
| Primary interfaces | `orch`, `orch exec`, `orch history`, standalone `ot` |
| Roles | gateway and worker |
| Providers | `ollama`, `vllm` |
| Workspace | provisioned `test-workspace/` |
| Session storage | `.orch/sessions/*.jsonl` + `.meta.json` |
| Model-facing tool | `ot` only |
| Continuity | compact summary + post-compact records |

## Key Runtime Changes

- `PRODUCT.md` is no longer provisioned into `test-workspace/` and is no longer injected into provider calls.
- `runtime-asset/bootstrap/AGENTS.md` is now a small shared charter.
- role-specific prompts live under `bootstrap/system-prompt/gateway/AGENTS.md` and `bootstrap/system-prompt/worker/AGENTS.md`
- `bootstrap/TOOLS.md` is the canonical OT tool guide for prompting
- the model no longer receives the generic `exec` tool
- gateway and worker receive different OT capabilities
- `bootstrap/USER.md` is shared user memory across sessions in the workspace
- `.orch/chatHistory.md` is shared conversation memory with session-tagged entries and bounded prompt loading

## OT Contract

Gateway OT operations:

- `delegate`
- `read`
- `list`
- `search`

Worker OT operations:

- `read`
- `list`
- `search`
- `write`
- `patch`
- `check`
- `complete`
- `fail`

Plan mode remains read-only and allows only:

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
