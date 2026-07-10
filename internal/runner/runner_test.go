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
	"syscall"
	"testing"
	"time"

	"github.com/theopoc/runny/internal/core"
)

const testMaxOutputBytes = 4 << 20

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

func TestRunBoundsOutput(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command: fmt.Sprintf("dd if=/dev/zero bs=%d count=1 2>/dev/null", testMaxOutputBytes+1024),
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusSucceeded {
		t.Fatalf("result = %#v", results[0])
	}
	wantLen := len(truncatedOutputMarker) + testMaxOutputBytes
	if len(results[0].Output) != wantLen {
		t.Fatalf("output length = %d, want %d", len(results[0].Output), wantLen)
	}
	if !strings.HasPrefix(results[0].Output, truncatedOutputMarker) {
		prefix := results[0].Output[:min(len(results[0].Output), len(truncatedOutputMarker))]
		t.Fatalf("output does not start with truncation marker: %q", prefix)
	}
}

func TestRunDisablesCapture(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	results, err := Run(context.Background(), core.RunRequest{
		Command:        "dd if=/dev/zero bs=1048576 count=32 2>/dev/null",
		Targets:        []core.Target{target},
		Mode:           core.ModeSerial,
		DisableLogging: true,
	})
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusSucceeded || results[0].Output != "" {
		t.Fatalf("result = %#v", results[0])
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
