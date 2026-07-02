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
	"github.com/saewyn/runny/internal/runner"
	"github.com/saewyn/runny/internal/tui"
)

var Version = "dev"

type Options struct {
	Args    []string
	WorkDir string
	HomeDir string
	Stdout  io.Writer
	Stderr  io.Writer
}

func Run(opts Options) int {
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.WorkDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(opts.Stderr, err)
			return 1
		}
		opts.WorkDir = wd
	}
	if opts.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(opts.Stderr, err)
			return 1
		}
		opts.HomeDir = home
	}
	parsed, err := cli.Parse(opts.Args)
	if err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return 2
	}
	if parsed.Version {
		fmt.Fprintf(opts.Stdout, "runny %s\n", Version)
		return 0
	}
	if parsed.Help {
		fmt.Fprint(opts.Stdout, helpText())
		return 0
	}
	cfg, err := config.Load(config.LoadOptions{
		HomeDir: opts.HomeDir,
		WorkDir: opts.WorkDir,
		Config:  parsed.Config,
		Flags:   flagOverrides(parsed),
	})
	if err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return 2
	}
	if parsed.Command != "" {
		cfg.Command = parsed.Command
	}
	if parsed.Recursive && parsed.Depth == nil {
		cfg.Depth = 0
	}
	targets, err := discovery.Discover(opts.WorkDir, discovery.Options{
		Recursive:     cfg.Recursive || cfg.Depth == 0 || cfg.Depth > 1,
		Depth:         cfg.Depth,
		IncludeHidden: cfg.IncludeHidden,
		Include:       cfg.Include,
		Exclude:       cfg.Exclude,
	})
	if err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return 1
	}
	if cfg.Auto {
		return runAuto(opts, cfg, targets)
	}
	mode := core.ModeParallel
	if cfg.Serial {
		mode = core.ModeSerial
	}
	if err := tui.Run(tui.Options{
		Command:        cfg.Command,
		Targets:        targets,
		Mode:           mode,
		Workers:        cfg.Workers,
		FailFast:       cfg.FailFast,
		SaveLogs:       cfg.SaveLogs,
		DisableLogging: cfg.DisableLogging,
		LogRoot:        filepath.Join(opts.WorkDir, ".runny", "runs"),
	}); err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return 1
	}
	return 0
}

func runAuto(opts Options, cfg config.Config, targets []core.Target) int {
	mode := core.ModeParallel
	if cfg.Serial {
		mode = core.ModeSerial
	}
	results, err := runner.Run(context.Background(), core.RunRequest{
		Command:        cfg.Command,
		Targets:        targets,
		Mode:           mode,
		Workers:        cfg.Workers,
		FailFast:       cfg.FailFast,
		DisableLogging: cfg.DisableLogging,
		LogRoot:        filepath.Join(opts.WorkDir, ".runny", "runs"),
	})
	if err != nil {
		fmt.Fprintln(opts.Stderr, err)
		return 1
	}
	exit := 0
	for _, result := range results {
		fmt.Fprintf(opts.Stdout, "%s %s\n", result.Target.RelPath, result.Status)
		if result.Status != core.StatusSucceeded {
			exit = 1
		}
	}
	return exit
}

func flagOverrides(parsed cli.Options) config.FlagOverrides {
	return config.FlagOverrides{
		Command:        strPtr(parsed.Command),
		Auto:           boolPtr(parsed.Auto),
		Recursive:      boolPtr(parsed.Recursive),
		Depth:          parsed.Depth,
		IncludeHidden:  boolPtr(parsed.IncludeHidden),
		Include:        parsed.Include,
		Exclude:        parsed.Exclude,
		Serial:         boolPtr(parsed.Serial),
		Workers:        parsed.Workers,
		FailFast:       boolPtr(parsed.FailFast),
		SaveLogs:       boolPtr(parsed.SaveLogs),
		DisableLogging: boolPtr(parsed.DisableLogging),
	}
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func boolPtr(v bool) *bool {
	if !v {
		return nil
	}
	return &v
}

func helpText() string {
	return `runny [flags] -- <command>

Flags:
  -a, --auto              run without TUI
  -c, --config FILE       config file
  -r, --recursive         discover recursively
  -d, --depth N           discovery depth, 0 unlimited
  -H, --include-hidden    include hidden directories
  -i, --include PATTERN   include directories
  -e, --exclude PATTERN   exclude directories
  -s, --serial            run serially
  -w, --workers N         max parallel tasks
  -f, --fail-fast         cancel queued work after first failure
  -L, --save-logs         persist logs
  -N, --disable-logging   disable log capture
  -v, --version           print version
`
}
