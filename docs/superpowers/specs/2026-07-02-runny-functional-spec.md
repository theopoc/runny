# runny Functional Spec

Date: 2026-07-03
Status: approved functional scope

## Summary

`runny` is a Go full-screen TUI for running one shell command across selected child directories. The CLI only launches the TUI and may prefill startup state. It does not provide non-interactive execution mode.

Project name and command name: `runny`.

## Goals

- Launch only as a TUI.
- Discover directories from the directory where `runny` starts.
- Exclude hidden directories by default.
- Skip symlinked directories by default.
- Support recursive discovery and bounded depth.
- Support fold/unfold when recursive discovery shows hierarchy.
- Let the user filter/search directories with `/`.
- Let the user select/deselect one, visible set, or all matching targets.
- Run one command across selected directories.
- Run in parallel by default with visible worker count.
- Support serial execution and worker count changes.
- Show task/status summary.
- Show directory/task list and focused target logs/details.
- Show complete keymap overlay on `?`.
- Support command palette on `:` for discoverable actions.
- Support command/run history inside the TUI.
- Support cancelling selected running or queued tasks.
- Support global cancellation with `ctrl+c`.
- Support rerun failed with confirmation.
- Support optional log persistence.
- Use Go, Bubble Tea v2, Lip Gloss, GoReleaser, GitHub Actions, Docker, and Homebrew cask publishing.

## Non-Goals

- Non-interactive CLI execution.
- `--auto` or any flag that runs work without opening the TUI.
- Remote execution.
- Different commands per target.
- Plugin system.
- Following symlinked directories.
- Windows support in the MVP.

## CLI Contract

The CLI is only an entrypoint into the TUI.

Supported startup commands:

```bash
runny
runny -- pnpm test
runny --config .runny.yaml
runny --depth 2 --workers 4 -- pnpm test
```

Behavior:

- `runny` opens the full TUI with an empty command prompt.
- `runny -- <command>` opens the TUI with command prefilled.
- CLI flags configure initial TUI state only.
- `runny --help` and `runny --version` may print and exit because they do not execute project commands.
- There is no `--auto`.

Startup flags:

| Long flag | Short flag | Meaning |
| --- | --- | --- |
| `--config PATH` | `-c PATH` | Load explicit config file. |
| `--recursive` | `-r` | Enable recursive discovery. |
| `--depth N` | `-d N` | Discovery depth. `1` direct children, `2+` bounded recursion, `0` unlimited. |
| `--include-hidden` | `-H` | Include hidden directories in discovery. |
| `--include PATTERN` | `-i PATTERN` | Start with only matching directories visible/selectable. |
| `--exclude PATTERN` | `-e PATTERN` | Start with matching directories removed. |
| `--workers N` | `-w N` | Initial max parallel tasks. |
| `--serial` | `-s` | Initial mode: one target at a time. |
| `--fail-fast` | `-f` | Initial fail-fast setting. |
| `--save-logs` | `-L` | Persist logs under `.runny/runs/<timestamp>/`. |
| `--disable-logging` | `-N` | Disable per-target log capture. |
| `--version` | `-v` | Print version and exit. |
| `--help` | `-h` | Print help and exit. |

Validation:

- `--include` and `--exclude` are mutually exclusive.
- `--serial` and `--workers` are mutually exclusive.
- `--disable-logging` and `--save-logs` are mutually exclusive.
- `--depth` must be `>= 0`.
- Startup command after `--` is optional.

## Configuration

Config files:

- Home config: `~/.runny.yaml`
- Local config: `./.runny.yaml`
- Explicit config: `--config PATH`

Precedence:

1. Defaults
2. `~/.runny.yaml`
3. `./.runny.yaml`
4. `--config PATH`
5. CLI startup flags

Config fields:

