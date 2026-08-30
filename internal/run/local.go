package run

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/theopoc/runny/internal/history"
	"github.com/theopoc/runny/internal/logs"
	"github.com/theopoc/runny/internal/runner"
)

// LocalOptions configures filesystem adapters for locally executed Runs.
type LocalOptions struct {
	CommandHistoryPath string
	RunHistoryPath     string
	LogRoot            string
}

// Runtime owns local Run goroutines across application shutdown.
type Runtime struct {
	ctx    context.Context
	cancel context.CancelFunc
	deps   dependencies

	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

// NewLocal wires PTY execution and legacy JSONL/log persistence.
func NewLocal(opts LocalOptions) *Runtime {
	ctx, cancel := context.WithCancel(context.Background())
	var idMu sync.Mutex
	var lastTimestamp time.Time
	var persistenceMu sync.Mutex
	deps := dependencies{
		execute: func(ctx context.Context, req executionRequest) executionOutcome {
			outcome := runner.Execute(ctx, runner.Request{
				Command:  req.Command,
				Target:   req.Target,
				OnOutput: req.OnOutput,
			})
			return executionOutcome(outcome)
		},
		now: time.Now,
		newID: func(now time.Time) (ID, error) {
			idMu.Lock()
			defer idMu.Unlock()
			if !now.After(lastTimestamp) {
				now = lastTimestamp.Add(time.Nanosecond)
			}
			lastTimestamp = now
			return ID(now.UTC().Format("20060102T150405.000000000Z")), nil
		},
	}
	if opts.CommandHistoryPath != "" {
		deps.recordCommand = func(command string, accepted time.Time) error {
			persistenceMu.Lock()
			defer persistenceMu.Unlock()
			return history.AppendCommand(opts.CommandHistoryPath, history.CommandEntry{
				Command: command,
				Time:    accepted,
			})
		}
	}
	archive := localArchive(opts)
	deps.archive = func(ctx context.Context, snapshot Snapshot, spec Spec) error {
		persistenceMu.Lock()
		defer persistenceMu.Unlock()
		return archive(ctx, snapshot, spec)
	}
	return &Runtime{ctx: ctx, cancel: cancel, deps: deps}
}

// Start accepts one local Run.
func (rt *Runtime) Start(ctx context.Context, spec Spec) (*Run, error) {
	if ctx == nil {
		return nil, errors.New("run context is required")
	}
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		return nil, errors.New("run runtime is closed")
	}
	rt.wg.Add(1)
	rt.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	stopRuntimeCancel := context.AfterFunc(rt.ctx, cancel)
	r, err := start(runCtx, spec, rt.deps, func() {
		stopRuntimeCancel()
		cancel()
		rt.wg.Done()
	})
	if err != nil {
		stopRuntimeCancel()
		cancel()
		rt.wg.Done()
		return nil, err
	}
	return r, nil
}

// CloseAndWait rejects new Runs, cancels active Runs, and waits for cleanup.
func (rt *Runtime) CloseAndWait() {
	rt.mu.Lock()
	if !rt.closed {
		rt.closed = true
		rt.cancel()
	}
	rt.mu.Unlock()
	rt.wg.Wait()
}

func localArchive(opts LocalOptions) archiveFunc {
	return func(_ context.Context, snapshot Snapshot, spec Spec) error {
		var archiveErr error
		logID := ""
		if spec.SaveLogs && !spec.DisableLogging {
			logID = string(snapshot.ID)
			store, err := logs.NewStore(logs.Options{
				Root: filepath.Join(opts.LogRoot, logID),
				Save: true,
			})
			if err != nil {
				archiveErr = errors.Join(archiveErr, fmt.Errorf("opening log store: %w", err))
			} else {
				for _, target := range snapshot.Targets {
					if err := store.Append(target.Target.ID, target.OutputTail); err != nil {
						archiveErr = errors.Join(archiveErr, fmt.Errorf("saving log for %s: %w", target.Target.ID, err))
					}
				}
				if err := store.Close(); err != nil {
					archiveErr = errors.Join(archiveErr, fmt.Errorf("closing log store: %w", err))
				}
			}
		}
		if opts.RunHistoryPath != "" {
			if err := history.AppendRun(opts.RunHistoryPath, historyEntry(snapshot, logID)); err != nil {
				archiveErr = errors.Join(archiveErr, fmt.Errorf("saving run history: %w", err))
			}
		}
		return archiveErr
	}
}

func historyEntry(snapshot Snapshot, logID string) history.RunEntry {
	entry := history.RunEntry{
		Command:   snapshot.Command,
		Total:     snapshot.Total,
		Succeeded: snapshot.Succeeded,
		Failed:    snapshot.Failed,
		Cancelled: snapshot.Cancelled,
		Time:      snapshot.Ended,
		Started:   snapshot.Started,
		Ended:     snapshot.Ended,
		LogID:     logID,
		Targets:   make([]history.TargetEntry, 0, len(snapshot.Targets)),
	}
	for _, target := range snapshot.Targets {
		entry.Targets = append(entry.Targets, history.TargetEntry{
			ID:       target.Target.ID,
			RelPath:  target.Target.RelPath,
			Status:   target.Status,
			ExitCode: target.ExitCode,
			Error:    target.Error,
			Started:  target.Started,
			Ended:    target.Ended,
		})
	}
	return entry
}
