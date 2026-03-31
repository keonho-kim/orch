# System Overview

`orch` is now organized as a two-layer agent system:

- gateway for interpretation, delegation, and synthesis
- worker for bounded execution

## Core Flow

```mermaid
flowchart TD
    U["User request"] --> G["Gateway run"]
    G --> D["Create worker task contracts"]
    D --> W["Worker runs"]
    W --> R["Worker results"]
    R --> G2["Gateway synthesis"]
```

## Key Runtime Properties

- OT-only model-facing tool contract
- role-specific prompt loading
- dynamic evidence loading
- compact-based continuity
- child-session lineage for delegated work
