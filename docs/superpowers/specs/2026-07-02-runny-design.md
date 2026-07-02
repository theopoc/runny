# runny Design

Date: 2026-07-02
Status: approved for implementation planning

## Summary

`runny` is a Go terminal UI for running one shell command across many child directories. It discovers directories from the current working directory, lets the user filter and select targets interactively, then runs the command in parallel by default. It also supports a non-interactive `--auto` mode for scripts and CI.

Project name and command name: `runny`.

## Goals

- Discover child directories from the directory where `runny` is launched.
- Run a shell command across selected directories.
- Support interactive select/deselect, select all, deselect all, filtering, and tree fold/unfold when recursive discovery is used.
- Run in parallel by default with configurable worker count.
- Support serial execution.
- Provide per-directory status and logs in the TUI.
- Provide global command history and project run history.
- Support optional log persistence.
- Use Go, Bubble Tea v2, Bubbles v2, Lip Gloss, GoReleaser, and GitHub Actions.

## Non-Goals

- Windows support in the MVP.
- Following symlinked directories in the MVP.
- Running different commands per target.
- Full plugin system or project templates.
- Remote execution.

## CLI

Primary modes:

- `runny`: open the TUI with an editable command field.
- `runny -- <command>`: open the TUI with the command prefilled.
- `runny --auto -- <command>`: run without the TUI using automatic target selection from config and flags.

Flags:

| Long flag | Short flag | Meaning |
| --- | --- | --- |
| `--auto` | `-a` | Run without TUI using automatic selection. |
| `--recursive` | `-r` | Enable recursive discovery. |
| `--depth N` | `-d N` | Discovery depth. `1` means direct children, `2+` means bounded recursion, `0` means unlimited. |
| `--include-hidden` | `-H` | Include hidden directories in discovery. |
| `--include PATTERN` | `-i PATTERN` | Keep only targets matching one or more include patterns. |
| `--exclude PATTERN` | `-e PATTERN` | Remove targets matching one or more exclude patterns. |
| `--workers N` | `-w N` | Maximum directories running at once. |
| `--serial` | `-s` | Run targets one by one. Incompatible with `--workers`. |
| `--fail-fast` | `-f` | Stop queued work and cancel active runs after first failure. |
| `--save-logs` | `-L` | Persist logs under `.runny/runs/<timestamp>/`. |
| `--disable-logging` | `-N` | Disable per-target log capture. Logging is enabled by default. |
| `--config PATH` | `-c PATH` | Load an explicit config file. |
| `--version` | `-V` | Print version and exit. |
| `--help` | `-h` | Print help and exit. |

Validation:

- `--include` and `--exclude` are mutually exclusive in the MVP.
- `--serial` and `--workers` are mutually exclusive.
- `--disable-logging` and `--save-logs` are mutually exclusive.
- `--depth` must be `>= 0`.
- `--auto` requires a command.
- `--version` exits before discovery or config validation that is unrelated to version output.

## Discovery

Default discovery:

- Starts from current working directory.
- Lists direct child directories only.
- Excludes hidden directories by default.
- Ignores symlinked directories by default.
- Selects all discovered targets initially.

Recursive discovery:

- Enabled by `--recursive` or by `--depth` greater than `1`.
- `--depth 0` means unlimited recursion.
- Results preserve hierarchy so the TUI can fold and unfold directories.
- Filtering includes matching directories and enough parent directories to keep context visible.

Pattern handling:

- Include/exclude patterns apply after discovery and hidden-directory filtering.
- Patterns match target name and relative path.
- Repeated `--include` means logical OR.
- Repeated `--exclude` means logical OR.
- Hidden directories are still ignored unless `--include-hidden` is set.

Symlinks:

- Symlinked directories are skipped in the MVP.
- This avoids cycles, duplicated work, and surprising filesystem traversal.
- Future option `--follow-symlinks` can be added after MVP if needed.

## Configuration

Config files:

- Home config: `~/.runny.yaml`
- Local config: `./.runny.yaml`
- Explicit config: `--config PATH`

Precedence:

1. Defaults
2. `~/.runny.yaml`
3. `./.runny.yaml`
4. `--config PATH` if provided
5. CLI flags

Config fields:

```yaml
depth: 1
recursive: false
include_hidden: false
include: []
exclude: []
workers: 0
serial: false
fail_fast: false
save_logs: false
disable_logging: false
```

