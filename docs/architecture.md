# Architecture

## Ownership

- `internal/cli`: entrypoints and hidden child-run commands
- `internal/apiserver`: attached local HTTP API, auth, SSE, and discovery-file lifecycle
- `internal/orchestrator`: gateway/worker coordination and ReAct loop
- `internal/session`: transcript, metadata, compact, lineage
- `internal/tooling`: OT execution and approval policy
- `internal/adapters`: provider transport and tool schema delivery
- `internal/workspace`: runtime workspace provisioning

## Important Shift

The central architectural change is that the runtime is no longer modeled as one broad agent with a generic execution tool.

It is now modeled as:

- a gateway coordinator
- worker executors
- an OT-only structured tool surface
