# Repository Guidelines

## Project Structure & Module Organization

`runny` is a Go CLI/TUI for running one shell command across selected child directories. Entry point lives in `cmd/runny/main.go`. Core behavior is split under `internal/`: `app`, `cli`, `config`, `discovery`, `run`, `runner`, `logs`, `history`, and `tui`. Shared domain types live in `internal/core`. Tests sit beside code as `*_test.go`; TUI golden fixtures live in `internal/tui/testdata/`.

## Development Flow

Carry implementation and documentation tasks through to a pull request with passing CI, unless the user requests a different stopping point.

1. Inspect the working tree, applicable instructions, relevant files, and `.github/workflows/ci.yml`. Preserve existing user changes. Fetch `origin` and create a dedicated branch from `origin/main` for new work; reuse the existing branch when continuing a task or PR.
2. Make focused changes and update affected tests and documentation. Use the `find-docs` skill when library documentation or examples are needed.
3. Run focused tests during development, then `rtk go test ./...` and `rtk go build ./cmd/runny` before committing. Run the additional checks from the CI workflow relevant to the change; report any local check that could not run.
4. For visible TUI changes, complete the Ghostty validation and screenshot workflow below. Finish terminal cleanup as soon as testing ends.
5. Review the final diff, use the `commit` skill for a scoped Conventional Commit, push the branch, and create or update its PR using `gh` and `.github/PULL_REQUEST_TEMPLATE.md`. Follow the evidence requirements below.
6. Wait for CI with `rtk gh pr checks <number> --watch --interval 10`. Inspect failing logs, fix failures caused by the change, rerun affected local checks, and push corrections. Repeat until all applicable checks pass on the latest PR commit. Pending checks, missing checks, and cancelled runs do not establish success; report external blockers explicitly when they prevent completion.
7. Return the PR link, validation results, confirmed CI status, and screenshots for visible TUI changes. Leave merging to the user unless explicitly requested.

## Build, Test, and Development Commands

- `rtk go test ./...`: run full test suite.
- `rtk go test ./internal/tui`: run TUI-focused tests.
- `rtk go test -race ./...`: check for data races.
- `rtk go vet ./...`: run static analysis.
- `rtk go build ./cmd/runny`: build binary.
- `rtk goreleaser check`: validate release configuration.
- `go run ./cmd/runny -- pnpm test`: launch TUI with an initial command.

Prefix commands with `rtk`, except `go run` during testing.

Distribute `runny` through native Go binaries, GitHub release archives, and Homebrew. Keep development, CI, and releases independent of container tooling. See `docs/maintainers/releases.md` for publication and recovery.

## Coding Style & Naming Conventions

Use standard Go formatting: tabs from `gofmt`, short package names, exported identifiers only when needed outside package boundaries. Keep CLI flags and config keys explicit and stable, for example `--include-hidden`, `--fail-fast`, and `include_hidden`. Preserve TUI key behavior: `?` help, `ctrl+c` clean quit, `del`/`x` cancellation. Prefer small package-local helpers.

## Testing Guidelines

Use Go `testing` package. Name tests `Test<Behavior>` and keep them beside target code. Add table tests for parser, config, discovery, and runner edge cases. For TUI changes, update model/view tests and golden fixtures in `internal/tui/testdata/` when output intentionally changes. Complete the validation gates in Development Flow before declaring work done.

## UI Testing Strategy

For visible TUI work, combine automated tests with real terminal inspection using the `ghostty-terminal-automation` skill:

1. List existing terminals, create a dedicated test window or tab, and record every terminal ID created for the task. Keep pre-existing user terminals untouched.
2. Launch `go run ./cmd/runny` from the working branch, exercise the changed flow, and wait for a stable screen. Inspect both normal and narrow layouts, including cell data for color, bold, borders, and truncation. Adjust and repeat until the changed flow renders correctly.
3. Capture and visually inspect final screenshots of the affected states before closing the terminals. Store them outside `.runny/runs/`, record terminal dimensions and the scenario, and preserve the files for PR evidence. Embed at least one final screenshot in the user-facing response using its absolute local path.
4. As soon as testing finishes, quit `runny` cleanly with `ctrl+c`, stop any remaining task-owned test processes, and close every Ghostty window or tab created for the tests. Perform this cleanup on success, failure, and interruption; list terminals again to verify the recorded test IDs are gone. Keep screenshot files available after cleanup.

For documentation-only or nonvisual changes, screenshots are optional unless the user requests them. If Ghostty or screenshot capture is unavailable, report the missing visual validation explicitly.

## TUI Visual Styling

Render all TUI chrome, panel contents, focused labels, command text, status messages, overlays, and footer hints with foreground-only ANSI styling on the terminal's inherited default background. Communicate focus with border color, foreground accents, and bold text. Reserve explicit backgrounds or reverse video for cursor cells and genuine row or text selections. Keep a rendered-ANSI regression test that rejects background-color SGR sequences on every non-selection path, then inspect normal and narrow layouts in Ghostty; any opaque rectangle behind ordinary text is a release-blocking visual regression.

## Commit & Pull Request Guidelines

History uses Conventional Commits with scoped subjects, such as `fix(tui): adjust running status colors`, `feat(tui): emphasize focused panel border`, and `docs(skills): update tui design guidance`.

PRs should include a concise summary, commands run and their results, linked issue when relevant, and screenshots for visible TUI changes. For each screenshot, describe the scenario and terminal dimensions, and confirm test-terminal cleanup. Attach screenshots through a supported upload mechanism or use a repository-accessible image URL; a local filesystem path is not viewable by GitHub reviewers. If uploading is unavailable, state that limitation in the PR and show the screenshot in the final response. Note behavior changes to flags, config precedence, logging, history, or shortcuts.

## Security & Configuration Tips

Default discovery excludes hidden directories and symlinks; preserve this unless feature scope says otherwise. Logs may contain command output, so avoid committing `.runny/runs/` artifacts. Keep user config examples in YAML aligned with README precedence: `~/.runny.yaml`, `./.runny.yaml`, explicit `--config`, then CLI flags.
