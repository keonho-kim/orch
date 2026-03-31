# Workspace Bootstrap

## Runtime Workspace

`orch` provisions a runtime workspace at `./test-workspace`.

Runs execute against that workspace, not directly against repository authoring paths.

## Synced Inputs

- repo `tools/**` -> `test-workspace/tools/**`
- bootstrap `AGENTS.md` -> `test-workspace/AGENTS.md`
- bootstrap `TOOLS.md` -> `test-workspace/bootstrap/TOOLS.md`
- bootstrap `SKILLS.md` -> `test-workspace/bootstrap/SKILLS.md`
- bootstrap `skills/**` -> `test-workspace/bootstrap/skills/**`
- bootstrap `system-prompt/**` -> `test-workspace/bootstrap/system-prompt/**`

## Preserved Input

- `test-workspace/bootstrap/USER.md`

`bootstrap/USER.md` is the shared durable user-memory file for the workspace.

## Removed Input

- repo `PRODUCT.md` is no longer provisioned into `test-workspace/`

## Why

This keeps runtime prompts small and role-specific, and avoids injecting large product guidance into smaller models.
