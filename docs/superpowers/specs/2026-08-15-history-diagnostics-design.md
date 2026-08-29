# History Diagnostics Design

## Goal

Turn History into a diagnostic browser for completed project runs while keeping
command reuse and rerun actions secondary. Historical inspection stays inside
the overlay and never replaces the live Tasks or Output state.

## Chosen approach

Use a responsive master-detail overlay with two tabs:

- `Project runs` opens by default and owns diagnostic navigation.
- `Commands` exposes global command history as a secondary picker.

Two alternatives were rejected:

- Rehydrating the main Tasks and Output panels would displace live state and
  make it unclear whether the user is viewing current or historical data.
- Keeping the current stacked Commands and Project runs sections would continue
  wasting height, especially when one section is empty, and would preserve the
  fixed-row clipping problem at minimum size.

## Project runs list

Project runs use the available overlay height instead of fixed limits. Each row
shows four scannable fields:

```text
WHEN   RESULT          TARGETS   COMMAND
16h    ok                10/10   sleep 10
17h    1 failed          16/17   for seq in 1..3; do echo ...
17h    7 cancelled       19/26   sleep 3
```

`TARGETS` is right-aligned successful targets over total targets.
`RESULT` includes text as well as semantic color, so status remains readable in
monochrome. The command receives all remaining width and truncates by terminal
cell width. Relative time remains useful for scanning; selected-run detail shows
the exact timestamp.

`/` filters runs by command using the existing fuzzy and exact-query behavior.
The tab and filter counts remain visible. Empty and no-match states explain the
next available action.

## Selected-run diagnostic

Selecting a project run exposes:

- full command;
- exact completion time and total duration;
- succeeded, failed, and cancelled counts;
- whether persisted logs remain available;
- target outcomes with relative path, status, exit code, duration, and error.

Failed and cancelled targets appear by default. `a` toggles all targets. A
successful run with this default filter shows a compact success state and tells
the user that `a` reveals every target.

`Enter` on a target opens its log viewport inside the same overlay. `Esc`
returns through a shallow stack: log viewport, target diagnostic, runs list,
then overlay close. Missing or deleted persisted logs show `logs unavailable`
without treating the history entry as corrupt.

Full command output is not duplicated into project history. Log drill-down uses
persisted files created by `--save-logs`. Without persisted logs, exit code,
duration, and captured error remain available, and the viewport explains that
`--save-logs` is required to retain output across sessions.

## Secondary actions

Diagnostic inspection is the default action:

- `Enter` inspects the selected run or target.
- `r` loads the selected run command into the main command editor and closes
  History without executing it.
- `R` starts the existing confirmed rerun-failed flow using failed targets from
  the selected historical run. No command executes without the existing
  confirmation step.
- In the `Commands` tab, `Enter` keeps the existing behavior: load the selected
  command and return focus to the command editor.

The contextual footer shows only actions valid for the active tab and depth.
History removes its duplicate in-overlay instruction row; the application
footer remains the single shortcut source.

## Responsive layout

At widths of 120 columns or more, History becomes a wide centered overlay with
the runs list on the left and selected-run diagnostic on the right. Focusing a
target and opening logs changes only the right pane, preserving the selected
run and list position.

From 80 through 119 columns, History uses one pane and drill-down navigation.
The runs list, target diagnostic, and log viewport each consume the full overlay
body. `Esc` restores the previous level and cursor position.

The existing `80x20` application minimum remains unchanged. History must fit at
that floor, compute visible rows from actual overlay height, and scroll instead
of clipping a fixed number of commands and runs. Below 80 columns, the existing
terminal-too-small screen remains authoritative.

The overlay width is no longer capped at 92 columns for wide terminals. It uses
available width with a bounded outer margin and a readable maximum suitable for
the master-detail view. Background panels render muted while History is open,
and the overlay uses one rounded border plus a surface background. No nested
decorative boxes are added.

## Persisted history model

Extend `history.RunEntry` additively with:

- run start and end times;
- a log-directory identifier when `--save-logs` persisted this run;
- target entries containing ID, relative path, final status, exit code, error,
  start time, and end time.

The log identifier is the generated run-directory basename, not an arbitrary or
absolute path. Log lookup resolves it beneath configured project log root and
continues using local-path validation for target IDs. History data must never
allow reading outside that root.

Existing JSONL entries remain readable because new fields are optional. A
legacy run still renders its summary counts and command; its detail pane says
`target details unavailable for legacy run`. Retention remains 100 project runs
and 50 global commands.

## State and rendering seams

Keep history behavior in the current Bubble Tea model, but separate pure helpers
for:

- active tab and overlay depth;
- filtered run and command collections;
- target visibility (`failures` or `all`);
- height-derived list windows and cursor clamping;
- compact row formatting by available cell width;
- secure persisted-log loading.

History state tracks independent run, command, target, and log offsets so moving
between tabs or drill-down levels restores spatial position. Opening History
resets depth to the project-runs list but preserves the existing search query;
`ctrl+u` clears it.

Persisted log reads happen through a `tea.Cmd`; key handling and rendering never
perform filesystem I/O synchronously. Loading, unavailable, empty, and read
error states render explicitly in the log viewport.

## Error handling

- Legacy run without target details remains selectable and reusable.
- Missing log directory or target log is a non-fatal unavailable state.
- Invalid stored log identifier or target ID is rejected before filesystem
  access and displayed as unavailable.
- A log read failure stays inside the viewport and does not close History.
- Empty tabs and filtered-empty results never expose actions that require a
  selected item.
- Cursor and offsets clamp after filtering, tab changes, and terminal resize.

## Testing and verification

Add history-package tests for new-field round trips, old JSONL compatibility,
retention, and rejection of unsafe log identifiers.

Add TUI component tests for:

- History opening on Project runs;
- tab switching and independent cursor restoration;
- `Enter` diagnostic drill-down and `Esc` back-stack behavior;
- failures-first target filtering and `a` all-target toggle;
- `r` loading a command without execution;
- `R` entering confirmation with historical failed targets;
- asynchronous log loading plus unavailable, empty, and error states;
- fuzzy filtering and cursor clamping;
- row budgets and widths at `140x40`, `100x30`, and `80x20`;
- legacy runs with summary-only detail;
- semantic status text and selection styling with and without color.

Verification order:

1. run focused history and TUI tests;
2. run `rtk go test ./internal/history ./internal/tui`;
3. run `rtk go test ./...`;
4. inspect wide master-detail, standard drill-down, target filter, and persisted
   log viewport in Ghostty;
5. capture final wide and minimum-size screenshots.

## Scope

No history deletion, sorting UI, mouse interaction, automatic command
execution, main-panel historical mode, output duplication in JSONL, change to
log retention, or change to the existing application minimum size. Command and
run retention limits remain unchanged.
