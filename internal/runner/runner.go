package runner

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/saewyn/runny/internal/core"
	"github.com/saewyn/runny/internal/logs"
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
	logRoot := req.LogRoot
	if req.SaveLogs && !req.DisableLogging {
		logRoot = filepath.Join(req.LogRoot, time.Now().UTC().Format("20060102T150405.000000000Z"))
	}
	logStore, err := logs.NewStore(logs.Options{Root: logRoot, Save: req.SaveLogs, Disabled: req.DisableLogging})
	if err != nil {
		return nil, err
	}
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
				result := runOne(runCtx, req.Command, targets[idx], logStore, req.DisableLogging)
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

func runOne(ctx context.Context, command string, target core.Target, logStore *logs.Store, disableLogging bool) core.RunResult {
	started := time.Now()
	if ctx.Err() != nil {
		return cancelledResult(target)
	}
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = target.AbsPath
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
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
	output := out.String()
	if output != "" && logStore != nil {
		_ = logStore.Append(target.ID, output)
	}
	if disableLogging {
		output = ""
	}
	result := core.RunResult{Target: target, Started: started, Ended: ended, Output: output}
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
