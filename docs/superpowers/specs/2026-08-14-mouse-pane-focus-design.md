# Mouse Pane Focus Design

## Goal

Allow mouse users to focus the `Tasks` or `Output` pane by left-clicking
anywhere inside that pane, including its border.

## User-visible behavior

- A left click inside `Tasks` changes focus to `Tasks`.
- A left click inside `Output` changes focus to `Output`.
- Clicking changes only pane focus. It does not select a task, move the task
  cursor, scroll output, or display a notice.
- Right clicks and clicks outside pane bounds do nothing.
- While an overlay is visible, pane clicks do nothing.
- Keyboard focus controls remain unchanged and fully equivalent.
- In compact or zoom mode, the sole visible pane remains focused; clicking
  cannot switch to a hidden pane.

## Implementation

Enable Bubble Tea mouse-cell motion so click events reach the model without
requiring drag motion support.

Handle left-click messages near the start of `Model.Update`, after window-size
updates and before keyboard-only dispatch. Derive pane rectangles from the same
width, height, compact-mode, zoom, header, and panel-gap calculations used by
rendering. Hit testing uses terminal cell coordinates and includes each pane's
border cells.

Keep coordinate calculation in a small package-local helper shared by mouse
handling. Rendering behavior and layout stay unchanged.

## Verification seam

Exercise the public Bubble Tea `Model.Update` seam with fixed window sizes and
synthetic mouse click messages. Verify left clicks focus both split panes,
while clicks outside panes, non-left clicks, and clicks behind overlays leave
focus unchanged. Add compact/zoom coverage proving hidden panes cannot receive
focus.

Run focused TUI tests during the red-green loop, then the full Go test suite.
Inspect the running TUI in Ghostty by clicking both pane interiors and borders,
and capture a final screenshot showing the clicked pane's focus border.

## Scope

Only mouse enablement, pane hit testing, focus state, and related tests change.
Task selection, cursor movement, scrolling, overlays, keyboard shortcuts,
layout, and styling remain unchanged.
