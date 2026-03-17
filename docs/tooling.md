# Tooling

## Model-Facing Surface

The model receives one structured callable tool: `exec`.

`exec` is the transport contract.

Inside `exec`, `orch` exposes one built-in CLI surface: `ot`.

## ReAct vs Plan

| Mode | Allowed behavior |
| --- | --- |
| `ReAct` | full approved local command surface |
| `Plan` | `cd`, `ot read`, `ot list`, `ot search` only |

## Built-in CLI Surface: `ot`

| Command | Purpose |
| --- | --- |
| `ot read --path <path>` | file content or quick directory inspection |
| `ot list [--path <path>]` | long listing |
| `ot search [--path <path>] [--name <glob>] [--content <pattern>]` | curated name/content search |
| `ot pointer --value <ot-pointer>` | resolve current-session JSONL line references |
| `ot subagent --prompt "<task>"` | bounded child ReAct session |

## Approval Rules

| Case | Approval |
| --- | --- |
| workspace `ot read/list/search` | auto-allowed |
| external read-only OT access | approval required |
| `ot write` | approval required |
| `ot subagent` | approval required unless self-driving mode is enabled |
| `rm`, `mv` | always approval-gated |

## Pointer Semantics

`ot pointer` is intentionally limited:

- current session only
- no cross-session transcript traversal
- reads referenced JSONL lines by pointer value

Example:

```text
ot pointer --value ot-pointer://current?lines=22,23
```

## Reference Syntax in User Prompts

These are not direct OT commands, but they influence prompt assembly:

| Syntax | Meaning |
| --- | --- |
| `@filename` | resolve a workspace file |
| `#dir-name` | resolve a workspace directory |
| `$<skill-name>` | force explicit skill selection |

## Notes

- structured tool schema is always sent on provider calls
- a concise text tool summary is also injected into prompt context
- `ot pointer` and `ot subagent` are `ot` subcommands, not separate model-facing tools
- `ot` remains the policy boundary for most model-driven local interaction
