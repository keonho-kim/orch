# Tooling

## Model-Facing Contract

The model receives one structured callable tool: `ot`.

`exec` is no longer part of the model-facing contract.

`bootstrap/TOOLS.md` is the text-form tool guide used for prompting.

## Role-Specific OT Operations

### Gateway

- `delegate`
- `read`
- `list`
- `search`

### Worker

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

- `read`
- `list`
- `search`

## Notes

- The model no longer authors raw shell commands.
- The model no longer controls `cwd`, `stdin`, or freeform argv.
- Writes, patches, and checks are explicit worker operations.
- Gateway delegation uses child worker runs under the hood.
- Standalone `ot` still exists as a CLI binary, but the model-facing tool contract is the structured `ot` schema.
