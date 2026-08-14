package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/logs"
)

type logStoreCloser interface {
	Close() error
}

func Run(ctx context.Context, req core.RunRequest) (results []core.RunResult, err error) {
	if req.Command == "" {
		return nil, errors.New("command is required")
	}
	targets := core.SelectedTargets(req.Targets)
	if len(targets) == 0 {
		return nil, errors.New("no selected targets")
	}
	results = make([]core.RunResult, len(targets))
	logStore, err := logs.NewStore(logs.Options{Root: req.LogRoot, Save: req.SaveLogs, Disabled: req.DisableLogging})
	if err != nil {
		return nil, err
	}
	defer func() {
		err = closeLogStore(logStore, err)
	}()
	if ctx.Err() != nil {
		for i, target := range targets {
			results[i] = cancelledResult(target)
		}
		return results, nil
	}
	workers := req.Workers
	if req.Mode == core.ModeSerial {
		workers = 1
	}
	if workers <= 0 {
		workers = min(runtime.NumCPU(), len(targets))
	}
	jobs := make(chan int)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				result := runOne(runCtx, req.Command, targets[idx], logStore, req.DisableLogging, req.OnEvent)
				results[idx] = result
				if req.FailFast && result.Status == core.StatusFailed {
					cancel()
				}
			}
		}()
	}
	for i := range targets {
		if runCtx.Err() != nil {
			results[i] = cancelledResult(targets[i])
			continue
		}
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	for i, result := range results {
		if result.Target.ID == "" {
			results[i] = cancelledResult(targets[i])
		}
	}
	return results, nil
}

func closeLogStore(store logStoreCloser, runErr error) error {
	closeErr := store.Close()
	if closeErr == nil {
		return runErr
	}
	return errors.Join(runErr, fmt.Errorf("closing log store: %w", closeErr))
}

func runOne(
	ctx context.Context,
	command string,
	target core.Target,
	logStore *logs.Store,
	disableLogging bool,
	onEvent func(core.Event),
) core.RunResult {
	started := time.Now()
	if ctx.Err() != nil {
		return cancelledResult(target)
	}
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = target.AbsPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var capture *tailBuffer
	var outputWriter io.Writer = io.Discard
	if !disableLogging {
		capture = newTailBuffer(MaxOutputBytes)
		outputWriter = &eventWriter{target: target, capture: capture, onEvent: onEvent}
	}
	cmd.Stdout = outputWriter
	cmd.Stderr = outputWriter
	err := cmd.Start()
	if err == nil {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case err = <-done:
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				_ = cmd.Process.Kill()
			}
			err = <-done
		}
	}
	ended := time.Now()
	var output string
	if capture != nil {
		output = capture.String()
	}
	var saveErr error
	if logStore != nil {
		if appendErr := logStore.Append(target.ID, output); appendErr != nil {
			saveErr = fmt.Errorf("saving log: %w", appendErr)
		}
	}
	result := core.RunResult{Target: target, Started: started, Ended: ended, Output: output}
	if saveErr != nil {
		result.Status = core.StatusFailed
		result.Error = errors.Join(err, ctx.Err(), saveErr).Error()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
		return result
	}
	if ctx.Err() != nil {
		result.Status = core.StatusCancelled
		result.Error = ctx.Err().Error()
		return result
	}
	if err == nil {
		result.Status = core.StatusSucceeded
		return result
	}
	result.Status = core.StatusFailed
	result.Error = err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	}
	return result
}

type eventWriter struct {
	mu      sync.Mutex
	target  core.Target
	capture io.Writer
	onEvent func(core.Event)
}

func (w *eventWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.capture.Write(p)
	if n > 0 && w.onEvent != nil {
		w.onEvent(core.Event{
			Type:     core.EventOutput,
			TargetID: w.target.ID,
			Target:   w.target,
			Output:   string(p[:n]),
			Time:     time.Now(),
		})
	}
	return n, err
}

func cancelledResult(target core.Target) core.RunResult {
	now := time.Now()
	return core.RunResult{
		Target:  target,
		Status:  core.StatusCancelled,
		Started: now,
		Ended:   now,
		Error:   context.Canceled.Error(),
	}
}
