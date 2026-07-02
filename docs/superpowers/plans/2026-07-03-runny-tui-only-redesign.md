# runny TUI-Only Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the approved runny redesign: TUI-only entrypoint, pug-style task/status dashboard, tofuref-style search/list/preview, and k9s-style command palette/keymap.

**Architecture:** Keep discovery, runner, logs, and history packages intact. Remove non-interactive auto execution from CLI/config/app orchestration. Expand `internal/tui` state and rendering without broad package splits so the current test suite can evolve incrementally.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lip Gloss v2, GoReleaser v2, GitHub Actions.

---

## File Structure

- Modify `internal/cli/parse.go`: remove `--auto`/`-a` and `Options.Auto`.
- Modify `internal/cli/parse_test.go`: assert `--auto` is rejected and command prefill still works.
- Modify `internal/config/config.go`: remove `auto` YAML field and flag override.
- Modify `internal/config/config_test.go`: assert strict YAML rejects `auto`.
- Modify `internal/app/app.go`: remove `runAuto`, runner import, and auto help docs.
- Modify `internal/app/app_test.go`: remove auto tests, keep version/help and add startup flag validation test.
- Modify `README.md`: document TUI-only usage and remove non-interactive Docker examples.
- Modify `Dockerfile` only if help/usage text becomes stale.
- Modify `internal/tui/model.go`: add command palette mode, preview scroll, dashboard stats, complete help rows, home/end navigation, `x` cancellation, filter clear.
- Modify `internal/tui/model_test.go`: add component tests for dashboard, command palette, keymap, preview scroll, TUI-only shortcuts.
- Modify `internal/tui/program_test.go`: extend E2E to cover `/`, `:workers 1`, `?`, `enter`, preview output.
- Modify `internal/tui/testdata/TestViewBeautifulDashboardGolden.golden`: update main dashboard golden.

## Task 1: Remove Non-Interactive Auto Mode

- [x] Write failing CLI test: `Parse([]string{"--auto"})` returns an error.
- [x] Remove `Auto` from `cli.Options` and flag registration.
- [x] Run `go test ./internal/cli`.
- [x] Write failing config test: YAML `auto: true` is unknown.
- [x] Remove `Auto` from `config.Config`, `FlagOverrides`, and flag application.
- [x] Run `go test ./internal/config`.
- [x] Remove `runAuto` from `internal/app/app.go`.
- [x] Update app tests to stop invoking `--auto`.
- [x] Run `go test ./internal/app`.

## Task 2: TUI Interaction Model

- [x] Add tests for `:` command palette open/close/filter.
- [x] Add tests for `:workers 1`, `:serial`, `:parallel`, `:run`, `:cancel`, `:history`, `:clear-filter`.
- [x] Add tests for `g`, `G`, `x`, `ctrl+u` in filter, and preview scroll keys.
- [x] Add fields to `Model`: `ShowPalette`, `Palette`, `PalettePos`, `PreviewOffset`, `LogFollow`.
- [x] Route global keys before focus-specific handlers.
- [x] Implement palette command parsing and state changes.
- [x] Implement preview scroll helpers.
- [x] Run `go test ./internal/tui -run 'TestModel.*Palette|TestModel.*Preview|TestModel.*Navigation'`.

## Task 3: Dashboard/List/Preview Render

- [x] Update golden expected layout to contain dashboard counters: queued, running, succeeded, failed, cancelled, workers, mode.
- [x] Render top dashboard under header.
- [x] Rename left pane to `Tasks`.
- [x] Render right pane as `Preview`, with path, status, command, selection state, and output/logs.
- [x] Render `/` active filter line and `:` active command palette line.
- [x] Render `?` help as full grouped keymap overlay.
- [x] Render command palette overlay with filtered commands and disabled hints.
- [x] Run `go test ./internal/tui -run TestViewBeautifulDashboardGolden`.

## Task 4: E2E And Runtime Proof

- [x] Extend `TestRunnyTUIProgramEndToEnd` to send `/`, `:workers 1`, `?`, `enter`, and verify preview output.
- [x] Keep `TestRunnyTUISmokeWithPseudoTTY` proving alternate screen startup.
- [x] Run `go test ./internal/tui -count=1`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `go run github.com/goreleaser/goreleaser/v2@latest check`.
- [x] Install binary with `go install ./cmd/runny`.
- [x] Smoke TUI help/version with `runny --version` and `runny --help`.

## Task 5: Documentation And Commit

- [x] Update README usage: TUI-only, no `--auto`.
- [x] Update Docker docs: TUI container only, no non-interactive run example.
- [x] Run final `go test ./...`.
- [ ] Commit as `feat(tui): redesign runny as tui-only`.
- [ ] Push `main`.

## Self-Review

- Spec coverage: CLI-only launches TUI, dashboard, list/preview, keymap, filter, palette, run controls, tests, and docs are covered.
- Placeholder scan: no TBD/TODO placeholders.
- Type consistency: model field names and package responsibilities match current code boundaries.
