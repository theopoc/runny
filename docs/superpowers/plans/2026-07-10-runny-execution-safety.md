# Runny Execution Safety Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix every accepted execution-safety review finding and keep local plus GitHub CI fully green.

**Architecture:** Preserve the current TUI scheduler and shell-backed runner, but make ownership explicit. CLI parsing preserves argv boundaries, the TUI owns one run context and log root, runner output is bounded, and `tui.Run` owns OS signals plus completion waiting.

**Tech Stack:** Go 1.26.5, standard `flag`, `context`, `os/signal`, `sync`, `/bin/sh`, Bubble Tea v2.0.7, Go testing, GitHub Actions.

## Global Constraints

- Preserve canonical keys: `?` help, `ctrl+c` quit confirmation, `del`/`x` cancellation.
- Hidden directories and symlinks remain excluded by default.
- Commands entered in TUI or YAML remain raw `/bin/sh -c` strings.
- CLI arguments after `--` preserve argument boundaries through POSIX quoting.
- Captured output retains at most 4 MiB per target and starts with `[runny: output truncated]\n` after truncation.
- `--disable-logging` performs no output capture.
- One `.runny/runs/<timestamp>/` directory owns every target log from one TUI run.
- Persistent target paths must remain beneath the run root and use private directory/file modes.
- Fail-fast cancels active peers and queued targets after first failure.
- OS SIGINT/SIGTERM cancels active work and waits for command cleanup without showing an overlay.
- Existing untracked `.agents/skills/cc-skills-golang/` is never staged.

---

### Task 1: CLI, config, and discovery contracts

