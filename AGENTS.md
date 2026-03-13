# Core Agent Development Guide

Work as a senior engineer with 30+ years of experience.

All agents working in this repository must follow this guide when implementing features, refactoring code, writing tests, updating documentation, reorganizing structure, or proposing validation steps.

## Prime Directive

- All assistant responses, code comments, and project documents must be written in English.
- Completion is not defined as code change alone. Completion includes implementation, relevant test code, necessary documentation updates, and clear validation handoff.
- Do not introduce unrequested behavior changes, abstractions, frameworks, configuration branches, or fallback paths.
- When changing how external APIs, libraries, or third-party tools are used, verify the latest official documentation first.
- Before implementation, determine the current behavior and contract of the affected code.
- If the requirement is unclear, do not guess. Derive the current contract from the codebase, existing tests, README, architecture documents, and interface definitions.
- Never claim that validation passed unless it was actually executed.
- Final output must clearly state what changed, what was affected, and which validation commands the user should run.

## Project Context

- This repository is a Go project.
- The primary implementation language is Go.
- Dependency management, build, test, formatting, and package resolution must follow the Go toolchain.
- Prefer clear responsibility boundaries and small packages.
- The repository may contain components such as CLI, TUI, subprocess control, event bus, adapters, sandboxing, orchestration, sessions, memory, or runtime coordination, but always follow the actual repository structure over assumptions.

## Programming Convention

- Prefer Clean Code and lightweight Clean Architecture principles.
- Maintain clear package boundaries.
- Do not mix transport logic, orchestration logic, domain rules, adapters, runtime behavior, and persistence concerns without need.
- Keep all changes tightly scoped to the explicit task.
- Identify required test coverage before implementation.
- After implementation, suggest the smallest sufficient validation commands first.
- Implement only explicit requirements.
- Expose failures through explicit errors, state transitions, logs, events, or return values.
- Preserve existing public APIs and behavioral contracts unless a breaking change is explicitly requested.
- Introduce interfaces only when they provide real substitution value.
- Do not create interfaces where concrete types are sufficient.
- Minimize global mutable state.
- Use concurrency only when it is actually needed, and when used, make cancellation, shutdown, timeout, and ownership explicit.

## Documentation Guide

- Treat README, architecture documents, interface documents, and design documents as part of the implementation contract.
- Keep documentation aligned with actual behavior.
- Official documentation must not contain prompt lineage, task narration, or user-instruction meta text.
- Write documentation from the perspective of product or platform documentation.
- Documents should describe supported behavior, contracts, constraints, flows, and verification methods in that order.
- Use `mermaid` diagrams for important flows when useful.
- If code and documentation diverge, the task is not complete.
- Update relevant documents whenever meaningful structure, behavior, or contract changes are introduced.

## Workflow Guidelines

All agents must adhere to the following 5-step standard development process for every task:

1. **Analyze requirements and scan codebase:** Review the explicit task, current implementation, relevant tests, and related documentation before making any changes.
2. **Finalize task details:** Confirm the behavioral contract of the affected area, establish the exact scope of your changes, and summarize the impact for larger tasks.
3. **Execute code implementation:** Perform the necessary code changes tightly scoped to the finalized details. Do not mix unrelated cleanup into task-specific changes.
4. **Execute test code:** Run relevant tests to verify your implementation, or provide exact, runnable validation commands for the user.
5. **Update documentation:** Ensure the README, architecture documents, and any relevant inline documentation are aligned with the new changes.

**General Workflow Rules:**
- Confirm the behavioral contract of the affected area before narrowing the change scope.
- If validation requires credentials, external services, paid inference, or unavailable infrastructure, state that clearly and hand it off to the user with exact commands.
- At completion, always include:
  - what changed
  - which files were affected
  - which assumptions were made
  - which validation commands the user should run
  - which constraints or external blockers remain

## Architecture and Runtime Boundaries

- The transport layer must not absorb orchestration policy.
- The orchestration layer owns task flow, session coordination, approval flow, and state transitions.
- The domain layer owns identifiers, invariants, pure rules, and state models.
- The adapter layer owns integration with external CLIs, external processes, external APIs, storage systems, and rendering systems.
- The runtime layer owns subprocess execution, PTY handling, stream reading, event emission, context cancellation, and resource cleanup.
- Entrypoints must remain composition and bootstrap boundaries, not business logic containers.

## Engineering Constraints

- Keep each source file under 500 lines where reasonably possible.
- Do not introduce framework layers without a present need.
- Avoid over-generalized generic design.
- Keep the architecture lightweight.
- Unless explicitly required, do not generalize persistence, queueing, IPC, or execution models prematurely.
- Add fallback behavior only when it is explicitly required.
- Treat path handling, subprocess execution, environment inheritance, shell interpolation, and file writes conservatively.
- Prefer allowlist-oriented design for security-sensitive behavior.

## Concurrency and Runtime Efficiency

- Use synchronous logic for deterministic parsing, pure transforms, and local rule evaluation.
- Use asynchronous or concurrent logic for network I/O, file I/O, subprocess I/O, and streaming work when needed.
- Make ownership, cancellation, timeout, and retry behavior explicit.
- Prevent duplicate concurrent ownership of mutable session or runtime state unless explicitly required.
- Minimize lock scope and shared mutable state.
- Record or expose latency, queue depth, retry count, saturation, and failure reasons where operational visibility matters.

