# Mouse Wheel Pane Scrolling Design

## Goal

Add mouse-wheel navigation to the focused dashboard pane without changing existing keyboard behavior.

## Interaction

- When **Tasks** has focus, one wheel notch moves the selected visible target by one row in the wheel direction. Existing cursor visibility logic keeps the selection on screen.
- When **Output** has focus, one wheel notch scrolls command output by three lines in the wheel direction. Scrolling switches output from tail mode to manual mode, matching existing keyboard scrolling.
- Mouse-wheel input is ignored while help, history, confirmation, or command-palette overlays are open and while command or filter input has focus.
- Only vertical wheel-up and wheel-down events affect state. Other mouse events remain ignored.

Wheel behavior depends on the focused pane, not pointer coordinates.

## Implementation

Enable Bubble Tea v2 mouse reporting declaratively through `tea.View.MouseMode`. Handle `tea.MouseWheelMsg` in `Model.Update` before keyboard dispatch. Reuse `moveCursor` for Tasks and `scrollPreview` for Output so filtering, folded targets, viewport bounds, and tail-mode behavior retain their current semantics.

No new viewport component or pane hit-testing is introduced.

## Testing

Use `Model.Update` as the public behavior seam:

- wheel input moves the Tasks selection one visible target and maintains its viewport;
- wheel input scrolls Output three lines and disables tail mode;
- wheel input is ignored outside Tasks and Output interaction contexts.

Run focused TUI tests during development, full `go test ./...` once complete, then inspect both focused panes in a real Ghostty terminal.

## Documentation

Update the README shortcut table to document focus-dependent mouse-wheel behavior.