**Files:**
- Modify: `internal/cli/parse.go`
- Modify: `internal/cli/parse_test.go`
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/discovery/discovery.go`
- Modify: `internal/discovery/discovery_test.go`
- Modify: `README.md`

**Interfaces:**
- Produces: `shellJoin(args []string) string` and `shellQuote(value string) string` in package `cli`.
- Produces: `*Set` presence fields on `cli.Options` for recursive, include-hidden, serial, fail-fast, save-logs, and disable-logging.
- Produces: `mergeFile(cfg *Config, path string, required bool) error` in package `config`.

- [ ] **Step 1: Write failing CLI regression tests**

Add `TestParsePreservesCommandArgumentBoundaries` with arguments `printf`, `%s\n`, and `hello; touch /tmp/runny-should-not-exist`; assert parsed command equals `printf '%s\n' 'hello; touch /tmp/runny-should-not-exist'`. Add table test `TestParseTracksExplicitFalseBooleanFlags` covering every long boolean flag and asserting value `false` plus its `Set` field `true`.

- [ ] **Step 2: Verify CLI tests fail**

Run: `rtk go test ./internal/cli -run 'TestParsePreserves|TestParseTracks' -count=1`

Expected: FAIL because command tokens are joined without quoting and flag presence is absent.

- [ ] **Step 3: Implement POSIX quoting and flag presence**

Use this quoting contract:

```go
func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_@%+=:,./-", r)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
```

After `fs.Parse`, use `fs.Visit` to set canonical presence fields for both long and short aliases. Set `opts.Command = shellJoin(fs.Args())`.

- [ ] **Step 4: Write and verify config-precedence regressions**

Add an app/config test where YAML has `serial: true`, CLI parses `--serial=false`, and `flagOverrides` produces a non-nil false pointer. Add `TestLoadRejectsMissingExplicitConfig` while retaining optional missing home/local behavior.

Run: `rtk go test ./internal/app ./internal/config -run 'Test.*(ExplicitFalse|MissingExplicit)' -count=1`

Expected: FAIL before implementation.

- [ ] **Step 5: Implement explicit false and required config behavior**

Replace `boolPtr(v bool)` use with `boolPtrIfSet(value, set bool) *bool`. Call `mergeFile(..., false)` for implicit files and `mergeFile(..., true)` for `LoadOptions.Config`. Wrap explicit-path errors with path context.

- [ ] **Step 6: Write and verify excluded-tree pruning regression**

Create an excluded directory with an unreadable descendant, restore permissions in cleanup, and assert unlimited discovery succeeds without targets from that subtree.

Run: `rtk go test ./internal/discovery -run TestDiscoverPrunesExcludedDirectories -count=1`

Expected: FAIL because current walker descends into excluded directories.

- [ ] **Step 7: Prune excluded directories and document CLI shell semantics**

Compute `excluded := matches(rel, opts.Exclude)` before target construction and `continue` before recursion when true. Update README examples to show normal argv quoting and `sh -c` for explicit shell composition.

- [ ] **Step 8: Verify Task 1**

Run: `rtk go test ./internal/cli ./internal/app ./internal/config ./internal/discovery -count=1`

Expected: all tests pass.

### Task 2: Bounded output and safe persistent logs

**Files:**
- Create: `internal/runner/tail_buffer.go`
- Create: `internal/runner/tail_buffer_test.go`
- Modify: `internal/runner/runner.go`
- Modify: `internal/runner/runner_test.go`
- Modify: `internal/logs/store.go`
- Modify: `internal/logs/store_test.go`
- Modify: `internal/core/types.go`

**Interfaces:**
- Produces: `newTailBuffer(limit int) *tailBuffer`, implementing `io.Writer` and `String() string`.
- Produces: `const maxOutputBytes = 4 << 20` and `const truncatedOutputMarker = "[runny: output truncated]\n"`.
- Changes: `core.RunRequest.LogRoot` means already-scoped run directory; runner never appends another timestamp.

- [ ] **Step 1: Write failing tail-buffer tests**

Test output below limit, output over limit retaining newest bytes, repeated writes, marker insertion, and concurrent writes under `-race`.

Run: `rtk go test ./internal/runner -run TestTailBuffer -count=1`

Expected: FAIL because `tailBuffer` does not exist.

- [ ] **Step 2: Implement synchronized bounded tail buffer**

Implement `tailBuffer` with `sync.Mutex`, fixed limit, and a single retained byte slice. A write larger than the limit keeps its final `limit` bytes. `String` returns the marker plus retained tail when truncation occurred.

- [ ] **Step 3: Write failing disable-logging and persistence-error tests**

Add runner tests proving a command emitting more than 4 MiB returns bounded marked output, `DisableLogging` returns empty output without a capture buffer, and a log write failure produces `StatusFailed` with `saving log:` context.

Run: `rtk go test ./internal/runner -run 'TestRun(BoundsOutput|DisablesCapture|SurfacesLogWriteFailure)' -count=1`

Expected: FAIL against unbounded `bytes.Buffer` and discarded `Store.Append` errors.

- [ ] **Step 4: Integrate output policy and persistence errors**

Select `io.Discard` when logging is disabled; otherwise use `newTailBuffer(maxOutputBytes)`. Call `Store.Append` once after completion and preserve its error. If command and persistence both fail, join errors; if only persistence fails, mark result failed.

- [ ] **Step 5: Write failing safe-path tests**

Assert `api/v1` and `api_v1` create distinct files, nested parent directories use mode `0700`, files use mode `0600`, and `../escape`, absolute paths, and empty IDs return errors without writes outside root.

Run: `rtk go test ./internal/logs -run 'TestStore(NestedTargetPaths|RejectsUnsafeTargetIDs|UsesPrivateModes)' -count=1`

Expected: FAIL against filename sanitization and permissive modes.

- [ ] **Step 6: Implement safe nested target paths**

Convert slash IDs with `filepath.FromSlash`, require `filepath.IsLocal`, reject `.` and empty values, create parent directories with `0700`, and open log files with `0600`. Keep the store mutex around buffer and file operations.

- [ ] **Step 7: Remove runner-generated timestamps**

Delete timestamp joining from `runner.Run`; use `req.LogRoot` directly. Update runner log tests to assert multiple selected targets write inside one supplied run root.

- [ ] **Step 8: Verify Task 2**

Run: `rtk go test -race ./internal/logs ./internal/runner -count=1`

Expected: all tests pass with no race reports.

### Task 3: TUI lifecycle, fail-fast, errors, and Unicode

**Files:**
- Modify: `internal/tui/view.go`
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/model_test.go`
- Modify: `internal/tui/program_test.go`
- Modify: `internal/tui/pty_test.go`

**Interfaces:**
- Produces: private `shutdownMsg` handled by `Model.Update`.
- Produces: `runProgram(ctx context.Context, opts Options) error`; public `Run` owns `signal.NotifyContext`.
- Adds: `runWait *sync.WaitGroup` and `runLogRoot string` to TUI-owned state.
- Produces: `deleteLastRune(value string) string`.

- [ ] **Step 1: Write failing runner-error and fail-fast model tests**

Add tests where `runFunc` returns an error with zero results and where first of three targets fails under worker limit 1. Assert failed target reaches terminal failed state, queued targets become cancelled, queue clears, no next command schedules, history totals remain correct, and normal quit is no longer blocked.

Run: `rtk go test ./internal/tui -run 'TestModel(RunnerErrorBecomesFailure|FailFastCancelsRemaining)' -count=1`

Expected: FAIL against current `applyRunDone` behavior.

