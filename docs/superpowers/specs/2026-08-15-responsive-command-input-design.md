# Responsive Command Input Design

## Goal

Keep long commands readable while they are edited without permanently reducing
space available to task and output panels.

## User-visible behavior

- When command input has focus, its content wraps to terminal cell width and box
  grows to fit wrapped lines.
- Growth stops before task and output panels become shorter than their existing
  10-row minimum.
- If wrapped content exceeds available input height, a vertical viewport keeps
  cursor row visible. Up and down indicators show hidden content.
- When focus leaves command input, box collapses to existing one-content-row
  shape and command is truncated with an ellipsis.
- Returning focus expands box again around current cursor position.
- Terminal resize immediately recomputes wrapping, input height, panel height,
  cursor viewport, and mouse hit regions.
- Command remains a logical single line. Paste normalization and execution
  behavior do not change.

At minimum supported terminal size, `80x20`, input remains one content row so
panels retain their current minimum. Taller terminals expose progressively more
wrapped command rows.

## Rendering design

Keep current custom command editor rather than replacing it with a multiline
text component. Existing editor owns selection, clipboard, cursor movement,
word movement, command history, and single-line shell-command semantics.

Split command into visual rows using terminal cell widths from
`ansi.StringWidth`. Wrapping is display-only: it must not insert newline
characters into `Model.Command`. Cursor and selection positions remain rune
indexes and are projected onto wrapped visual rows during rendering.

Focused command input calculates:

1. content width from box width and borders;
2. required visual rows from command content and cursor cell;
3. maximum visible rows from terminal height after reserving fixed chrome and
   existing 10-row panel minimum;
4. visible row window containing cursor when required rows exceed maximum.

Unfocused command input continues using one truncated content row. Palette and
filter rendering keep current single-row behavior.

## Layout integration

Derive command-box height and panel height from shared layout metrics. Rendering,
panel mouse hit-testing, directory viewport sizing, and overlay placement must
consume those same metrics so expanded input cannot desynchronize visible panel
positions from interactive regions.

Extra focused command rows reduce panel height one-for-one. Panel height never
drops below 10. Existing compact-width behavior and `80x20` too-small boundary
remain unchanged.

## Edge cases

- Empty command still renders cursor cell on one row.
- A command ending exactly at right edge reserves a visible cursor cell and
  wraps cursor to next row.
- Wide Unicode graphemes use display-cell width, not byte or rune count.
- Selection styling may span wrapped rows without changing selected text.
- Moving cursor, selecting, pasting, or recalling history updates vertical
  viewport immediately.
- At vertical cap, indicators distinguish content above, below, or both.

## Regression and verification seams

Create a fast component-level repro using a long command with focused input at
`80x24`. Assert that multiple command fragments and cursor are visible on
separate rows; this catches original one-line horizontal-window symptom.

Add focused tests for:

- collapse to one content row outside command focus;
- expand again around preserved cursor when focus returns;
- wide-to-narrow-to-wide resize behavior;
- panel 10-row minimum and cursor-following vertical viewport at cap;
- correct overflow indicators;
- wide Unicode wrapping and boundary cursor placement;
- panel mouse hit-testing after input height changes.

Keep existing command editing, selection, clipboard, history, palette, filter,
golden, and PTY tests green. Verification order:

1. run targeted red-capable regression test before and after fix;
2. run `rtk go test ./internal/tui`;
3. run `rtk go test ./...`;
4. inspect focused and unfocused states in Ghostty at multiple terminal sizes and
   retain final screenshot as visual evidence.

## Scope

No changes to command value semantics, execution, shell parsing, history,
clipboard shortcuts, selection behavior, filter input, command palette,
minimum supported terminal size, or non-TUI interfaces.
