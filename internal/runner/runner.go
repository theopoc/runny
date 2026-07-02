package runner

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/saewyn/runny/internal/core"
)

func Run(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
	if req.Command == "" {
		return nil, errors.New("command is required")
	}
	targets := core.SelectedTargets(req.Targets)
	if len(targets) == 0 {
		return nil, errors.New("no selected targets")
	}
	results := make([]core.RunResult, len(targets))
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
				result := runOne(runCtx, req.Command, targets[idx])
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

func runOne(ctx context.Context, command string, target core.Target) core.RunResult {
	started := time.Now()
	if ctx.Err() != nil {
		return cancelledResult(target)
	}
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = target.AbsPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out, err := cmd.CombinedOutput()
	ended := time.Now()
	result := core.RunResult{Target: target, Started: started, Ended: ended, Output: string(out)}
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

func cancelledResult(target core.Target) core.RunResult {
	now := time.Now()
	return core.RunResult{Target: target, Status: core.StatusCancelled, Started: now, Ended: now, Error: context.Canceled.Error()}
}
