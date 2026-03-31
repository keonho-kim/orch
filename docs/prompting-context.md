# Prompting Context

## Default Prompt Inputs

The default provider call now uses:

- shared bootstrap `AGENTS.md`
- role-specific system prompt
- `bootstrap/TOOLS.md`
- compact summary plus post-compact records
- current user request or worker task contract
- dynamically loaded evidence blocks when needed

## Removed From Default Runtime Prompting

- `PRODUCT.md`
- large repo-level developer guidance
- full `chatHistory.md`
- full skills index on every call

## Dynamic Loading

The runtime now prefers loading context only when needed:

- `bootstrap/TOOLS.md`
- selected skills
- resolved references
- bounded user memory from `bootstrap/USER.md`
- bounded shared conversation memory from `.orch/chatHistory.md`
- active cached plan for gateway runs
- worker task title, contract, and task status

## Role Split

### Gateway

- interprets request
- decomposes work
- delegates worker tasks
- synthesizes final answer

### Worker

- executes assigned contract only
- does not re-delegate
- returns bounded results
