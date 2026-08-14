package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/theopoc/runny/internal/core"
)

type closeErrorStore struct {
	err error
}

func (s closeErrorStore) Close() error {
	return s.err
}

func TestCloseLogStoreReportsCloseError(t *testing.T) {
	runErr := errors.New("run failed")
	closeErr := errors.New("close failed")
	err := closeLogStore(closeErrorStore{err: closeErr}, runErr)
	if !errors.Is(err, runErr) || !errors.Is(err, closeErr) {
		t.Fatalf("closeLogStore() error = %v, want joined run and close errors", err)
	}
	if !strings.Contains(err.Error(), "closing log store:") {
		t.Fatalf("closeLogStore() error = %q, want close context", err)
	}
}

func TestRunnerRunsCommandInTargetDirectory(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command: "pwd",
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != core.StatusSucceeded {
		t.Fatalf("results = %#v", results)
	}
}

func TestRunnerMarksFailure(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command: "exit 7",
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusFailed || results[0].ExitCode != 7 {
		t.Fatalf("result = %#v", results[0])
	}
}

func TestRunStreamsStdoutAndStderrBeforeCommandFinishes(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	events := make(chan core.Event, 4)
	type outcome struct {
		results []core.RunResult
		err     error
	}
	done := make(chan outcome, 1)

	go func() {
		results, err := Run(context.Background(), core.RunRequest{
			Command: "printf 'stdout-before-finish\\n'; sleep 1; printf 'stderr-before-finish\\n' >&2",
			Targets: []core.Target{target},
			Mode:    core.ModeSerial,
			OnEvent: func(event core.Event) {
				events <- event
			},
		})
		done <- outcome{results: results, err: err}
	}()

	select {
	case event := <-events:
		if event.Type != core.EventOutput || event.TargetID != target.ID || event.Output != "stdout-before-finish\n" {
			t.Fatalf("first event = %#v", event)
		}
	case <-done:
		t.Fatal("command finished before first output event")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first output event was not delivered while command was running")
	}

	select {
	case result := <-done:
		t.Fatalf("command finished before delayed stderr was observed: %#v", result)
	case <-time.After(100 * time.Millisecond):
	}

	var streamed strings.Builder
	streamed.WriteString("stdout-before-finish\n")
	for !strings.Contains(streamed.String(), "stderr-before-finish\n") {
		select {
		case event := <-events:
			if event.Type != core.EventOutput || event.TargetID != target.ID {
				t.Fatalf("event = %#v", event)
			}
			streamed.WriteString(event.Output)
		case <-time.After(2 * time.Second):
			t.Fatalf("stderr event was not delivered: %q", streamed.String())
		}
	}

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if len(result.results) != 1 || result.results[0].Output != streamed.String() {
			t.Fatalf("results = %#v, streamed = %q", result.results, streamed.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command did not finish")
	}
}

func TestEventWriterSerializesConcurrentOutput(t *testing.T) {
	const writers = 16
	var active atomic.Int32
	var concurrent atomic.Bool
	var eventCount atomic.Int32
	writer := &eventWriter{
		target:  core.Target{ID: "api"},
		capture: newTailBuffer(writers),
		onEvent: func(core.Event) {
			if active.Add(1) > 1 {
				concurrent.Store(true)
			}
			time.Sleep(2 * time.Millisecond)
			active.Add(-1)
			eventCount.Add(1)
		},
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := writer.Write([]byte("x")); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if concurrent.Load() {
		t.Fatal("output event sink was called concurrently")
	}
	if eventCount.Load() != writers {
		t.Fatalf("event count = %d, want %d", eventCount.Load(), writers)
	}
}

func TestRunBoundsOutput(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command: fmt.Sprintf("dd if=/dev/zero bs=%d count=1 2>/dev/null", MaxOutputBytes+1024),
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusSucceeded {
		t.Fatalf("result = %#v", results[0])
	}
	wantLen := len(TruncatedOutputMarker) + MaxOutputBytes
	if len(results[0].Output) != wantLen {
		t.Fatalf("output length = %d, want %d", len(results[0].Output), wantLen)
	}
	if !strings.HasPrefix(results[0].Output, TruncatedOutputMarker) {
		prefix := results[0].Output[:min(len(results[0].Output), len(TruncatedOutputMarker))]
		t.Fatalf("output does not start with truncation marker: %q", prefix)
	}
}

func TestRunDisablesCapture(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	eventCount := 0
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	results, err := Run(context.Background(), core.RunRequest{
		Command:        "dd if=/dev/zero bs=1048576 count=32 2>/dev/null",
		Targets:        []core.Target{target},
		Mode:           core.ModeSerial,
		DisableLogging: true,
		OnEvent: func(core.Event) {
			eventCount++
		},
	})
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusSucceeded || results[0].Output != "" {
		t.Fatalf("result = %#v", results[0])
	}
	if eventCount != 0 {
		t.Fatalf("disabled logging emitted %d output events", eventCount)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Fatalf("disabled logging allocated %d bytes, want no output-sized capture", allocated)
	}
}

func TestRunSurfacesLogWriteFailure(t *testing.T) {
	logRoot := filepath.Join(t.TempDir(), "run")
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command:  fmt.Sprintf("rm -rf %q; printf blocked > %q; echo hello", logRoot, logRoot),
		Targets:  []core.Target{target},
		Mode:     core.ModeSerial,
		SaveLogs: true,
		LogRoot:  logRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusFailed {
		t.Fatalf("status = %q, want %q", results[0].Status, core.StatusFailed)
	}
	if !strings.Contains(results[0].Error, "saving log:") {
		t.Fatalf("error = %q, want saving log context", results[0].Error)
	}
}

func TestRunSurfacesSilentLogWriteFailure(t *testing.T) {
	logRoot := filepath.Join(t.TempDir(), "run")
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command:  fmt.Sprintf("rm -rf %q; : > %q", logRoot, logRoot),
		Targets:  []core.Target{target},
		Mode:     core.ModeSerial,
		SaveLogs: true,
		LogRoot:  logRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusFailed {
		t.Fatalf("status = %q, want %q", results[0].Status, core.StatusFailed)
	}
	if !strings.Contains(results[0].Error, "saving log:") {
		t.Fatalf("error = %q, want saving log context", results[0].Error)
	}
}

func TestRunJoinsCommandAndLogWriteFailures(t *testing.T) {
	logRoot := filepath.Join(t.TempDir(), "run")
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command:  fmt.Sprintf("rm -rf %q; printf blocked > %q; echo hello; exit 7", logRoot, logRoot),
		Targets:  []core.Target{target},
		Mode:     core.ModeSerial,
		SaveLogs: true,
		LogRoot:  logRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	result := results[0]
	if result.Status != core.StatusFailed || result.ExitCode != 7 {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Error, "exit status 7") || !strings.Contains(result.Error, "saving log:") {
		t.Fatalf("error = %q, want command and saving log errors", result.Error)
	}
}

func TestRunnerRespectsLoggingOptions(t *testing.T) {
	target := core.Target{ID: "api/service", RelPath: "api/service", AbsPath: t.TempDir(), Selected: true}
	worker := core.Target{ID: "worker", RelPath: "worker", AbsPath: t.TempDir(), Selected: true}
	logRoot := t.TempDir()
	results, err := Run(context.Background(), core.RunRequest{
		Command:  "echo hello",
		Targets:  []core.Target{target, worker},
		Mode:     core.ModeSerial,
		SaveLogs: true,
		LogRoot:  logRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Output != "hello\n" || results[1].Output != "hello\n" {
		t.Fatalf("results = %#v", results)
	}
	for _, path := range []string{
		filepath.Join(logRoot, "api", "service.log"),
		filepath.Join(logRoot, "worker.log"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello\n" {
			t.Fatalf("saved log %q = %q", path, data)
		}
	}

	results, err = Run(context.Background(), core.RunRequest{
		Command:        "echo hidden",
		Targets:        []core.Target{target},
		Mode:           core.ModeSerial,
		DisableLogging: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Output != "" {
		t.Fatalf("disabled output = %q", results[0].Output)
	}
}

func TestRunnerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(ctx, core.RunRequest{
		Command: "sleep 1",
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusCancelled {
		t.Fatalf("result = %#v", results[0])
	}
}

func TestRunnerLogWriteFailureOverridesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logRoot := filepath.Join(t.TempDir(), "run")
	targetDir := t.TempDir()
	started := filepath.Join(targetDir, "started")
	target := core.Target{ID: "api", RelPath: "api", AbsPath: targetDir, Selected: true}
	type runOutcome struct {
		results []core.RunResult
		err     error
	}
	done := make(chan runOutcome, 1)
	go func() {
		results, err := Run(ctx, core.RunRequest{
			Command:  fmt.Sprintf("mkdir %q; touch %q; sleep 20", filepath.Join(logRoot, "api.log"), started),
			Targets:  []core.Target{target},
			Mode:     core.ModeSerial,
			SaveLogs: true,
			LogRoot:  logRoot,
		})
		done <- runOutcome{results: results, err: err}
	}()

	for range 50 {
		if _, err := os.Stat(started); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(started); err != nil {
		t.Fatal("command did not start")
	}
	cancel()

	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		if len(outcome.results) != 1 || outcome.results[0].Status != core.StatusFailed {
			t.Fatalf("results = %#v, want failed persistence", outcome.results)
		}
		for _, want := range []string{context.Canceled.Error(), "saving log:"} {
			if !strings.Contains(outcome.results[0].Error, want) {
				t.Fatalf("error = %q, want %q", outcome.results[0].Error, want)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not cancel")
	}
}

func TestRunnerCancellationKillsChildProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are unix-specific")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	target := core.Target{ID: "api", RelPath: "api", AbsPath: dir, Selected: true}
	done := make(chan []core.RunResult, 1)
	go func() {
		results, err := Run(ctx, core.RunRequest{
			Command: "sleep 20 & echo $! > child.pid; wait",
			Targets: []core.Target{target},
			Mode:    core.ModeSerial,
		})
		if err != nil {
			t.Errorf("run: %v", err)
		}
		done <- results
	}()
	var pid int
	for range 50 {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			pid = parsed
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("child pid was not written")
	}
	cancel()
	select {
	case results := <-done:
		if len(results) != 1 || results[0].Status != core.StatusCancelled {
			t.Fatalf("results = %#v", results)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not cancel")
	}
	time.Sleep(100 * time.Millisecond)
	if processAlive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("child process %d still alive", pid)
	}
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}
