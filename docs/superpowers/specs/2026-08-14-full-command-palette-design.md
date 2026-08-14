# Full Command Palette Design

## Goal

Display every matching command in command palette. Remove truncated-list message such as `4 more command(s)`.

## Design

`Model.paletteRows` will render every entry returned by `filteredPaletteCommands` instead of stopping after eight rows. Existing ordering, selection marker, fuzzy highlighting, descriptions, input guidance, and keyboard navigation stay unchanged.

Palette overlay already derives its height from rendered rows. Removing row cap therefore makes popup grow vertically to contain complete result set at normal supported terminal sizes. No multi-column layout or internal scrolling will be introduced.

## Responsive Behavior

At standard `80x24` floor, current twelve-command set fits vertically. Narrow width keeps existing truncation and overlay-sizing behavior; this change affects row count only. If future command growth exceeds available terminal height, dedicated scrolling can be designed separately rather than silently hiding commands now.

## Test Seams

- `Model.paletteRows`: empty query renders all commands and never renders `more command(s)`.
- `Model.View`: command palette overlay contains complete command set at standard terminal size.
- Existing filtered, fuzzy-highlighted, no-match, selection, and keyboard tests remain green.

## Scope

No command definitions, ordering, filtering, keybindings, execution behavior, or other overlays change.