- [ ] **Step 2: Implement terminal runner errors and fail-fast cancellation**

When `done.err != nil && len(done.results) == 0`, synthesize a result for `done.targetID`. After recording a failed result with `FailFast`, cancel the root context, cancel active target contexts, mark queued targets cancelled, decrement pending counts exactly once, clear the queue, and never call `startQueuedRuns` for cancelled work.

- [ ] **Step 3: Write failing one-run-root and Unicode tests**

Assert `startRun` creates one timestamped `runLogRoot` reused by all per-target requests. Add table tests for command, filter, palette, and history backspace using `é` and emoji; assert valid UTF-8 and complete-rune removal.

Run: `rtk go test ./internal/tui -run 'TestModel(UsesOneLogRootPerRun|BackspaceRemovesRune)' -count=1`

Expected: FAIL before implementation.

- [ ] **Step 4: Implement run-scoped log root and rune deletion**

Set `m.runLogRoot = filepath.Join(m.LogRoot, time.Now().UTC().Format("20060102T150405.000000000Z"))` once in `startRun` only when persistence is enabled. Pass it through every `RunRequest`. Implement rune deletion with `utf8.DecodeLastRuneInString` and use it at all four backspace sites.

- [ ] **Step 5: Write failing signal-cleanup test**

Use a cancellable context with `runProgram`, a blocking `runFunc`, and a wait channel. Cancel context and assert program cancels the active run, waits for `runFunc` completion, and returns within the test timeout. Keep interactive `ctrl+c` confirmation tests unchanged.

Run: `rtk go test ./internal/tui -run TestRunProgramWaitsForSignalCleanup -count=1`

Expected: FAIL because `runProgram` and wait ownership do not exist.

- [ ] **Step 6: Implement app-owned signal and command waiting**

Public `Run` creates `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)`, defers stop, and calls `runProgram`. `runProgram` creates a `sync.WaitGroup`, installs it in model options, starts Bubble Tea with `tea.WithoutSignals()`, sends `shutdownMsg` when context ends, and waits after `Program.Run`. Each target command calls `Add(1)` before returning the `tea.Cmd` and `Done()` on exit. `shutdownMsg` calls `cancelAll` and returns `tea.Quit` without confirmation.

- [ ] **Step 7: Stabilize end-to-end rendering assertion**

Remove fixed `time.Sleep(100 * time.Millisecond)` as synchronization. Keep final model assertions, then assert rendered content through `final.View().Content` or an existing terminal-cell helper rather than raw incremental escape output.

- [ ] **Step 8: Verify Task 3**

Run: `rtk go test -race ./internal/tui -count=10`

Expected: all iterations pass with no race reports or leaked child processes.

### Task 4: CI gates, full verification, and delivery

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `README.md` if behavior text needs final alignment
- Modify: `docs/superpowers/plans/2026-07-10-runny-execution-safety.md` checkboxes only as tasks complete

**Interfaces:**
- Produces: GitHub CI race gate `go test -race ./...`.

- [ ] **Step 1: Add race detector to CI**

Add `- run: go test -race ./...` after the normal test step in `.github/workflows/ci.yml`. Keep vet, normal tests, build, and GoReleaser checks intact.

- [ ] **Step 2: Run formatting and repository checks**

Run:

```text
rtk gofmt -w <changed-go-files>
rtk git diff --check
rtk go vet ./...
rtk go test ./...
rtk go test -race ./...
rtk go build ./cmd/runny
rtk go mod verify
rtk go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

Expected: every command exits 0; vulnerability scan reports zero reachable vulnerabilities.

- [ ] **Step 3: Run runtime subprocess smoke**

Build a temporary binary outside the repository. Launch a long-running command in a PTY, confirm quit, and verify its process group no longer exists. Repeat with SIGTERM and verify no child remains with PPID 1.

- [ ] **Step 4: Review full branch**

Generate one review package from merge base `6db927d` to HEAD. Review exact design compliance, command-injection boundaries, path containment, cancellation accounting, races, and test determinism. Fix every Critical/Important finding and rerun affected tests.

- [ ] **Step 5: Commit and push scoped changes**

Stage only approved source, tests, docs, and workflow files. Never stage `.agents/skills/cc-skills-golang/`. Use atomic conventional commits, then `rtk git push -u origin agent/harden-execution-safety`.

- [ ] **Step 6: Open draft PR and verify GitHub CI**

Open one draft PR against `main` with root causes, behavior changes, tests, race evidence, and runtime smoke. Use `gh pr checks --watch --fail-fast` and inspect any failed job through `gh run view --log-failed`. Completion requires every required check successful.
