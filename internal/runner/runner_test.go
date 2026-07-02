package runner

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/saewyn/runny/internal/core"
)

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
