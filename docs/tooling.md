# Tooling

## Model-Facing Contract

The model receives one structured callable tool: `ot`.

`exec` is no longer part of the model-facing contract.

`bootstrap/TOOLS.md` is the text-form tool guide used for prompting.

## Role-Specific OT Operations

### Gateway

- `context`
- `task_list`
- `task_get`
- `delegate`
- `read`
- `list`
- `search`

### Worker

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

### Plan Mode

Plan mode is read-only:

- `context`
- `task_list`
- `task_get`
- `read`
- `list`
- `search`

## Notes

- The model no longer authors raw shell commands.
- The model no longer controls `cwd`, `stdin`, or freeform argv.
- Writes, patches, and checks are explicit worker operations.
- Gateway delegation uses child worker runs under the hood.
- `delegate` now supports sync and async worker starts through the same hidden child-run path.
- `task_list` and `task_get` expose delegated child runs as first-class tasks derived from session metadata.
- `context` exposes the persisted prompt-input snapshot for the active run.
- Standalone `ot` still exists as a CLI binary, but the model-facing tool contract is the structured `ot` schema.
