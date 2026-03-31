# OT Tools

This file is the canonical OT tool guide for runtime prompting.

Gateway operations:
- `delegate`
- `read`
- `list`
- `search`

Worker operations:
- `read`
- `list`
- `search`
- `write`
- `patch`
- `check`
- `complete`
- `fail`

Plan mode:
- `read`
- `list`
- `search`

Rules:
- Use only OT operations allowed for the active role and mode.
- Do not invent shell commands or freeform execution behavior.
- Gateway delegates executable work.
- Worker executes only the assigned task contract.
