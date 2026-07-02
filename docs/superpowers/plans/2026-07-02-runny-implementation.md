# runny Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `runny`, a Go CLI/TUI that discovers child directories and runs one shell command across selected targets.

**Architecture:** Keep one shared core model used by CLI, discovery, runner, logs, history, and TUI. The CLI builds validated options, discovery builds a target tree, the runner emits events, and the TUI or auto renderer consumes those events.

**Tech Stack:** Go module `github.com/saewyn/runny`, standard `flag` package for CLI parsing, `go.yaml.in/yaml/v4` for strict YAML config loading, Bubble Tea v2/Bubbles v2/Lip Gloss for TUI, GoReleaser v2, GitHub Actions, Homebrew tap publishing to `saewyn/homebrew-tap`.

---

## Scope Check

This is one cohesive CLI product. The implementation is split by package so each task produces working, testable software:

- CLI/config/discovery define target selection.
- Runner/logs/history define execution behavior.
- TUI and auto mode consume the same runner.
- README, CI, GoReleaser, and Homebrew tap publishing complete the repository.

Before implementation, verify Go is available. Current planning environment returned `rtk: Failed to run go version: No such file or directory`, so Task 1 starts with toolchain verification.

## File Structure

- Create `go.mod`: module and dependency declarations.
- Create `cmd/runny/main.go`: thin entrypoint that calls `internal/app.Run`.
- Create `internal/core/types.go`: shared status, target, request, result, and event types.
- Create `internal/config/config.go`: defaults, strict YAML loading, merge precedence, validation.
- Create `internal/config/config_test.go`: config precedence and validation tests.
- Create `internal/cli/parse.go`: flag parsing, short flags, command after `--`, version/help detection.
- Create `internal/cli/parse_test.go`: CLI parsing tests.
- Create `internal/discovery/discovery.go`: directory scan, hidden exclusion, symlink skipping, depth, include/exclude, visible tree helpers.
- Create `internal/discovery/discovery_test.go`: discovery tests using temp directories and symlinks.
- Create `internal/logs/store.go`: per-target memory log capture and optional persisted logs.
- Create `internal/logs/store_test.go`: logging enabled/disabled/save behavior tests.
- Create `internal/history/history.go`: JSONL command and run history with retention.
- Create `internal/history/history_test.go`: history append/read/retention tests.
- Create `internal/runner/runner.go`: serial/parallel execution, worker pool, `/bin/sh -c`, cancellation, fail-fast, process cleanup.
- Create `internal/runner/runner_test.go`: runner behavior tests.
- Create `internal/app/app.go`: orchestration for TUI mode, auto mode, version output, exit codes.
- Create `internal/app/app_test.go`: auto-mode smoke tests with injected filesystem paths.
- Create `internal/tui/model.go`: Bubble Tea model state and Update logic.
- Create `internal/tui/view.go`: layout rendering, tree table, logs viewport, overlays.
- Create `internal/tui/model_test.go`: keybinding and state transition tests.
- Create `README.md`: initial user-facing documentation with Homebrew install and tap trust examples.
- Create `.gitignore`: ignore build/log/session artifacts.
- Create `.github/workflows/ci.yml`: formatting, vet, tests, build, GoReleaser check.
- Create `.github/workflows/release.yml`: tag release workflow.
- Create `.goreleaser.yaml`: macOS/Linux builds, archives, checksums, ldflags, Homebrew cask publishing.

## Shared Decisions

- Module path: `github.com/saewyn/runny`.
- Command string: join all args after `--` with one space.
- Shell: `/bin/sh -c <command>`.
- Config precedence: defaults, `~/.runny.yaml`, `./.runny.yaml`, explicit `--config`, CLI flags.
- `--recursive` without explicit `--depth` normalizes to unlimited depth (`0`).
- `--depth 1` means direct children.
- `--depth 0` means unlimited recursion.
- Hidden directories are excluded unless `include_hidden` or `--include-hidden` is true.
- Symlinked directories are skipped.
- `--include` and `--exclude` are mutually exclusive.
- `--serial` and `--workers` are mutually exclusive.
- `--disable-logging` and `--save-logs` are mutually exclusive.
- `workers: 0` means `min(runtime.NumCPU(), selected_target_count)`.
- Per-target logging is enabled by default.
- Homebrew tap repository: `github.com/saewyn/homebrew-tap`.
- Homebrew tap name for users: `saewyn/tap`.
- Release workflow needs secret `TAP_GITHUB_TOKEN` with permission to push to `saewyn/homebrew-tap`.
- Homebrew trust docs should include `brew trust --tap saewyn/tap` and `tap "saewyn/tap", trusted: true` for Brewfiles.
- Use GoReleaser `homebrew_casks`, not `brews`, because GoReleaser Homebrew formulas are deprecated for precompiled binaries.
- User install command should use `brew install --cask`.

---

### Task 1: Toolchain, Module, Core Types

**Files:**
- Create: `go.mod`
- Create: `cmd/runny/main.go`
- Create: `internal/core/types.go`
- Create: `internal/core/types_test.go`

- [ ] **Step 1: Verify Go toolchain**

Run:

```bash
go version
```

Expected: prints Go version. If command is missing, install Go before continuing. Do not write source files until this passes.

- [ ] **Step 2: Initialize module**

Run:

```bash
go mod init github.com/saewyn/runny
go get go.yaml.in/yaml/v4
go get charm.land/bubbletea/v2
go get charm.land/bubbles/v2
go get github.com/charmbracelet/lipgloss/v2
```

Expected: `go.mod` and `go.sum` exist.

- [ ] **Step 3: Write core type tests**

Create `internal/core/types_test.go`:

```go
package core

import "testing"

func TestSelectedTargets(t *testing.T) {
	targets := []Target{
		{ID: "a", RelPath: "api", Selected: true},
		{ID: "b", RelPath: "web", Selected: false},
		{ID: "c", RelPath: "worker", Selected: true},
	}

	selected := SelectedTargets(targets)
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].RelPath != "api" || selected[1].RelPath != "worker" {
		t.Fatalf("selected targets = %#v", selected)
	}
}

func TestStatusTerminal(t *testing.T) {
	terminal := []Status{StatusSucceeded, StatusFailed, StatusCancelled, StatusSkipped}
	for _, status := range terminal {
		if !status.Terminal() {
			t.Fatalf("%s should be terminal", status)
		}
	}

	active := []Status{StatusQueued, StatusRunning}
	for _, status := range active {
		if status.Terminal() {
			t.Fatalf("%s should not be terminal", status)
		}
	}
}
```

- [ ] **Step 4: Run core tests and confirm failure**

Run:

```bash
go test ./internal/core
```

Expected: FAIL because `Target`, `Status`, `SelectedTargets`, and status constants do not exist.

- [ ] **Step 5: Create core implementation**

Create `internal/core/types.go`:

