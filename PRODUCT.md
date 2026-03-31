# Product Direction

## Vision

`orch` is a small-LLM-first local agent system for repository work.

The system prioritizes:

- strong controller ownership
- narrow execution scope
- explicit tool contracts
- bounded prompt context
- clean separation between coordination and execution

## Supported Behavior

- The application starts as a Bubble Tea chat UI and supports `orch exec` and `orch history`.
- The runtime provisions `test-workspace/` from bootstrap assets and repository tools.
- `PRODUCT.md` remains a repository document, but it is not part of runtime prompt input.
- The shared runtime prompt base comes from `runtime-asset/bootstrap/AGENTS.md`.
- Role-specific prompts live under `runtime-asset/bootstrap/system-prompt/gateway/AGENTS.md` and `runtime-asset/bootstrap/system-prompt/worker/AGENTS.md`.
- `runtime-asset/bootstrap/TOOLS.md` is the canonical OT tool guide for prompt loading.
- The default execution structure is gateway plus worker, not one unconstrained agent.
- Gateway runs interpret requests, inspect minimal evidence, delegate worker tasks, and synthesize results.
- Worker runs execute only the assigned task contract and must not re-delegate.
- The model-facing tool contract is `ot` only.
- Gateway ReAct allows only `delegate`, `read`, `list`, and `search`.
- Worker ReAct allows `read`, `list`, `search`, `write`, `patch`, `check`, `complete`, and `fail`.
- Plan mode remains read-only and allows only `read`, `list`, and `search`.
- Session storage remains under `.orch/sessions`.
- Compact summaries remain the main reinjection mechanism.
- `bootstrap/USER.md` is workspace-shared durable user memory.
- `.orch/chatHistory.md` is workspace-shared conversation memory with session-tagged entries and bounded prompt loading.
- Child runs are treated as worker task executions with task metadata and parent lineage.

## Tool Policy

- No model-facing `exec` tool exists.
- The model does not shape raw shell commands, cwd, stdin, or argv.
- OT operations are role-limited and mode-limited.
- Outside-workspace model inspection is not part of the strict runtime path.
- Writes, patches, and checks remain approval-gated worker operations.

## Prompting Model

- Always inject:
  - common bootstrap `AGENTS.md`
  - role-specific system prompt
- Load dynamically when needed:
  - `bootstrap/TOOLS.md`
  - selected skills
  - resolved references
  - bounded excerpts from `bootstrap/USER.md`
  - bounded excerpts from `.orch/chatHistory.md`
  - active plan for gateway runs
- Do not inject:
  - `PRODUCT.md`
  - large fixed developer guidance
  - full `chatHistory.md`

## Verification Handoff

Validation executed for this change:

- `go test ./...`
