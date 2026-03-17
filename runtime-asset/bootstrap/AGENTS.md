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
- A user may explicitly request a skill via `$<skill-name>`.
- `tools/` contains executable shell scripts used by `ot` and custom workspace commands.

Core rules:
- Stay broad and simple.
- Plan before non-trivial changes.
- Read only the files and skills relevant to the current task.
- Keep tool usage explicit and auditable.
- Prefer `ot read --path <path>` for file content and quick directory inspection.
- Prefer `ot list [--path <path>]` for long directory listings.
- Prefer `ot search [--path <path>] [--name <glob>] [--content <pattern>]` for curated name and content search.
- Use `ot pointer --value <ot-pointer>` when compact output or `chatHistory.md` references current-session transcript pointers.
- Every provider call includes a concise text tool summary and the current skills index in prompt context.
- Use `ot subagent --prompt <task>` only when a bounded child run should work in a separate child session.
- User prompts may include `@filename` and `#dir-name` references that orch resolves against the current workspace.
- Use direct `rg` or `find` only when the curated OT commands do not cover the search behavior you need.
- Use `/clear` to open a new session and clear the visible conversation. It does not delete saved session history.
- Use `/compact` only when you need to force session compaction before the automatic threshold.
- Use custom `bash tools/*.sh` scripts only when the curated OT commands do not cover the task.
- Prefer small, reversible changes.
- Do not invent extra roles, agent hierarchies, or workflow layers unless the product contract explicitly requires them.