```go
package core

import "time"

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusSkipped   Status = "skipped"
)

func (s Status) Terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusSkipped:
		return true
	default:
		return false
	}
}

type Target struct {
	ID       string
	RelPath  string
	AbsPath  string
	Name     string
	Depth    int
	ParentID string
	Children []string
	Selected bool
	Folded   bool
	Hidden   bool
	Skipped  bool
}

type ExecutionMode string

const (
	ModeParallel ExecutionMode = "parallel"
	ModeSerial   ExecutionMode = "serial"
)

type RunRequest struct {
	Command        string
	Targets        []Target
	Mode           ExecutionMode
	Workers        int
	FailFast       bool
	SaveLogs       bool
	DisableLogging bool
	LogRoot        string
}

type TargetResult struct {
	TargetID string
	Status   Status
	ExitCode int
	Duration time.Duration
	LogPath  string
	Error    string
}

type EventKind string

const (
	EventStatus EventKind = "status"
	EventOutput EventKind = "output"
	EventResult EventKind = "result"
)

type RunnerEvent struct {
	Kind     EventKind
	TargetID string
	Status   Status
	Stream   string
	Data     []byte
	Result   *TargetResult
}

func SelectedTargets(targets []Target) []Target {
	selected := make([]Target, 0, len(targets))
	for _, target := range targets {
		if target.Selected && !target.Skipped {
			selected = append(selected, target)
		}
	}
	return selected
}
```

- [ ] **Step 6: Add thin entrypoint**

Create `cmd/runny/main.go`:

```go
package main

import (
	"context"
	"os"

	"github.com/saewyn/runny/internal/app"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	code := app.Run(context.Background(), os.Args[1:], app.BuildInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, os.Stdout, os.Stderr)
	os.Exit(code)
}
```

- [ ] **Step 7: Run tests**

Run:

```bash
go test ./internal/core
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum cmd/runny/main.go internal/core/types.go internal/core/types_test.go
git commit -m "feat: add runny module skeleton"
```

---

