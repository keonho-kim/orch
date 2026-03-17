# TUI

## Overview

The TUI is a Bubble Tea console interface for the same runtime that powers `orch exec`.

It is session-oriented rather than shell-oriented.

## Main Interaction Model

| Area | Responsibility |
| --- | --- |
| timeline | rendered runs, streamed output, reasoning visibility |
| composer | prompt entry, slash commands, mode-aware submission |
| status line | current state, warnings, result feedback |
| modal flows | approvals, settings, session history, exit confirmation |

## Composer Modes

Two sticky composer modes exist:

- `ReAct Deep Agent`
- `Plan`

`Shift+Tab` switches between them.

## Slash Commands

When the input starts with `/`, the TUI shows a dropdown above the composer.

Current behavior:

- `Up` / `Down` navigates dropdown entries
- `Tab` or `Enter` completes a partial slash command selection
- once fully selected, `Enter` executes the slash command normally

Current slash commands:

| Command | Meaning |
| --- | --- |
| `/clear` | open a new session and clear the visible conversation |
| `/compact` | compact the current session |
| `/exit` | quit, cancelling active runs after confirmation if needed |

## Message History and Navigation

| Key | Behavior |
| --- | --- |
| `Up` / `Down` | recall past prompts when slash menu is not visible |
| `PgUp` / `PgDn` | scroll timeline |
| `Home` / `End` | jump to top or bottom |
| mouse wheel | scroll timeline |

## Thinking Visibility

`Ctrl+T` toggles reasoning visibility for the active run.

Rendering modes:

- expanded `THINK` box
- collapsed `THINKING ...` placeholder

## Session History Picker

`orch history` enters the same TUI runtime and opens a session picker.

Picker behavior:

- `Up` / `Down` moves selection
- `Enter` restores the selected session
- `Esc` closes the picker

## `/clear` Semantics

`/clear` no longer means “hide old text in the current viewport only”.

It now means:

1. open a new session
2. clear the visible conversation by switching to that new session
3. finalize the old session in the background

Constraint:

- a new session is not opened while a run is still active

## View Rendering

The TUI renders:

- the `ORCH` wordmark and version in the scrollable timeline
- one section per run
- `USER` and `ORCH` blocks
- a separate reasoning block for the active run
- a right-aligned command meta chip in the composer row

## Settings and Exit

| Key | Behavior |
| --- | --- |
| `Ctrl+S` | open settings |
| `Ctrl+C` | quit immediately |
| `/exit` | controlled shutdown path |

`/exit` uses a confirmation modal if a run is active.
