# Repository Guidelines

## Project Structure & Module Organization

`runny` is a Go CLI/TUI for running one shell command across selected child directories. Entry point lives in `cmd/runny/main.go`. Core behavior is split under `internal/`: `cli`, `config`, `discovery`, `runner`, `logs`, `history`, and `tui`. Shared domain types live in `internal/core`. Tests sit beside code as `*_test.go`; TUI golden fixtures live in `internal/tui/testdata/`.

## Build, Test, and Development Commands

- `rtk go test ./...`: run full test suite.
- `rtk go test ./internal/tui`: run TUI-focused tests.
- `rtk go build ./cmd/runny`: build binary.
- `go run ./cmd/runny -- pnpm test`: launch TUI with an initial command.
- `rtk docker build -t runny .`: build container image for terminal runs.

Prefix commands with `rtk`, except `go run` during testing.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs from `gofmt`, short package names, exported identifiers only when needed outside package boundaries. Keep CLI flags and config keys explicit and stable, for example `--include-hidden`, `--fail-fast`, and `include_hidden`. Preserve TUI key behavior: `?` help, `ctrl+c` clean quit, `del`/`x` cancellation. Prefer small package-local helpers.

## Testing Guidelines

Use Go `testing` package. Name tests `Test<Behavior>` and keep them beside target code. Add table tests for parser, config, discovery, and runner edge cases. For TUI changes, update model/view tests and golden fixtures in `internal/tui/testdata/` when output intentionally changes. Run `rtk go test ./...` before declaring work done.

## UI Testing Strategy

For visible TUI work, combine automated tests with real terminal inspection. Use the `ghostty-terminal-automation` skill to launch `runny`, send keys, wait for stable screens, and capture screenshots. Loop: run `go run ./cmd/runny`, resize terminal, navigate changed flow, screenshot, inspect, adjust code, repeat. Keep a final screenshot for PR evidence. Use cell inspection for color, bold, border, and truncation behavior.

## TUI Visual Styling

Keep panel contents, focused labels, and command text on the terminal's inherited default background. Communicate focus with border color, foreground accents, and bold text. Reserve explicit backgrounds or reverse video for the cursor and genuine row or text selections; treat opaque black rectangles behind labels or command text as visual regressions.

## Commit & Pull Request Guidelines

History uses Conventional Commits with scoped subjects, such as `fix(tui): adjust running status colors`, `feat(tui): emphasize focused panel border`, and `docs(skills): update tui design guidance`.

PRs should include summary, test evidence, linked issue when relevant, and screenshots or terminal captures for visible TUI changes. Note behavior changes to flags, config precedence, logging, history, or shortcuts.

## Security & Configuration Tips

Default discovery excludes hidden directories and symlinks; preserve this unless feature scope says otherwise. Logs may contain command output, so avoid committing `.runny/runs/` artifacts. Keep user config examples in YAML aligned with README precedence: `~/.runny.yaml`, `./.runny.yaml`, explicit `--config`, then CLI flags.