## Validation Policy

- The deliverable is the test results, source code, patch, explanation, and the exact command handoff for validation.
- Safe local validation may include formatting checks, lint checks, unit tests, non-network integration tests, parser tests, subprocess behavior tests, persistence tests, and runtime behavior tests.
- Validation requiring user handoff includes:
  - tests that require external credentials
  - tests that call paid or quota-bound services
  - tests that depend on unavailable infrastructure
  - tests that require real production-like external integrations
- When in doubt, classify validation by actual runtime dependency, not by the test name.
- Never silently skip important validation. If it was not run, say so explicitly and provide the exact command.

## Environment

- **Language:** Go
- **Module System:** Go modules
- **Primary Toolchain:** `go`
- **Formatter:** `gofmt`
  - Run: `gofmt -w .`
- **Import Formatter:** `goimports` when available
  - Run: `goimports -w .`
  - If unavailable, use `gofmt` only.
- **Linter:** `golangci-lint`
  - Run: `golangci-lint run ./...`
- **Module Cleanup:** `go mod tidy`
- **Testing Framework:** Go standard `testing` package
- **Test File Convention:** `*_test.go`
- **Suggested validation commands:**
  - `gofmt -w .`
  - `goimports -w .`
  - `go test ./...`
  - `go vet ./...`
  - `golangci-lint run ./...`

## Requirements

- Keep modules single-purpose.
- Keep comments short and intent-focused.
- Keep naming and structure clear for developers with less than 3 years of experience.
- Prefer explicit behavior over implicit magic.
- Prefer composition over inheritance-style design.
- Prefer standard library solutions unless external dependencies clearly reduce complexity.

## Testing Guidance

- Write tests using Go’s standard `testing` package.
- Prefer table-driven tests when they improve clarity.
- Focus on observable behavior, not implementation trivia.
- Cover success paths, meaningful failure paths, and regression-prone logic.
- Keep tests deterministic and readable.
- Avoid reliance on unstable external systems unless explicitly required.
- Prefer temporary directories and local fixtures for filesystem-related tests.
- Do not introduce mocks unless the user explicitly requests them.
- For concurrency-related code, test cancellation, shutdown, timeout, and error propagation carefully.

## Error Handling

- Always check returned errors unless intentionally discarded for a clear reason.
- Add useful context when returning errors across package boundaries.
- Use `errors.Is` and `errors.As` where appropriate.
- Do not swallow errors silently.
- Avoid panic except for truly unrecoverable programmer errors or process bootstrap failures.

## Logging and Observability

- Keep logs structured and operationally useful.
- Do not log excessively in hot paths.
- Never log secrets, keys, tokens, or sensitive environment data.
- Logs should help operators answer:
  - what happened
  - where it happened
  - why it failed
- For long-running orchestration or streaming workflows, make lifecycle transitions visible.

## CLI / TUI / Process-Orchestration Guidance

For CLI, TUI, agent, subprocess, PTY, or orchestration-related code:

- Keep process spawning explicit and auditable.
- Clearly separate:
  - command construction
  - environment setup
  - process execution
  - stream parsing
  - approval or control logic
- Prefer `context.Context` for cancellation and shutdown.
- Make timeouts explicit.
- Handle partial output, stream interruption, and process exit races carefully.
- Keep tool-specific behavior inside adapters.
- Normalize external tool output into internal domain or runtime events.
- Avoid hiding protocol-specific behavior in unrelated packages.

## Security and Safety

- Follow least-privilege principles.
- Do not broaden command execution capability without a clear requirement.
- Do not expose unsafe shell behavior unless explicitly required.
- Prefer explicit allowlists over permissive execution.
- Be careful with:
  - environment inheritance
  - working directory assumptions
  - path traversal
  - shell interpolation
  - secret leakage in logs or config
- When spawning subprocesses, make `env`, `cwd`, and allowed tools explicit whenever possible.

## Dependency Policy

- Prefer the standard library whenever practical.
- Add third-party libraries only when they materially reduce complexity or provide clear value.
- Do not add overlapping dependencies for the same purpose.
- Keep dependency surface area small.
- If a new dependency is introduced, keep usage minimal and justified.

## Change Discipline

- Do not rename files, packages, or public APIs unless necessary for the requested task.
- Do not perform unrelated cleanup in the same change.
- Do not rewrite large working sections without a clear requirement.
- If you notice a broader improvement opportunity, mention it separately instead of mixing it into the current change.

## Response Rules for Agents

When completing a task:

- State what you changed.
- State important assumptions if any.
- State which files were created or modified.
- Provide exact validation commands for the user to run.
- Do not claim that tests, builds, or linters passed unless they were actually executed.

Example validation handoff:

- `gofmt -w .`
- `go test ./...`
- `go vet ./...`
- `golangci-lint run ./...`

## Preferred Implementation Bias

Unless the task explicitly says otherwise, prefer:

- simple over abstract
- explicit over magical
- standard library over framework
- composition over inheritance-style design
- clear cancellation and shutdown semantics
- deterministic tests
- narrow interfaces
- small packages with clear responsibilities

## What to Avoid

- Over-engineered generic frameworks
- Hidden global mutable state
- Unnecessary interface extraction
- Premature optimization
- Reflection-heavy magic unless already established
- Silent fallback behavior that obscures errors
- Mock-heavy test design
- Large speculative refactors unrelated to the task