```yaml
command: ""
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

## Discovery

Default:

- Start from current working directory.
- List direct child directories only.
- Exclude hidden directories by default.
- Skip symlinked directories by default.
- Select all discovered targets initially.

Recursive:

- Enabled by `--recursive`, config, or depth greater than `1`.
- `depth: 0` means unlimited recursion.
- Parent/child relationships are preserved.
- Filtered descendants keep parent context visible.
- Folded parents hide descendants from selection commands that target visible rows.

Pattern handling:

- Include/exclude patterns apply after hidden and symlink filtering.
- Patterns match relative path.
- Repeated include patterns are OR.
- Repeated exclude patterns are OR.
- Hidden directories stay excluded unless `include_hidden: true`.

## TUI Functional Areas

The TUI must expose:

- Current command.
- Selected target count.
- Status counts: idle, queued, running, succeeded, failed, cancelled, skipped.
- Execution mode and worker count.
- Searchable directory/task list.
- Focused target details: path, status, selected state, command, output/logs.
- Persistent shortcuts for common actions.
- Complete keymap overlay.
- Command palette.
- Filter/search input.

## Navigation Model

Global keys:

| Key | Action |
| --- | --- |
| `?` | Open complete keymap overlay. |
| `/` | Enter filter/search mode. |
| `:` | Open command palette. |
| `tab` | Cycle panes or overlay fields. |
| `esc` | Close overlay, leave filter/command mode, return to main table. |
| `q` | Quit only when no run is active and no input mode is active. |
| `ctrl+c` | Cancel all active/queued work and quit cleanly. |

Table keys:

| Key | Action |
| --- | --- |
| `up`, `k` | Move cursor up. |
| `down`, `j` | Move cursor down. |
| `home`, `g` | Move to first visible row. |
| `end`, `G` | Move to last visible row. |
| `space` | Toggle selected target. |
| `a` | Select visible targets. |
| `A` | Deselect visible targets. |
| `right`, `l` | Unfold focused directory. |
| `left`, `h` | Fold focused directory. |
| `enter` | Run selected targets when idle. |
| `del`, `x` | Cancel selected running or queued targets. |
| `R` | Rerun failed targets with confirmation. |

Output/log keys:

| Key | Action |
| --- | --- |
| `pageup`, `ctrl+b` | Scroll focused target output up. |
| `pagedown`, `ctrl+f` | Scroll focused target output down. |
| `ctrl+u` | Half-page focused target output up. |
| `ctrl+d` | Half-page focused target output down. |
| `L` | Toggle log follow mode. |

## Filter/Search

- `/` opens filter mode.
- Filter input stays visible while active.
- Typing filters visible directory/task rows live.
- Matching descendants keep parent context visible.
- Filter supports substring first.
- Fuzzy matching can be added later after exact behavior is stable.
- `esc` exits filter mode and keeps current filter.
- `ctrl+u` clears filter.
- Empty filter shows full tree respecting fold state.

## Command Palette

- `:` opens command palette.
- Palette lists available commands, filters as user types, and executes on `enter`.
- Commands are discoverable aliases for key actions.

Initial commands:

| Command | Action |
| --- | --- |
| `:run` | Run selected targets. |
| `:failed` | Select failed targets. |
| `:rerun-failed` | Confirm and rerun failed targets. |
| `:cancel` | Cancel selected running/queued targets. |
| `:cancel-all` | Cancel all active/queued work. |
| `:workers N` | Set worker count. |
| `:serial` | Switch to serial mode. |
| `:parallel` | Switch to parallel mode. |
| `:logs` | Focus target output/logs. |
| `:history` | Open history overlay. |
| `:clear-filter` | Clear filter. |

## Help / Keymap Overlay

- `?` opens complete keymap overlay.
- Overlay groups commands by context: global, table, filter, command palette, target output/logs, run control.
- Current context appears first.
- Disabled commands show muted reason.
- `esc`, `?`, or `q` closes overlay.
- Overlay must fit small terminals by scrolling.

## Execution Model

Execution stays inside TUI:

- User starts run with `enter` or `:run`.
- Command runs with `/bin/sh -c <command>`.
- Working directory is target directory.
- Parallel mode is default.
- Worker count controls max active targets.
- Serial mode means worker count 1.
- Queue is visible in status summary.
- Completed targets keep final status and logs.
- Rerun failed resets only failed target statuses/logs.

Statuses:

- `idle`
- `queued`
- `running`
- `succeeded`
- `failed`
- `cancelled`
- `skipped`

Failure:

- Default: continue remaining targets.
- Fail-fast: cancel queued work and active runs after first failure.

Cancellation:

- `del` or `x`: cancel selected running/queued targets, or focused running/queued target if none selected.
- `:cancel-all`: cancel all active/queued work.
- `ctrl+c`: cancel all and quit cleanly.

## History

History is a TUI feature:

- Command history from `~/.runny/history.jsonl`.
- Project run history from `./.runny/history.jsonl`.
- `H` or `:history` opens overlay.
- User can reuse command, inspect run summary, or select failed run targets if available.

## Logs

Default:

- Per-target log capture enabled.
- Focused target log is visible.
- Log follow mode enabled for running focused target unless user scrolls.

Persistence:

- `save_logs: true` writes logs under `.runny/runs/<timestamp>/<target>.log`.
- `disable_logging: true` disables capture and target output display.

## Architecture

Packages:

- `cmd/runny`: TUI entrypoint, startup flag parsing, version/help output.
- `internal/config`: config loading, merge, validation.
- `internal/discovery`: directory scan, depth, hidden handling, symlink skipping, include/exclude.
- `internal/runner`: shell execution, worker scheduling, serial mode, cancellation, process cleanup.
- `internal/history`: command history and project run history.
- `internal/logs`: per-target log buffers and optional persistence.
- `internal/tui`: Bubble Tea model, update routing, view rendering, overlays, command palette, filter.

Key implementation boundary:

- `internal/runner` remains UI-agnostic.
- `internal/tui` owns scheduling presentation, keyboard routing, overlays, and visible state.
- Long-running shell work runs through Bubble Tea commands/messages, never directly inside `Update`.

## Testing

Use layered TUI tests:

- Unit tests for config, discovery, runner, logs, history.
- Component tests for TUI key handling and state transitions.
- Golden tests for main TUI states, help overlay, command palette, filter mode, history overlay.
- E2E test with Bubble Tea program input covering filter, selection, run, target output update, help, command palette.
- Pseudo-TTY smoke test proving alternate screen and real binary startup.
- Runtime smoke with real directories proving command execution remains TUI-driven.

Verification commands:

```bash
go test ./...
go vet ./...
go run github.com/goreleaser/goreleaser/v2@latest check
go test ./internal/tui -run 'TestRunnyTUIProgramEndToEnd|TestRunnyTUISmokeWithPseudoTTY' -count=1
```

Manual verification:

```bash
mkdir -p /tmp/runny-test/api /tmp/runny-test/web
cd /tmp/runny-test
runny -- printf ok
```

Inside TUI:

1. `/api` filters list.
2. `esc` returns to table.
3. `space` toggles focused target.
4. `a` selects visible targets.
5. `?` opens complete keymap.
6. `:` opens command palette.
7. `:workers 1` sets worker count.
8. `enter` or `:run` runs selected targets.
9. Focused logs are visible.
10. `ctrl+c` cancels active work and exits cleanly.

## Migration From Current Build

Required spec changes:

- Remove `--auto` from docs, parser, config, tests, and app flow.
- Remove auto-mode code path from `internal/app`.
- Treat all CLI flags as startup config only.
- Keep `--help` and `--version`.
- Rework TUI rendering around status summary, task list, and focused target logs/details.
- Add command palette mode.
- Expand help overlay to full keymap.
- Add target output scroll state.
- Add golden files for primary functional states.

## Open Follow-Up

After this spec, implementation planning should split work into:

1. CLI contract cleanup: remove non-interactive execution.
2. TUI model refactor: modes, palette, filter, target output scrolling.
3. TUI render refactor: status summary, task list, focused target logs/details, overlays.
4. Runner integration: keep worker scheduling visible and cancellable.
5. Tests and docs: golden, E2E, README update, release checks.