### Task 2: Config Loading, Merge, Validation

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write config tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergePrecedence(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home.yaml")
	local := filepath.Join(dir, "local.yaml")
	explicit := filepath.Join(dir, "explicit.yaml")

	mustWrite(t, home, "depth: 2\nworkers: 2\ninclude_hidden: true\n")
	mustWrite(t, local, "depth: 3\nfail_fast: true\n")
	mustWrite(t, explicit, "workers: 5\nsave_logs: true\n")

	flags := Options{Depth: 4, DisableLogging: false}
	set := map[string]bool{"depth": true}

	got, err := Load(LoadRequest{
		HomePath:     home,
		LocalPath:    local,
		ExplicitPath: explicit,
		FlagValues:   flags,
		FlagSet:      set,
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got.Depth != 4 {
		t.Fatalf("depth = %d, want 4", got.Depth)
	}
	if got.Workers != 5 {
		t.Fatalf("workers = %d, want 5", got.Workers)
	}
	if !got.IncludeHidden {
		t.Fatal("include_hidden should come from home config")
	}
	if !got.FailFast {
		t.Fatal("fail_fast should come from local config")
	}
	if !got.SaveLogs {
		t.Fatal("save_logs should come from explicit config")
	}
}

func TestValidateMutualExclusions(t *testing.T) {
	tests := []Options{
		{Include: []string{"api"}, Exclude: []string{"legacy"}},
		{Serial: true, Workers: 2},
		{SaveLogs: true, DisableLogging: true},
		{Depth: -1},
	}

	for _, opts := range tests {
		if err := opts.Validate(); err == nil {
			t.Fatalf("Validate(%#v) returned nil", opts)
		}
	}
}

func TestNormalizeRecursiveDefaultDepth(t *testing.T) {
	opts := Defaults()
	opts.Recursive = true

	got := opts.Normalized(map[string]bool{})
	if got.Depth != 0 {
		t.Fatalf("recursive default depth = %d, want 0", got.Depth)
	}
}

func mustWrite(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
```

- [ ] **Step 2: Run config tests and confirm failure**

Run:

```bash
go test ./internal/config
```

Expected: FAIL because config package does not exist.

- [ ] **Step 3: Implement config package**

Create `internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"go.yaml.in/yaml/v4"
)

type Options struct {
	Depth          int
	Recursive      bool
	IncludeHidden  bool
	Include        []string
	Exclude        []string
	Workers        int
	Serial         bool
	FailFast       bool
	SaveLogs       bool
	DisableLogging bool
	ConfigPath     string
	Auto           bool
}

type FileConfig struct {
	Depth          *int     `yaml:"depth"`
	Recursive      *bool    `yaml:"recursive"`
	IncludeHidden  *bool    `yaml:"include_hidden"`
	Include        []string `yaml:"include"`
	Exclude        []string `yaml:"exclude"`
	Workers        *int     `yaml:"workers"`
	Serial         *bool    `yaml:"serial"`
	FailFast       *bool    `yaml:"fail_fast"`
	SaveLogs       *bool    `yaml:"save_logs"`
	DisableLogging *bool    `yaml:"disable_logging"`
}

type LoadRequest struct {
	HomePath     string
	LocalPath    string
	ExplicitPath string
	FlagValues   Options
	FlagSet      map[string]bool
}

func Defaults() Options {
	return Options{
		Depth: 1,
	}
}

func Load(req LoadRequest) (Options, error) {
	opts := Defaults()

	for _, path := range []string{req.HomePath, req.LocalPath, req.ExplicitPath} {
		if path == "" {
			continue
		}
		if err := applyFile(&opts, path); err != nil {
			return Options{}, err
		}
	}

	applyFlags(&opts, req.FlagValues, req.FlagSet)
	opts = opts.Normalized(req.FlagSet)
	if err := opts.Validate(); err != nil {
		return Options{}, err
	}
	return opts, nil
}

func (o Options) Normalized(flagSet map[string]bool) Options {
	if o.Recursive && o.Depth == 1 && !flagSet["depth"] {
		o.Depth = 0
	}
	return o
}

func (o Options) Validate() error {
	if len(o.Include) > 0 && len(o.Exclude) > 0 {
		return fmt.Errorf("include and exclude are mutually exclusive")
	}
	if o.Serial && o.Workers > 0 {
		return fmt.Errorf("serial and workers are mutually exclusive")
	}
	if o.SaveLogs && o.DisableLogging {
		return fmt.Errorf("save_logs and disable_logging are mutually exclusive")
	}
	if o.Depth < 0 {
		return fmt.Errorf("depth must be >= 0")
	}
	if o.Workers < 0 {
		return fmt.Errorf("workers must be >= 0")
	}
	return nil
}

func applyFile(opts *Options, path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	loader, err := yaml.NewLoader(f, yaml.WithKnownFields(), yaml.WithSingleDocument())
	if err != nil {
		return fmt.Errorf("create yaml loader %s: %w", path, err)
	}

	var cfg FileConfig
	if err := loader.Load(&cfg); err != nil {
		return fmt.Errorf("load config %s: %w", path, err)
	}

	if cfg.Depth != nil {
		opts.Depth = *cfg.Depth
	}
	if cfg.Recursive != nil {
		opts.Recursive = *cfg.Recursive
	}
	if cfg.IncludeHidden != nil {
		opts.IncludeHidden = *cfg.IncludeHidden
	}
	if cfg.Include != nil {
		opts.Include = append([]string(nil), cfg.Include...)
	}
	if cfg.Exclude != nil {
		opts.Exclude = append([]string(nil), cfg.Exclude...)
	}
	if cfg.Workers != nil {
		opts.Workers = *cfg.Workers
	}
	if cfg.Serial != nil {
		opts.Serial = *cfg.Serial
	}
	if cfg.FailFast != nil {
		opts.FailFast = *cfg.FailFast
	}
	if cfg.SaveLogs != nil {
		opts.SaveLogs = *cfg.SaveLogs
	}
	if cfg.DisableLogging != nil {
		opts.DisableLogging = *cfg.DisableLogging
	}
	return nil
}

func applyFlags(opts *Options, flags Options, set map[string]bool) {
	if set["auto"] {
		opts.Auto = flags.Auto
	}
	if set["depth"] {
		opts.Depth = flags.Depth
	}
	if set["recursive"] {
		opts.Recursive = flags.Recursive
	}
	if set["include_hidden"] {
		opts.IncludeHidden = flags.IncludeHidden
	}
	if set["include"] {
		opts.Include = append([]string(nil), flags.Include...)
	}
	if set["exclude"] {
		opts.Exclude = append([]string(nil), flags.Exclude...)
	}
	if set["workers"] {
		opts.Workers = flags.Workers
	}
	if set["serial"] {
		opts.Serial = flags.Serial
	}
	if set["fail_fast"] {
		opts.FailFast = flags.FailFast
	}
	if set["save_logs"] {
		opts.SaveLogs = flags.SaveLogs
	}
	if set["disable_logging"] {
		opts.DisableLogging = flags.DisableLogging
	}
	if set["config"] {
		opts.ConfigPath = flags.ConfigPath
	}
}
```

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/config
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go go.mod go.sum
git commit -m "feat: add config loading and validation"
```

---

### Task 3: CLI Flag Parsing

**Files:**
- Create: `internal/cli/parse.go`
- Create: `internal/cli/parse_test.go`

- [ ] **Step 1: Write CLI parse tests**

Create `internal/cli/parse_test.go`:

```go
package cli

import "testing"

func TestParseTUICommandAfterDashDash(t *testing.T) {
	got, err := Parse([]string{"--depth", "2", "-i", "api", "--", "npm", "test"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Command != "npm test" {
		t.Fatalf("command = %q, want %q", got.Command, "npm test")
	}
	if got.Options.Depth != 2 || got.Options.Include[0] != "api" {
		t.Fatalf("options = %#v", got.Options)
	}
	if !got.FlagSet["depth"] || !got.FlagSet["include"] {
		t.Fatalf("flag set = %#v", got.FlagSet)
	}
}

func TestParseShortFlags(t *testing.T) {
	got, err := Parse([]string{"-a", "-r", "-H", "-w", "4", "-f", "-L", "--", "pwd"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !got.Options.Auto || !got.Options.Recursive || !got.Options.IncludeHidden || !got.Options.FailFast || !got.Options.SaveLogs {
		t.Fatalf("options = %#v", got.Options)
	}
	if got.Options.Workers != 4 {
		t.Fatalf("workers = %d, want 4", got.Options.Workers)
	}
}

func TestParseVersion(t *testing.T) {
	got, err := Parse([]string{"-V"})
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if !got.ShowVersion {
		t.Fatal("ShowVersion should be true")
	}
}
```

- [ ] **Step 2: Run CLI tests and confirm failure**

Run:

```bash
go test ./internal/cli
```

Expected: FAIL because CLI package does not exist.

- [ ] **Step 3: Implement CLI parser**

Create `internal/cli/parse.go`:

```go
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/saewyn/runny/internal/config"
)

type Result struct {
	Options     config.Options
	FlagSet     map[string]bool
	Command     string
	ShowVersion bool
	ShowHelp    bool
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func Parse(args []string) (Result, error) {
	var opts config.Options
	var include stringList
	var exclude stringList
	var showVersion bool
	var showHelp bool

	fs := flag.NewFlagSet("runny", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.BoolVar(&opts.Auto, "auto", false, "run without TUI")
	fs.BoolVar(&opts.Auto, "a", false, "run without TUI")
	fs.BoolVar(&opts.Recursive, "recursive", false, "enable recursive discovery")
	fs.BoolVar(&opts.Recursive, "r", false, "enable recursive discovery")
	fs.IntVar(&opts.Depth, "depth", 1, "discovery depth")
	fs.IntVar(&opts.Depth, "d", 1, "discovery depth")
	fs.BoolVar(&opts.IncludeHidden, "include-hidden", false, "include hidden directories")
	fs.BoolVar(&opts.IncludeHidden, "H", false, "include hidden directories")
	fs.Var(&include, "include", "include pattern")
	fs.Var(&include, "i", "include pattern")
	fs.Var(&exclude, "exclude", "exclude pattern")
	fs.Var(&exclude, "e", "exclude pattern")
	fs.IntVar(&opts.Workers, "workers", 0, "parallel workers")
	fs.IntVar(&opts.Workers, "w", 0, "parallel workers")
	fs.BoolVar(&opts.Serial, "serial", false, "run serially")
	fs.BoolVar(&opts.Serial, "s", false, "run serially")
	fs.BoolVar(&opts.FailFast, "fail-fast", false, "stop after first failure")
	fs.BoolVar(&opts.FailFast, "f", false, "stop after first failure")
	fs.BoolVar(&opts.SaveLogs, "save-logs", false, "persist logs")
	fs.BoolVar(&opts.SaveLogs, "L", false, "persist logs")
	fs.BoolVar(&opts.DisableLogging, "disable-logging", false, "disable log capture")
	fs.BoolVar(&opts.DisableLogging, "N", false, "disable log capture")
	fs.StringVar(&opts.ConfigPath, "config", "", "config path")
	fs.StringVar(&opts.ConfigPath, "c", "", "config path")
	fs.BoolVar(&showVersion, "version", false, "print version")
	fs.BoolVar(&showVersion, "V", false, "print version")
	fs.BoolVar(&showHelp, "help", false, "print help")
	fs.BoolVar(&showHelp, "h", false, "print help")

	if err := fs.Parse(args); err != nil {
		return Result{}, err
	}

	opts.Include = []string(include)
	opts.Exclude = []string(exclude)

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		name := canonicalName(f.Name)
		set[name] = true
	})

	command := strings.Join(fs.Args(), " ")
	if opts.Auto && command == "" {
		return Result{}, fmt.Errorf("auto mode requires a command after --")
	}

	return Result{
		Options:     opts,
		FlagSet:     set,
		Command:     command,
		ShowVersion: showVersion,
		ShowHelp:    showHelp,
	}, nil
}

func canonicalName(name string) string {
	switch name {
	case "a":
		return "auto"
	case "r":
		return "recursive"
	case "d":
		return "depth"
	case "H":
		return "include_hidden"
	case "include-hidden":
		return "include_hidden"
	case "i":
		return "include"
	case "e":
		return "exclude"
	case "w":
		return "workers"
	case "s":
		return "serial"
	case "f":
		return "fail_fast"
	case "fail-fast":
		return "fail_fast"
	case "L":
		return "save_logs"
	case "save-logs":
		return "save_logs"
	case "N":
		return "disable_logging"
	case "disable-logging":
		return "disable_logging"
	case "c":
		return "config"
	default:
		return name
	}
}
```

- [ ] **Step 4: Run CLI tests**

Run:

```bash
go test ./internal/cli
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/parse.go internal/cli/parse_test.go
git commit -m "feat: add cli flag parsing"
```

---

### Task 4: Directory Discovery

**Files:**
- Create: `internal/discovery/discovery.go`
- Create: `internal/discovery/discovery_test.go`

- [ ] **Step 1: Write discovery tests**

Create `internal/discovery/discovery_test.go`:

```go
package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/saewyn/runny/internal/config"
	"github.com/saewyn/runny/internal/core"
)

func TestDiscoverDirectExcludesHiddenAndSymlink(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "api")
	mkdir(t, root, ".hidden")
	mkdir(t, root, "real")
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "linked")); err != nil {
			t.Fatalf("symlink: %v", err)
		}
	}

	targets, err := Discover(root, config.Defaults())
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	paths := relPaths(targets)
	if contains(paths, ".hidden") || contains(paths, "linked") {
		t.Fatalf("unexpected paths: %#v", paths)
	}
	if !contains(paths, "api") || !contains(paths, "real") {
		t.Fatalf("missing paths: %#v", paths)
	}
}

func TestDiscoverDepthAndTree(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "api", "cmd")
	mkdir(t, root, "api", "internal")

	opts := config.Defaults()
	opts.Depth = 2
	targets, err := Discover(root, opts)
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	paths := relPaths(targets)
	for _, want := range []string{"api", filepath.Join("api", "cmd"), filepath.Join("api", "internal")} {
		if !contains(paths, want) {
			t.Fatalf("missing %s in %#v", want, paths)
		}
	}
}

func TestIncludeExclude(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "api")
	mkdir(t, root, "web")
	mkdir(t, root, "legacy")

	opts := config.Defaults()
	opts.Include = []string{"api"}
	targets, err := Discover(root, opts)
	if err != nil {
		t.Fatalf("Discover include returned error: %v", err)
	}
	if paths := relPaths(targets); len(paths) != 1 || paths[0] != "api" {
		t.Fatalf("include paths = %#v", paths)
	}

	opts = config.Defaults()
	opts.Exclude = []string{"legacy"}
	targets, err = Discover(root, opts)
	if err != nil {
		t.Fatalf("Discover exclude returned error: %v", err)
	}
	if contains(relPaths(targets), "legacy") {
		t.Fatalf("legacy should be excluded: %#v", relPaths(targets))
	}
}

func mkdir(t *testing.T, root string, parts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(append([]string{root}, parts...)...), 0o755); err != nil {
		t.Fatalf("mkdir %v: %v", parts, err)
	}
}

func relPaths(targets []core.Target) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, target.RelPath)
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run discovery tests and confirm failure**

Run:

```bash
go test ./internal/discovery
```

Expected: FAIL because discovery package does not exist.

- [ ] **Step 3: Implement discovery**

Create `internal/discovery/discovery.go`:

```go
package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/saewyn/runny/internal/config"
	"github.com/saewyn/runny/internal/core"
)

func Discover(root string, opts config.Options) ([]core.Target, error) {
	maxDepth := opts.Depth
	targets := []core.Target{}
	byID := map[string]int{}

	var walk func(abs, parentID string, depth int) error
	walk = func(abs, parentID string, depth int) error {
		if maxDepth != 0 && depth > maxDepth {
			return nil
		}

		entries, err := os.ReadDir(abs)
		if err != nil {
			return fmt.Errorf("read directory %s: %w", abs, err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

		for _, entry := range entries {
			name := entry.Name()
			if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if !entry.IsDir() {
				continue
			}

			childAbs := filepath.Join(abs, name)
			rel, err := filepath.Rel(root, childAbs)
			if err != nil {
				return fmt.Errorf("relative path for %s: %w", childAbs, err)
			}
			rel = filepath.ToSlash(rel)
			if !matches(opts, name, rel) {
				if maxDepth == 0 || depth < maxDepth {
					if err := walk(childAbs, parentID, depth+1); err != nil {
						return err
					}
				}
				continue
			}

			id := rel
			target := core.Target{
				ID:       id,
				RelPath:  rel,
				AbsPath:  childAbs,
				Name:     name,
				Depth:    depth,
				ParentID: parentID,
				Selected: true,
				Folded:   depth == 1,
			}
			byID[id] = len(targets)
			targets = append(targets, target)
			if parentID != "" {
				parentIndex := byID[parentID]
				targets[parentIndex].Children = append(targets[parentIndex].Children, id)
			}

			if maxDepth == 0 || depth < maxDepth {
				if err := walk(childAbs, id, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := walk(root, "", 1); err != nil {
		return nil, err
	}
	return targets, nil
}

func matches(opts config.Options, name, rel string) bool {
	if len(opts.Include) > 0 {
		for _, pattern := range opts.Include {
			if wildcardMatch(pattern, name) || wildcardMatch(pattern, rel) || strings.Contains(rel, pattern) {
				return true
			}
		}
		return false
	}
	for _, pattern := range opts.Exclude {
		if wildcardMatch(pattern, name) || wildcardMatch(pattern, rel) || strings.Contains(rel, pattern) {
			return false
		}
	}
	return true
}

func wildcardMatch(pattern, value string) bool {
	ok, err := filepath.Match(pattern, value)
	return err == nil && ok
}
```

- [ ] **Step 4: Run discovery tests**

Run:

```bash
go test ./internal/discovery
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/discovery/discovery.go internal/discovery/discovery_test.go
git commit -m "feat: add directory discovery"
```

---

### Task 5: Logs and History

**Files:**
- Create: `internal/logs/store.go`
- Create: `internal/logs/store_test.go`
- Create: `internal/history/history.go`
- Create: `internal/history/history_test.go`

- [ ] **Step 1: Write logs tests**

Create `internal/logs/store_test.go`:

```go
package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreCapturesByDefault(t *testing.T) {
	store := NewStore(Options{Capture: true})
	store.Write("api", "stdout", []byte("hello\n"))
	if got := store.Content("api"); got != "hello\n" {
		t.Fatalf("content = %q, want hello", got)
	}
}

func TestStoreDisabled(t *testing.T) {
	store := NewStore(Options{Capture: false})
	store.Write("api", "stdout", []byte("hello\n"))
	if got := store.Content("api"); got != "" {
		t.Fatalf("content = %q, want empty", got)
	}
}

func TestStorePersists(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(Options{Capture: true, Save: true, Root: dir})
	store.Write("api/service", "stderr", []byte("boom\n"))
	path, err := store.CloseTarget("api/service")
	if err != nil {
		t.Fatalf("CloseTarget returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(data), "boom") {
		t.Fatalf("log data = %q", data)
	}
	if filepath.Base(path) != "api_service.log" {
		t.Fatalf("log basename = %s", filepath.Base(path))
	}
}
```

- [ ] **Step 2: Write history tests**

Create `internal/history/history_test.go`:

```go
package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCommandHistoryRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	store := NewStore(path, 2)

	for _, cmd := range []string{"one", "two", "three"} {
		if err := store.AppendCommand(CommandEntry{Command: cmd, Timestamp: time.Unix(1, 0)}); err != nil {
			t.Fatalf("append command: %v", err)
		}
	}

	entries, err := store.Commands()
	if err != nil {
		t.Fatalf("commands: %v", err)
	}
	if len(entries) != 2 || entries[0].Command != "two" || entries[1].Command != "three" {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestRunHistoryRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	store := NewStore(path, 100)
	entry := RunEntry{Command: "npm test", SelectedTargets: 3, Failed: 1, Timestamp: time.Unix(2, 0)}

	if err := store.AppendRun(entry); err != nil {
		t.Fatalf("append run: %v", err)
	}
	entries, err := store.Runs()
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(entries) != 1 || entries[0].Command != "npm test" || entries[0].Failed != 1 {
		t.Fatalf("entries = %#v", entries)
	}
}
```

- [ ] **Step 3: Run logs/history tests and confirm failure**

Run:

```bash
go test ./internal/logs ./internal/history
```

Expected: FAIL because packages do not exist.

- [ ] **Step 4: Implement log store**

Create `internal/logs/store.go`:

```go
package logs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Options struct {
	Capture bool
	Save    bool
	Root    string
}

type Store struct {
	mu      sync.Mutex
	options Options
	buffers map[string]*bytes.Buffer
}

func NewStore(options Options) *Store {
	return &Store{options: options, buffers: map[string]*bytes.Buffer{}}
}

func (s *Store) Write(targetID, stream string, data []byte) {
	if !s.options.Capture {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	buf := s.buffers[targetID]
	if buf == nil {
		buf = &bytes.Buffer{}
		s.buffers[targetID] = buf
	}
	buf.Write(data)
}

func (s *Store) Content(targetID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buffers[targetID] == nil {
		return ""
	}
	return s.buffers[targetID].String()
}

func (s *Store) CloseTarget(targetID string) (string, error) {
	if !s.options.Capture || !s.options.Save {
		return "", nil
	}
	s.mu.Lock()
	content := ""
	if s.buffers[targetID] != nil {
		content = s.buffers[targetID].String()
	}
	s.mu.Unlock()

	if err := os.MkdirAll(s.options.Root, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.options.Root, sanitize(targetID)+".log")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func sanitize(value string) string {
	value = strings.Trim(value, "/")
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, "/", "_")
	if value == "" {
		return "root"
	}
	return value
}
```

- [ ] **Step 5: Implement history store**

Create `internal/history/history.go`:

```go
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type CommandEntry struct {
	Command   string    `json:"command"`
	Timestamp time.Time `json:"timestamp"`
}

type RunEntry struct {
	Command         string        `json:"command"`
	Mode            string        `json:"mode"`
	SelectedTargets int           `json:"selected_targets"`
	Succeeded       int           `json:"succeeded"`
	Failed          int           `json:"failed"`
	Cancelled       int           `json:"cancelled"`
	Duration        time.Duration `json:"duration"`
	Timestamp       time.Time     `json:"timestamp"`
}

type Store struct {
	path      string
	retention int
}

func NewStore(path string, retention int) Store {
	return Store{path: path, retention: retention}
}

func (s Store) AppendCommand(entry CommandEntry) error {
	entries, err := s.Commands()
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	if len(entries) > s.retention {
		entries = entries[len(entries)-s.retention:]
	}
	return writeJSONL(s.path, entries)
}

func (s Store) Commands() ([]CommandEntry, error) {
	return readJSONL[CommandEntry](s.path)
}

func (s Store) AppendRun(entry RunEntry) error {
	entries, err := s.Runs()
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	if len(entries) > s.retention {
		entries = entries[len(entries)-s.retention:]
	}
	return writeJSONL(s.path, entries)
}

func (s Store) Runs() ([]RunEntry, error) {
	return readJSONL[RunEntry](s.path)
}

func readJSONL[T any](path string) ([]T, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var entries []T
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry T
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}

func writeJSONL[T any](path string, entries []T) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 6: Run logs/history tests**

Run:

```bash
go test ./internal/logs ./internal/history
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/logs/store.go internal/logs/store_test.go internal/history/history.go internal/history/history_test.go
git commit -m "feat: add logs and history stores"
```

---

### Task 6: Runner

**Files:**
- Create: `internal/runner/runner.go`
- Create: `internal/runner/runner_test.go`

- [ ] **Step 1: Write runner tests**

Create `internal/runner/runner_test.go`:

```go
package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/saewyn/runny/internal/core"
)

func TestRunSucceeds(t *testing.T) {
	target := tempTarget(t, "api")
	result, events := runForTest(t, core.RunRequest{
		Command: "printf hello",
		Targets: []core.Target{target},
		Workers: 1,
	})

	if result.ExitCode != 0 || result.Failed != 0 || result.Succeeded != 1 {
		t.Fatalf("summary = %#v", result)
	}
	if len(events) == 0 {
		t.Fatal("events should not be empty")
	}
}

func TestRunReportsFailure(t *testing.T) {
	target := tempTarget(t, "api")
	result, _ := runForTest(t, core.RunRequest{
		Command: "exit 7",
		Targets: []core.Target{target},
		Workers: 1,
	})
	if result.ExitCode == 0 || result.Failed != 1 {
		t.Fatalf("summary = %#v", result)
	}
}

func TestSerialUsesOneWorker(t *testing.T) {
	one := tempTarget(t, "one")
	two := tempTarget(t, "two")
	result, _ := runForTest(t, core.RunRequest{
		Command: "pwd",
		Targets: []core.Target{one, two},
		Mode: core.ModeSerial,
		Workers: 9,
	})
	if result.Succeeded != 2 {
		t.Fatalf("summary = %#v", result)
	}
}

func tempTarget(t *testing.T, name string) core.Target {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return core.Target{ID: name, RelPath: name, AbsPath: dir, Selected: true}
}

func runForTest(t *testing.T, req core.RunRequest) (Summary, []core.RunnerEvent) {
	t.Helper()
	events := []core.RunnerEvent{}
	summary, err := Run(context.Background(), req, func(event core.RunnerEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	return summary, events
}
```

- [ ] **Step 2: Run runner tests and confirm failure**

Run:

```bash
go test ./internal/runner
```

Expected: FAIL because runner package does not exist.

- [ ] **Step 3: Implement runner**

Create `internal/runner/runner.go`:

```go
package runner

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/saewyn/runny/internal/core"
)

type Summary struct {
	Succeeded int
	Failed    int
	Cancelled int
	Skipped   int
	ExitCode  int
	Results   []core.TargetResult
}

type EmitFunc func(core.RunnerEvent)

func Run(ctx context.Context, req core.RunRequest, emit EmitFunc) (Summary, error) {
	targets := core.SelectedTargets(req.Targets)
	workers := workerCount(req, len(targets))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan core.Target)
	results := make(chan core.TargetResult)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for target := range jobs {
				result := runTarget(ctx, req.Command, target, emit)
				results <- result
				if req.FailFast && result.Status == core.StatusFailed {
					cancel()
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, target := range targets {
			select {
			case <-ctx.Done():
				results <- core.TargetResult{TargetID: target.ID, Status: core.StatusSkipped}
			case jobs <- target:
				emit(core.RunnerEvent{Kind: core.EventStatus, TargetID: target.ID, Status: core.StatusQueued})
			}
		}
		wg.Wait()
		close(results)
	}()

	var summary Summary
	for result := range results {
		summary.Results = append(summary.Results, result)
		switch result.Status {
		case core.StatusSucceeded:
			summary.Succeeded++
		case core.StatusFailed:
			summary.Failed++
		case core.StatusCancelled:
			summary.Cancelled++
		case core.StatusSkipped:
			summary.Skipped++
		}
	}
	if summary.Failed > 0 || summary.Cancelled > 0 {
		summary.ExitCode = 1
	}
	return summary, nil
}

func workerCount(req core.RunRequest, targetCount int) int {
	if targetCount == 0 {
		return 1
	}
	if req.Mode == core.ModeSerial {
		return 1
	}
	if req.Workers > 0 {
		if req.Workers > targetCount {
			return targetCount
		}
		return req.Workers
	}
	cpus := runtime.NumCPU()
	if cpus > targetCount {
		return targetCount
	}
	if cpus < 1 {
		return 1
	}
	return cpus
}

func runTarget(ctx context.Context, command string, target core.Target, emit EmitFunc) core.TargetResult {
	start := time.Now()
	emit(core.RunnerEvent{Kind: core.EventStatus, TargetID: target.ID, Status: core.StatusRunning})

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = target.AbsPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return failed(target.ID, start, err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return failed(target.ID, start, err)
	}
	if err := cmd.Start(); err != nil {
		return failed(target.ID, start, err)
	}

	var scanWG sync.WaitGroup
	scan := func(stream string, scanner *bufio.Scanner) {
		defer scanWG.Done()
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			line = append(line, '\n')
			emit(core.RunnerEvent{Kind: core.EventOutput, TargetID: target.ID, Stream: stream, Data: line})
		}
	}
	scanWG.Add(2)
	go scan("stdout", bufio.NewScanner(stdout))
	go scan("stderr", bufio.NewScanner(stderr))

	err = cmd.Wait()
	scanWG.Wait()

	if ctx.Err() != nil {
		killProcessGroup(cmd)
		result := core.TargetResult{TargetID: target.ID, Status: core.StatusCancelled, Duration: time.Since(start), Error: ctx.Err().Error()}
		emit(core.RunnerEvent{Kind: core.EventResult, TargetID: target.ID, Status: result.Status, Result: &result})
		return result
	}
	if err != nil {
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		result := core.TargetResult{TargetID: target.ID, Status: core.StatusFailed, ExitCode: exitCode, Duration: time.Since(start), Error: err.Error()}
		emit(core.RunnerEvent{Kind: core.EventResult, TargetID: target.ID, Status: result.Status, Result: &result})
		return result
	}

	result := core.TargetResult{TargetID: target.ID, Status: core.StatusSucceeded, Duration: time.Since(start)}
	emit(core.RunnerEvent{Kind: core.EventResult, TargetID: target.ID, Status: result.Status, Result: &result})
	return result
}

func failed(targetID string, start time.Time, err error) core.TargetResult {
	return core.TargetResult{TargetID: targetID, Status: core.StatusFailed, ExitCode: 1, Duration: time.Since(start), Error: err.Error()}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
```

- [ ] **Step 4: Run runner tests**

Run:

```bash
go test ./internal/runner
```

Expected: PASS on macOS/Linux.

- [ ] **Step 5: Commit**

```bash
git add internal/runner/runner.go internal/runner/runner_test.go
git commit -m "feat: add command runner"
```

---

### Task 7: App Orchestration and Auto Mode

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`
- Modify: `cmd/runny/main.go`

- [ ] **Step 1: Write app tests**

Create `internal/app/app_test.go`:

```go
package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionOutput(t *testing.T) {
	var out bytes.Buffer
	code := Run(testContext(), []string{"--version"}, BuildInfo{Version: "1.2.3", Commit: "abc", Date: "today"}, &out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "runny 1.2.3") {
		t.Fatalf("output = %q", out.String())
	}
}

func TestAutoModeRunsCommand(t *testing.T) {
	root := t.TempDir()
	mkdir(t, root, "api")
	oldwd, _ := os.Getwd()
	defer os.Chdir(oldwd)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var out bytes.Buffer
	code := Run(testContext(), []string{"--auto", "--", "pwd"}, BuildInfo{Version: "dev"}, &out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("code = %d, output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), "api") {
		t.Fatalf("output = %q", out.String())
	}
}

func testContext() context.Context {
	return context.Background()
}

func mkdir(t *testing.T, root string, parts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(append([]string{root}, parts...)...), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}
```

- [ ] **Step 2: Run app tests and confirm failure**

Run:

```bash
go test ./internal/app
```

Expected: FAIL because app package does not exist.

- [ ] **Step 3: Implement app package**

Create `internal/app/app.go`:

```go
package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/saewyn/runny/internal/cli"
	"github.com/saewyn/runny/internal/config"
	"github.com/saewyn/runny/internal/core"
	"github.com/saewyn/runny/internal/discovery"
	"github.com/saewyn/runny/internal/logs"
	"github.com/saewyn/runny/internal/runner"
	"github.com/saewyn/runny/internal/tui"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func Run(ctx context.Context, args []string, build BuildInfo, stdout, stderr io.Writer) int {
	parsed, err := cli.Parse(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if parsed.ShowVersion {
		fmt.Fprintf(stdout, "runny %s (%s %s)\n", build.Version, build.Commit, build.Date)
		return 0
	}
	if parsed.ShowHelp {
		fmt.Fprintln(stdout, helpText())
		return 0
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	home, _ := os.UserHomeDir()
	explicit := parsed.Options.ConfigPath

	opts, err := config.Load(config.LoadRequest{
		HomePath:     filepath.Join(home, ".runny.yaml"),
		LocalPath:    filepath.Join(cwd, ".runny.yaml"),
		ExplicitPath: explicit,
		FlagValues:   parsed.Options,
		FlagSet:      parsed.FlagSet,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	targets, err := discovery.Discover(cwd, opts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	if !opts.Auto {
		return tui.Run(ctx, tui.Options{Command: parsed.Command, Targets: targets, Config: opts, Stdout: stdout, Stderr: stderr})
	}

	return runAuto(ctx, parsed.Command, opts, targets, stdout, stderr)
}

func runAuto(ctx context.Context, command string, opts config.Options, targets []core.Target, stdout, stderr io.Writer) int {
	mode := core.ModeParallel
	if opts.Serial {
		mode = core.ModeSerial
	}
	logStore := logs.NewStore(logs.Options{Capture: !opts.DisableLogging, Save: opts.SaveLogs, Root: filepath.Join(".runny", "runs", "latest")})
	req := core.RunRequest{
		Command: command,
		Targets: targets,
		Mode: mode,
		Workers: opts.Workers,
		FailFast: opts.FailFast,
		SaveLogs: opts.SaveLogs,
		DisableLogging: opts.DisableLogging,
	}
	summary, err := runner.Run(ctx, req, func(event core.RunnerEvent) {
		if event.Kind == core.EventOutput {
			logStore.Write(event.TargetID, event.Stream, event.Data)
			fmt.Fprintf(stdout, "[%s] %s", event.TargetID, string(event.Data))
		}
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "succeeded=%d failed=%d cancelled=%d skipped=%d\n", summary.Succeeded, summary.Failed, summary.Cancelled, summary.Skipped)
	return summary.ExitCode
}

func helpText() string {
	return `runny runs one shell command across child directories.

Usage:
  runny
  runny -- <command>
  runny --auto -- <command>`
}
```

- [ ] **Step 4: Add temporary TUI shim**

Create `internal/tui/model.go` with a temporary compiling shim. Task 8 replaces it with full TUI:

```go
package tui

import (
	"context"
	"fmt"
	"io"

	"github.com/saewyn/runny/internal/config"
	"github.com/saewyn/runny/internal/core"
)

type Options struct {
	Command string
	Targets []core.Target
	Config  config.Options
	Stdout  io.Writer
	Stderr  io.Writer
}

func Run(ctx context.Context, opts Options) int {
	fmt.Fprintln(opts.Stderr, "interactive TUI unavailable until Task 8")
	return 2
}
```

- [ ] **Step 5: Run app tests**

Run:

```bash
go test ./internal/app
```

Expected: PASS.

- [ ] **Step 6: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/app.go internal/app/app_test.go internal/tui/model.go cmd/runny/main.go
git commit -m "feat: add app orchestration and auto mode"
```

---

### Task 8: TUI Model and Keybindings

**Files:**
- Modify: `internal/tui/model.go`
- Create: `internal/tui/view.go`
- Create: `internal/tui/model_test.go`

- [ ] **Step 1: Write TUI model tests**

Create `internal/tui/model_test.go`:

```go
package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/saewyn/runny/internal/config"
	"github.com/saewyn/runny/internal/core"
)

func TestToggleSelection(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}, Config: config.Defaults()})
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	got := updated.(Model)
	if got.targets[0].Selected {
		t.Fatal("target should be deselected")
	}
}

func TestHelpOverlay(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}, Config: config.Defaults()})
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(Model)
	if got.overlay != overlayHelp {
		t.Fatalf("overlay = %q, want help", got.overlay)
	}
}

func TestFoldUnfold(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}, Folded: true},
		{ID: "api/cmd", RelPath: "api/cmd", Selected: true, ParentID: "api", Depth: 2},
	}, Config: config.Defaults()})
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	got := updated.(Model)
	if got.targets[0].Folded {
		t.Fatal("target should be unfolded")
	}
}
```

- [ ] **Step 2: Run TUI tests and confirm failure**

Run:

```bash
go test ./internal/tui
```

Expected: FAIL because temporary shim has no `NewModel`, `Model`, or key handling.

- [ ] **Step 3: Implement TUI model**

Replace `internal/tui/model.go`:

```go
package tui

import (
	"context"
	"io"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"

	"github.com/saewyn/runny/internal/config"
	"github.com/saewyn/runny/internal/core"
)

type Options struct {
	Command string
	Targets []core.Target
	Config  config.Options
	Stdout  io.Writer
	Stderr  io.Writer
}

type overlay string

const (
	overlayNone    overlay = ""
	overlayHelp    overlay = "help"
	overlayHistory overlay = "history"
	overlayRerun   overlay = "rerun"
)

type Model struct {
	command  textinput.Model
	filter   textinput.Model
	logs     viewport.Model
	targets  []core.Target
	focus    int
	cursor   int
	overlay  overlay
	running  bool
	config   config.Options
}

func NewModel(opts Options) Model {
	command := textinput.New()
	command.SetValue(opts.Command)
	filter := textinput.New()
	filter.Placeholder = "filter directories"
	logs := viewport.New(viewport.WithWidth(40), viewport.WithHeight(20))

	return Model{
		command: command,
		filter:  filter,
		logs:    logs,
		targets: append([]core.Target(nil), opts.Targets...),
		config:  opts.Config,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.overlay != overlayNone {
		switch msg.String() {
		case "esc":
			m.overlay = overlayNone
		case "enter":
			if m.overlay == overlayRerun {
				m.overlay = overlayNone
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if !m.running {
			return m, tea.Quit
		}
	case "?":
		m.overlay = overlayHelp
	case "H":
		m.overlay = overlayHistory
	case "R":
		if !m.running {
			m.overlay = overlayRerun
		}
	case " ":
		m.toggleCurrent()
	case "a":
		m.setVisibleSelection(true)
	case "A":
		m.setVisibleSelection(false)
	case "right", "l":
		m.setFolded(false)
	case "left":
		m.setFolded(true)
	}
	return m, nil
}

func (m *Model) toggleCurrent() {
	if len(m.targets) == 0 || m.cursor >= len(m.targets) {
		return
	}
	m.targets[m.cursor].Selected = !m.targets[m.cursor].Selected
}

func (m *Model) setVisibleSelection(selected bool) {
	for i := range m.targets {
		m.targets[i].Selected = selected
	}
}

func (m *Model) setFolded(folded bool) {
	if len(m.targets) == 0 || m.cursor >= len(m.targets) {
		return
	}
	if len(m.targets[m.cursor].Children) > 0 {
		m.targets[m.cursor].Folded = folded
	}
}

func Run(ctx context.Context, opts Options) int {
	program := tea.NewProgram(NewModel(opts))
	if _, err := program.Run(); err != nil {
		return 1
	}
	return 0
}
```

- [ ] **Step 4: Implement view rendering**

Create `internal/tui/view.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/saewyn/runny/internal/core"
)

func (m Model) View() string {
	if m.overlay == overlayHelp {
		return helpView()
	}
	if m.overlay == overlayHistory {
		return "History\n\nenter reuse command · esc close\n"
	}
	if m.overlay == overlayRerun {
		return "Re-run failed targets?\n\nenter confirm · esc cancel\n"
	}

	var b strings.Builder
	b.WriteString("Command: ")
	b.WriteString(m.command.Value())
	b.WriteString("\nFilter: ")
	b.WriteString(m.filter.Value())
	b.WriteString("\n\n")
	b.WriteString(renderTargets(m.targets, m.cursor))
	b.WriteString("\n\nLogs\n")
	b.WriteString(m.logs.View())
	b.WriteString("\n\nspace toggle · / filter · enter run · H history · ? help\n")
	return b.String()
}

func renderTargets(targets []core.Target, cursor int) string {
	var b strings.Builder
	b.WriteString("sel fold directory status\n")
	for i, target := range targets {
		selected := " "
		if target.Selected {
			selected = "x"
		}
		fold := " "
		if len(target.Children) > 0 {
			if target.Folded {
				fold = ">"
			} else {
				fold = "v"
			}
		}
		prefix := " "
		if i == cursor {
			prefix = ">"
		}
		fmt.Fprintf(&b, "%s [%s] %s %s %s\n", prefix, selected, fold, target.RelPath, core.StatusQueued)
	}
	return b.String()
}

func helpView() string {
	return `runny help

space toggle target
a select all
A deselect all
/ filter
enter run or confirm
right/l unfold
left fold
H history
? help
del cancel selected active runs
R re-run failed targets
ctrl+c cancel all and quit
q quit when idle
esc close overlay
`
}
```

- [ ] **Step 5: Run TUI tests**

Run:

```bash
go test ./internal/tui
```

Expected: PASS.

- [ ] **Step 6: Run all tests**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/model.go internal/tui/view.go internal/tui/model_test.go
git commit -m "feat: add tui model and keybindings"
```

---

### Task 9: Runner Integration in TUI

**Files:**
- Modify: `internal/tui/model.go`
- Modify: `internal/tui/view.go`
- Modify: `internal/tui/model_test.go`

- [ ] **Step 1: Add runner integration tests**

Append to `internal/tui/model_test.go`:

```go
func TestRerunFailedOverlayDisabledWhileRunning(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}, Config: config.Defaults()})
	model.running = true

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(Model)
	if got.overlay != overlayNone {
		t.Fatalf("overlay = %q, want none", got.overlay)
	}
}

func TestDeleteMarksCancellationRequest(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}, Config: config.Defaults()})
	model.running = true

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	got := updated.(Model)
	if len(got.cancelRequested) != 1 || !got.cancelRequested["api"] {
		t.Fatalf("cancelRequested = %#v", got.cancelRequested)
	}
}
```

- [ ] **Step 2: Run TUI tests and confirm failure**

Run:

```bash
go test ./internal/tui
```

Expected: FAIL because `cancelRequested` does not exist.

- [ ] **Step 3: Add run-state fields and delete behavior**

Modify `internal/tui/model.go` `Model` struct:

```go
	cancelRequested map[string]bool
	statuses        map[string]core.Status
	logContent      map[string]string
```

Initialize these maps in `NewModel`:

```go
cancelRequested: map[string]bool{},
statuses:        map[string]core.Status{},
logContent:      map[string]string{},
```

Add delete case in `handleKey`:

```go
case "delete", "backspace":
	if m.running {
		m.requestCancel()
	}
```

Add helper:

```go
func (m *Model) requestCancel() {
	for _, target := range m.targets {
		if target.Selected && m.statuses[target.ID] == core.StatusRunning {
			m.cancelRequested[target.ID] = true
		}
	}
	if len(m.cancelRequested) == 0 && len(m.targets) > 0 && m.cursor < len(m.targets) {
		m.cancelRequested[m.targets[m.cursor].ID] = true
	}
}
```

- [ ] **Step 4: Add runner command message types**

Create message types in `internal/tui/model.go`:

```go
type runnerEventMsg struct{ event core.RunnerEvent }
type runnerDoneMsg struct{ exitCode int }
```

Keep actual process execution behind a function field for testability:

```go
runFunc func(context.Context, core.RunRequest, func(core.RunnerEvent)) (int, error)
```

Default `runFunc` should call `runner.Run` and return its summary exit code.

- [ ] **Step 5: Run TUI tests**

Run:

```bash
go test ./internal/tui
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/model.go internal/tui/view.go internal/tui/model_test.go
git commit -m "feat: integrate runner state into tui"
```

---

### Task 10: README, Gitignore, CI, Release, Homebrew

**Files:**
- Create: `README.md`
- Create: `.gitignore`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Create `.gitignore`**

Create `.gitignore`:

```gitignore
.claude/
.superpowers/
.runny/runs/
dist/
runny
*.test
```

- [ ] **Step 2: Create README**

Create `README.md`:

```markdown
# runny

`runny` runs one shell command across many child directories from a terminal UI.

## Install

With Homebrew:

```bash
brew install --cask saewyn/tap/runny
```

If your Homebrew setup requires tap trust:

```bash
brew tap saewyn/tap
brew trust --tap saewyn/tap
brew install --cask runny
```

In a Brewfile:

```ruby
tap "saewyn/tap", trusted: true
cask "saewyn/tap/runny", trusted: true
```

With Go:

```bash
go install github.com/saewyn/runny/cmd/runny@latest
```

## Usage

Open the TUI with an empty command field:

```bash
runny
```

Open the TUI with a command:

```bash
runny -- npm test
```

Run without the TUI:

```bash
runny --auto -- npm test
```

Scan recursively:

```bash
runny --depth 2 -- npm test
runny --recursive -- npm test
```

Filter targets:

```bash
runny --include api -- npm test
runny --exclude legacy -- npm test
```

Control execution:

```bash
runny --workers 4 -- npm test
runny --serial -- npm test
runny --fail-fast -- npm test
```

Logs are captured in memory by default. Disable capture for very noisy commands:

```bash
runny --disable-logging --auto -- npm test
```

Persist logs:

```bash
runny --save-logs -- npm test
```

## Config

`runny` loads `~/.runny.yaml`, then `./.runny.yaml`, then CLI flags.

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

## Keybindings

| Key | Action |
| --- | --- |
| `space` | Toggle target selection |
| `a` | Select all visible targets |
| `A` | Deselect all visible targets |
| `/` | Filter |
| `enter` | Run or confirm |
| `right` / `l` | Unfold |
| `left` | Fold |
| `H` | History |
| `?` | Help |
| `del` | Cancel selected active runs |
| `R` | Re-run failed targets with confirmation |
| `ctrl+c` | Cancel active runs and quit |
| `q` | Quit when idle |

## Release

Tagged releases use GoReleaser:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser publishes release archives and updates the Homebrew cask in `saewyn/homebrew-tap`. Configure repository secret `TAP_GITHUB_TOKEN` with write access to that tap before the first tag release.
```

- [ ] **Step 3: Create CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
          cache: true
      - name: Check formatting
        run: test -z "$(gofmt -l .)"
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./...
      - name: Build Linux
        run: GOOS=linux GOARCH=amd64 go build ./cmd/runny
      - name: Build macOS
        run: GOOS=darwin GOARCH=amd64 go build ./cmd/runny
      - name: GoReleaser check
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: check
```

- [ ] **Step 4: Create release workflow**

Create `.github/workflows/release.yml`:

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: stable
          cache: true
      - uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
```

- [ ] **Step 5: Create GoReleaser config**

Create `.goreleaser.yaml`:

```yaml
version: 2

project_name: runny

before:
  hooks:
    - go mod tidy

builds:
  - id: runny
    main: ./cmd/runny
    binary: runny
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    mod_timestamp: "{{ .CommitTimestamp }}"
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{ .Version }} -X main.commit={{ .Commit }} -X main.date={{ .CommitDate }}

archives:
  - id: runny
    ids:
      - runny
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

homebrew_casks:
  - name: runny
    ids:
      - runny
    binaries:
      - runny
    directory: Casks
    homepage: "https://github.com/saewyn/runny"
    description: "Run one shell command across selected child directories from a TUI"
    license: "MIT"
    caveats: |
      If your Homebrew setup requires trusted taps:
        brew trust --tap saewyn/tap
    repository:
      owner: saewyn
      name: homebrew-tap
      token: "{{ .Env.TAP_GITHUB_TOKEN }}"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com

snapshot:
  version_template: "{{ incpatch .Version }}-next"
```

- [ ] **Step 6: Prepare Homebrew tap prerequisites**

Before first tag release, create or verify the tap repository and secret:

```bash
gh repo view saewyn/homebrew-tap
```

Expected: repository exists. If missing, create it as a public repository named `homebrew-tap`.

```bash
gh secret set TAP_GITHUB_TOKEN --repo saewyn/runny
```

Expected: secret saved in the main `runny` repository. Token must have contents write access to `saewyn/homebrew-tap`.

- [ ] **Step 7: Run formatting, tests, builds, GoReleaser check**

Run:

```bash
gofmt -w cmd internal
go test ./...
go vet ./...
go build ./cmd/runny
goreleaser check
```

Expected: all commands pass. If `goreleaser` is unavailable locally, install it or run the GitHub Action after pushing.

- [ ] **Step 8: Smoke test binary**

Run:

```bash
./runny --version
mkdir -p /tmp/runny-smoke/api
cd /tmp/runny-smoke
/Users/saewyn/Documents/parallelizer\ tui/runny --auto -- pwd
```

Expected: version prints, auto mode prints target-prefixed output for `api`, exit code `0`.

- [ ] **Step 9: Smoke test Homebrew install after a tag release**

After GitHub release and tap update complete:

```bash
brew tap saewyn/tap
brew trust --tap saewyn/tap
brew install --cask runny
runny --version
brew uninstall --cask runny
```

Expected: Homebrew installs `runny`, trusted tap command succeeds, version prints.

- [ ] **Step 10: Commit**

```bash
git add README.md .gitignore .github/workflows/ci.yml .github/workflows/release.yml .goreleaser.yaml
git commit -m "chore: add docs ci and release config"
```

---

## Final Verification

- [ ] Run full test suite:

```bash
go test ./...
```

- [ ] Run static checks:

```bash
go vet ./...
test -z "$(gofmt -l .)"
```

- [ ] Build binary:

```bash
go build ./cmd/runny
```

- [ ] Check release config:

```bash
goreleaser check
```

- [ ] Verify Homebrew tap prerequisites:

```bash
gh repo view saewyn/homebrew-tap
gh secret list --repo saewyn/runny | grep TAP_GITHUB_TOKEN
```

- [ ] Smoke CLI:

```bash
./runny --version
./runny --auto -- pwd
./runny --auto --depth 1 --include api -- pwd
```

Expected final state:

- `runny` builds.
- Tests pass.
- CI and release config exist.
- GoReleaser cask config publishes to `saewyn/homebrew-tap`.
- README documents `brew install --cask`, `brew trust --tap saewyn/tap`, and Brewfile `trusted: true`.
- README documents TUI, `--auto`, config, hidden directory default, logging default, `--disable-logging`, `--save-logs`, and keybindings.
- Git history contains small commits per task.
