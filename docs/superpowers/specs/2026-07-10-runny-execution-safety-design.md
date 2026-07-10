# Runny Execution Safety Design

## Goal

Fix every accepted P1/P2 finding from the 2026-07-10 Go codebase review while preserving runny's current TUI interaction model, configuration precedence, and Unix shell-command support.

## Scope

This change covers CLI command reconstruction, explicit boolean flag precedence, explicit config-file errors, discovery pruning, subprocess and signal lifecycle, fail-fast behavior, bounded output capture, persistent-log layout and errors, Unicode backspace handling, and deterministic TUI tests. It does not add new CLI modes, change canonical keys, or modify the untracked repository-local skills checkout.

## CLI and Configuration

Arguments after `--` are treated as command arguments. Each token is POSIX-shell quoted before being joined, preserving spaces and metacharacters when the resulting command is passed to `/bin/sh -c`. Raw commands entered in the TUI or loaded from YAML remain raw shell strings. Users needing shell composition from CLI arguments can pass `sh -c` explicitly.

Boolean flags retain both value and presence so explicit forms such as `--serial=false` override YAML values. Missing implicit files (`~/.runny.yaml`, `./.runny.yaml`) remain optional; a missing path supplied through `--config` is an error.

## Execution Lifecycle

Interactive `ctrl+c` keeps the merged quit-confirmation overlay. Confirming Yes cancels all active and queued work before quitting. SIGINT received outside raw TTY input and SIGTERM do not show an overlay: they immediately cancel the shared run context, kill active process groups, wait for all runner commands to finish, then return.

The TUI continues scheduling per-target commands for progressive status updates. On the first failed result with fail-fast enabled, it cancels the shared run context, marks queued targets cancelled, and prevents additional scheduling. Already running peers are cancelled consistently with `runner.Run` fail-fast behavior. A runner-level error that returns no result becomes a failed result for the affected target, ensuring terminal state, history, and quit behavior remain consistent.

## Output and Logs

Captured output uses a bounded tail buffer capped at 4 MiB per target. When older bytes are dropped, output starts with a truncation marker. `--disable-logging` connects stdout and stderr to `io.Discard`, avoiding capture allocation entirely.

One run timestamp is created when a TUI run starts. Every target in that run writes beneath the same directory. Target IDs map to validated local nested paths, so `api/v1` becomes `api/v1.log` while `api_v1` remains `api_v1.log`; absolute paths and traversal are rejected. Parent directories are created with private permissions. A requested persistent-log write failure is surfaced as target failure instead of being discarded.

## Discovery and Input

Excluded directories are pruned before recursive descent. Backspace removes one complete UTF-8 rune from command, filter, palette, and history inputs; invalid partial UTF-8 is never produced.

## Testing and CI

Every fix starts with a focused regression test that fails for the reviewed behavior. Runner tests cover bounded/discarded output, process-group cancellation, shared log roots, safe target paths, and persistence errors. TUI tests cover fail-fast, signal cleanup coordination, runner errors, and Unicode editing. CLI/config/discovery tests cover quoting, explicit false values, explicit missing config, and excluded-tree pruning.

The Bubble Tea end-to-end test stops asserting against an asynchronous raw terminal diff stream and instead verifies the final model view or synchronized terminal state. Local verification matches CI and adds stronger gates:

- `rtk gofmt`/format check
- `rtk go vet ./...`
- `rtk go test ./...`
- `rtk go test -race ./...`
- `rtk go build ./cmd/runny`
- `rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `rtk git diff --check`

GitHub Actions `ci.yml` must remain green on the draft PR. Current `origin/main` CI is green at merge commit `6db927d`.

## Delivery

Implementation lives on `agent/harden-execution-safety`, based on current `origin/main`. Changes are split into atomic conventional commits where concerns are independently reviewable, pushed to `theopoc/runny`, and opened as one draft pull request with root-cause and verification evidence.
