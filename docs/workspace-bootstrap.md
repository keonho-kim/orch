# Workspace Bootstrap

## Overview

`orch` does not operate directly inside repository source files as its runtime surface.

Instead, it provisions a runtime workspace at:

```text
./test-workspace
```

## Provisioned Inputs

The runtime workspace is rebuilt from repository assets and runtime bootstrap files.

| Source | Target in `test-workspace` |
| --- | --- |
| repo `PRODUCT.md` | `PRODUCT.md` |
| runtime bootstrap `AGENTS.md` | `AGENTS.md` |
| runtime bootstrap `SKILLS.md` | `bootstrap/SKILLS.md` |
| runtime bootstrap `skills/**` | `bootstrap/skills/**` |
| repo `tools/**` | `tools/**` |

## Preserved State

`bootstrap/USER.md` is special.

It is preserved across reprovisioning to keep durable user-facing or machine-managed preference state.

Examples:

- detected language preference block
- durable user notes that belong in bootstrap memory

## What This Means

`test-workspace` is:

- generated runtime content
- not the source-of-truth authoring location for the repository

The source of truth remains in the repository root and runtime bootstrap asset tree.

## Skills Layout

The canonical skill layout at runtime is:

```text
bootstrap/SKILLS.md
bootstrap/skills/<skill-name>/SKILL.md
bootstrap/skills/<skill-name>/...
```

This is the layout referenced by prompt assembly and `$<skill-name>` resolution.

## References and Hidden Files

Workspace references:

- `@filename`
- `#dir-name`

resolve against the current workspace root.

Important behavior:

- hidden files in the workspace, such as `.env`, are indexable
- directory references are also workspace-scoped
- the reference system is separate from bootstrap provisioning itself

## Tools in the Workspace

The `tools/ot/` scripts are provisioned into the runtime workspace so shell-backed OT commands can execute there.

Some `ot` subcommands are handled specially in Go rather than by shell scripts:

- `ot pointer`
- `ot subagent`

## Relationship to Repository Paths

Think of the project in two layers:

| Layer | Purpose |
| --- | --- |
| repository root | source of truth |
| `test-workspace` | provisioned runtime surface |

This split keeps runtime behavior explicit and reproducible while preserving one durable bootstrap memory file.
