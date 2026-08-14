# Filter Input Display Design

## Goal

Keep the focused filter input visually empty until the user types, and display
only the filter text once typing starts.

## User-visible behavior

- Pressing `/` focuses the filter input but does not render `/` or a
  `<filter>` placeholder. The field contains only the cursor (`▌`).
- Typing `api` renders `api▌`, without a leading slash.
- The `/` shortcut, filtering behavior, exact-match syntax, navigation, and
  filter-related messages outside the input remain unchanged.

## Implementation

Change the focused-filter branch of `Model.commandInputValue` so it returns the
current filter followed by the cursor. No state or input-handling changes are
needed.

## Verification seam

Exercise the user-visible `Model.View()` output after focusing the filter and
after entering text. Verify that the focused input contains the cursor and
typed value, and that neither `/ <filter>` nor `/ api` appears in the input.
Run the focused TUI test during the red-green loop, then the full Go test suite.

## Scope

Only TUI rendering and its regression test change. Documentation, shortcuts,
filter semantics, history search, and empty-result messages remain unchanged.