`workers: 0` means auto workers: `min(runtime.NumCPU(), selected_target_count)`.
`disable_logging: false` means per-target log capture is enabled by default.

## Architecture

Packages:

- `cmd/runny`: CLI parsing, config loading, mode selection, version output.
- `internal/config`: config file loading, merge logic, validation.
- `internal/discovery`: directory scan, depth handling, hidden handling, symlink handling, include/exclude filtering.
- `internal/runner`: shell execution, worker pool, serial mode, cancellation, events.
- `internal/history`: global command history and project run history.
- `internal/logs`: optional in-memory log buffers and optional file persistence.
- `internal/tui`: Bubble Tea application, Bubbles components, key handling, overlays.

Data flow:

1. CLI parses flags and command after `--`.
2. Config loader merges defaults, home config, local config, explicit config, and flags.
3. Discovery produces a hierarchical target tree.
4. TUI mode lets the user edit command, filter, select targets, fold/unfold tree nodes, inspect history, and start execution.
5. Auto mode skips the TUI and runs selected targets immediately.
6. Runner emits target events and log chunks.
7. TUI or auto console renderer consumes events.
8. History records command and run summaries.

Core model:

- `RunRequest`: command, targets, execution mode, workers, fail-fast, save-logs, disable-logging.
- `Target`: relative path, absolute path, parent/children links, selected state, folded state, symlink flag.
- `TargetResult`: status, exit code, duration, log path if saved.
- `RunnerEvent`: target status changes and output chunks.

## TUI

Layout:

- Top: command field when empty or before execution.
- Left: tree table of directories.
- Tree table columns: selected marker, fold marker, directory, status.
- Right: log viewport for the focused directory.
- Bottom: compact help footer.

Main footer:

```text
space toggle · / filter · enter run · H history · ? help
```

Keybindings:

| Key | Action |
| --- | --- |
| `space` | Toggle current target selection. During a run, this also supports selecting active targets for cancellation. |
| `a` | Select all currently visible or filter-matching targets. |
| `A` | Deselect all currently visible or filter-matching targets. |
| `/` | Focus filter input. |
| `enter` | Start run, confirm overlay action, or reuse history command depending on focus. |
| `tab` | Move focus between command, tree, logs, and active overlay. |
| `right` / `l` | Unfold focused tree node. |
| `left` | Fold focused tree node. |
| `H` | Open history overlay. |
| `?` | Open help overlay. |
| `esc` | Close overlay, cancel filter, or return focus. |
| `del` | Cancel active selected targets, or focused target if none selected. |
| `R` | Re-run failed targets after confirmation. |
| `ctrl+c` | Cancel all active runs and quit TUI cleanly. |
| `q` | Quit only when no run is active and no text input is focused. |

Overlays:

- History overlay:
  - Global reusable command history.
  - Project run history with timestamp, command, selected target count, success/failure count, duration.
  - `enter` reuses selected command.
- Help overlay:
  - Full keybinding list.
- Re-run failed overlay:
  - Triggered by `R`.
  - Shows number of failed targets.
  - `enter` confirms, `esc` cancels.
  - Disabled while another run is active.

## Execution

Command execution:

- Shell: `/bin/sh -c <command>`.
- Working directory: each selected target directory.
- Parallel by default.
- Default workers: `min(runtime.NumCPU(), selected_target_count)`.
- Serial mode: one target at a time.

Statuses:

- `queued`
- `running`
- `succeeded`
- `failed`
- `cancelled`
- `skipped`

Failure behavior:

- Default: continue remaining targets.
- Final process exit code is non-zero if any target failed or was cancelled.
- `--fail-fast` cancels queued work and active runs after first failure.

Cancellation:

- Global cancellation stops all active target processes and prevents queued work from starting.
- Target cancellation via `del` stops selected active targets or the focused active target.
- Cancellation relies on Go contexts and process cleanup.

Re-run failed:

- `R` re-runs only failed targets.
- Requires confirmation.
- Resets status and logs only for targets being re-run.

## Logs

Default:

- Per-target log capture is enabled by default.
- Logs are held in memory per target unless logging is disabled.
- TUI shows logs for the focused target.
- Auto mode prefixes output by target or prints a structured summary at the end.

Disabled logging:

