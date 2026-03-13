# Workspace Agent Guide

Read these files in order before meaningful work:
- `PRODUCT.md`
- `AGENTS.md`
- `bootstrap/USER.md`
- `bootstrap/SKILLS.md`

Purpose of each path:
- `PRODUCT.md` is the product contract. Follow it and do not restate it in bootstrap files.
- `AGENTS.md` describes the workflow for this workspace and how to use the supporting files.
- `bootstrap/USER.md` stores stable user facts and preferences that remain useful across runs.
- `bootstrap/USER.md` may also contain a hidden machine-managed preference block appended by orch.
- `bootstrap/SKILLS.md` is the local skill index.
- `bootstrap/skills/<skill-name>/...` contains the actual skill instructions and helper assets.
- `tools/` contains executable shell scripts used by `ot` and custom workspace commands.

Core rules:
- Stay broad and simple.
- Plan before non-trivial changes.
- Read only the files and skills relevant to the current task.
- Keep tool usage explicit and auditable.
- Prefer `ot read --path <path>` for file and directory inspection.
- Prefer `rg` or `find` for search and discovery work.
- Use custom `bash tools/*.sh` scripts only when the curated OT commands do not cover the task.
- Prefer small, reversible changes.
- Do not invent extra roles, agent hierarchies, or workflow layers unless the product contract explicitly requires them.
