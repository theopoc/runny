# History Diagnostics Implementation Plan

## Test seams

- `internal/history`: additive JSONL round trip through `AppendRun` and
  `ReadRuns`.
- `internal/logs`: persisted log lookup constrained beneath configured root.
- `internal/tui`: user behavior through `Model.Update` and rendered output
  through `Model.View`/history overlay rows.

## Vertical slices

1. Persist target-level run metadata and backward-compatible legacy entries.
2. Resolve persisted target logs securely from run and target identifiers.
3. Open History on Project runs with tabs, independent cursors, and dynamic
   list windows.
4. Add run diagnostic and failures-first target navigation with responsive
   wide master-detail and standard drill-down layouts.
5. Add secondary command reuse and confirmed historical failed-target rerun.
6. Add asynchronous target-log drill-down with loading and unavailable states.
7. Update footer/help, replace obsolete history tests, run full verification,
   then inspect wide and minimum layouts in Ghostty.

Each slice follows one failing behavior test, minimal implementation, then the
next slice. Existing command history, current-run rerun, and overlay behavior
stay green throughout.
