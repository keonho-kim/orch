---
description: Read the workspace contract files before doing non-trivial work and keep USER.md focused on durable user information.
---

# Workspace Bootstrap

Use this skill when the task requires orientation or when the workspace contract feels ambiguous.

Workflow:
1. Read `AGENTS.md`.
2. Read `bootstrap/USER.md`.
3. Read `bootstrap/SKILLS.md` only if the task needs a skill.
4. Read only the relevant `bootstrap/skills/<skill-name>/` directories.
5. Read the role-specific prompt under `bootstrap/system-prompt/<role>/AGENTS.md` when role behavior matters.

Rules:
- Keep `bootstrap/USER.md` for durable user facts and preferences.
- Do not duplicate large policy documents inside bootstrap files.
- Stay focused on the current task and avoid inventing extra workflow layers.