- `--disable-logging` / `-N` disables per-target log capture.
- `disable_logging: true` disables per-target log capture from config.
- When logging is disabled, the TUI still shows target statuses and final errors, but does not keep full stdout/stderr buffers.
- Disabled logging is useful for very noisy commands or very large target sets.

Optional persistence:

- `--save-logs` writes files under `.runny/runs/<timestamp>/`.
- Each target gets a sanitized log filename.
- Run metadata records command, start time, end time, target statuses, and log paths.
- `--save-logs` requires logging to remain enabled.

## History

Global command history:

- Stored at `~/.runny/history.jsonl`.
- Keeps reusable commands independent of a specific project.
- Used by the TUI history overlay to quickly refill the command field.

Project run history:

- Stored at `./.runny/history.jsonl`.
- Records command, timestamp, execution mode, selected target count, result counts, duration, and optional log paths.
- Used by the TUI history overlay for recent run inspection.

History should avoid unbounded growth. Initial retention can keep the latest 50 global commands and latest 100 project runs.

## README

Initial `README.md` should include:

- Project pitch.
- Install instructions.
- Homebrew install instructions:
  - `brew install --cask saewyn/tap/runny`
  - `brew tap saewyn/tap && brew trust --tap saewyn/tap && brew install --cask runny`
  - Brewfile example: `tap "saewyn/tap", trusted: true`
- TUI usage examples.
- `--auto` usage examples.
- Config file examples for `~/.runny.yaml` and `./.runny.yaml`.
- Keybindings.
- History, default log behavior, `--disable-logging`, and `--save-logs`.
- Release/install notes.

## CI and Release

GitHub Actions:

- Run on pull requests and pushes to `main`.
- Check formatting with `gofmt`.
- Run `go vet ./...`.
- Run `go test ./...`.
- Build Linux and macOS binaries.
- Run GoReleaser check.

GoReleaser:

- Release on `v*` tags.
- Build macOS and Linux binaries.
- Generate archives and checksums.
- Inject version into `runny --version` / `runny -V` via ldflags.
- Publish a Homebrew cask to the personal tap `saewyn/homebrew-tap`, exposed to users as `saewyn/tap`.
- Use GoReleaser `homebrew_casks` config because GoReleaser Homebrew formulas are deprecated for precompiled binaries.
- Configure cask name `runny`, `binaries: [runny]`, `directory: Casks`, homepage, description, license, and repository token.
- Release workflow must pass `TAP_GITHUB_TOKEN` so GoReleaser can push cask commits to the tap repository.

Homebrew tap setup:

- Create a separate GitHub repository named `homebrew-tap`.
- Add or keep Homebrew tap CI generated by `brew tap-new saewyn/tap` or equivalent `brew test-bot` workflow.
- Document tap trust for users who set `HOMEBREW_REQUIRE_TAP_TRUST`:
  - `brew trust --tap saewyn/tap`
  - `tap "saewyn/tap", trusted: true` in Brewfile.
- Track macOS signing/notarization as a release risk. MVP can ship unsigned, but signed/notarized casks are preferred for smooth macOS installs.

## Testing Strategy

Unit tests:

- Config precedence and validation.
- Discovery depth, hidden handling, symlink skipping, include/exclude behavior.
- Runner worker count, serial mode, fail-fast, cancellation, exit-code summary.
- Logging defaults, disabled logging, and `save_logs` conflict validation.
- History read/write and retention.

TUI tests:

- Bubble Tea model update tests for selection, filtering, fold/unfold, overlays, and keybindings.
- No real terminal required.

Smoke tests:

- `runny --version`
- `runny --auto -- pwd`
- `runny --auto --depth 1 --include <pattern> -- pwd`

## Open Risks

- TUI performance with very large trees depends on virtualized rendering or careful visible-row calculation.
- Process-group cleanup needs attention so cancellation does not leave child processes behind.
- Shell quoting must remain clear: command starts after `--`, and `runny` should not try to parse shell syntax itself.
- Homebrew casks for unsigned binaries can trigger macOS Gatekeeper/quarantine friction; signing/notarization may be needed before broad distribution.

## Implementation Boundary

The MVP should produce a usable `runny` repo with:

- Go module and CLI.
- TUI with selected layout and keybindings.
- Discovery, config, runner, history, logs.
- README.
- GitHub Actions.
- GoReleaser config.
- Homebrew tap publishing via GoReleaser.
- Tests and smoke verification.
