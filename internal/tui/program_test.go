package tui

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
	runpkg "github.com/theopoc/runny/internal/run"
)

type synchronizedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *synchronizedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestRunnyTUIProgramEndToEnd(t *testing.T) {
	targets := []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}
	run := &fakeActiveRun{events: []runpkg.Event{
		targetEvent(runpkg.EventTargetQueued, targets[0], core.StatusQueued, "", ""),
		targetEvent(runpkg.EventTargetQueued, targets[1], core.StatusQueued, "", ""),
		targetEvent(runpkg.EventTargetStarted, targets[0], core.StatusRunning, "", ""),
		targetEvent(runpkg.EventTargetFinished, targets[0], core.StatusSucceeded, "api ok\n", ""),
		targetEvent(runpkg.EventTargetStarted, targets[1], core.StatusRunning, "", ""),
		targetEvent(runpkg.EventTargetFinished, targets[1], core.StatusSucceeded, "web ok\n", ""),
		completedEvent("run-1", "echo ok",
			runpkg.TargetSnapshot{Target: targets[0], Status: core.StatusSucceeded, OutputTail: "api ok\n"},
			runpkg.TargetSnapshot{Target: targets[1], Status: core.StatusSucceeded, OutputTail: "web ok\n"},
		),
	}}
	model := NewModel(Options{
		Command:  "echo ok",
		Targets:  targets,
		startRun: fakeStart(run, nil),
	})

	var screen synchronizedBuffer
	completed := make(chan struct{})
	var completedOnce sync.Once
	reader, writer := io.Pipe()
	defer writer.Close()
	program := tea.NewProgram(
		model,
		tea.WithInput(reader),
		tea.WithOutput(&screen),
		tea.WithWindowSize(120, 26),
		tea.WithoutSignals(),
		tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
			if next, ok := msg.(runEventMsg); ok && next.event.Kind == runpkg.EventCompleted {
				completedOnce.Do(func() { close(completed) })
			}
			return msg
		}),
	)
	done := make(chan struct {
		model tea.Model
		err   error
	}, 1)
	go func() {
		finalModel, err := program.Run()
		done <- struct {
			model tea.Model
			err   error
		}{model: finalModel, err: err}
	}()
	program.Send(tea.KeyPressMsg(tea.Key{Code: '?'}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: '/'}))
	program.Send(tea.KeyPressMsg(tea.Key{Text: "w"}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	for _, char := range "workers 1" {
		program.Send(tea.KeyPressMsg(tea.Key{Text: string(char)}))
	}
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))

	select {
	case <-completed:
	case <-time.After(3 * time.Second):
		t.Fatal("program did not process run completion")
	}
	program.Send(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	program.Send(tea.KeyPressMsg(tea.Key{Text: "y"}))

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		final := result.model.(Model)
		if final.Workers != 1 {
			t.Fatalf("workers = %d, want 1", final.Workers)
		}
		if final.Filter != "" {
			t.Fatalf("filter = %q, want cleared by escape", final.Filter)
		}
		if final.Status["api"] != core.StatusSucceeded || final.Status["web"] != core.StatusSucceeded {
			t.Fatalf("statuses = %#v", final.Status)
		}
		if rendered := stripANSI(final.View().Content); !strings.Contains(rendered, "2 ok") {
			t.Fatalf("rendered output missing completion:\n%s", rendered)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit")
	}
}

func TestRunProgramDeliversLiveOutputBeforeTargetCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var screen synchronizedBuffer
	reader, writer := io.Pipe()
	defer writer.Close()
	opts := Options{
		Command: "printf 'live now\\n'; sleep 20",
		Targets: []core.Target{{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}},
		programOptions: []tea.ProgramOption{
			tea.WithInput(reader),
			tea.WithOutput(&screen),
			tea.WithWindowSize(120, 26),
		},
	}

	done := make(chan error, 1)
	go func() { done <- runProgram(ctx, opts) }()
	if _, err := writer.Write([]byte{'\r'}); err != nil {
		t.Fatalf("write enter: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		content := screen.String()
		if strings.Contains(content, "live now") && strings.Contains(content, "1 running") {
			break
		}
		time.Sleep(time.Millisecond)
	}
	content := screen.String()
	if !strings.Contains(content, "live now") || !strings.Contains(content, "1 running") {
		t.Fatalf("live output missing before completion:\n%s", content)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not stop")
	}
}

func TestRunProgramWaitsForProcessCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	targetDir := t.TempDir()
	pidPath := filepath.Join(targetDir, "shell.pid")
	reader, writer := io.Pipe()
	defer writer.Close()
	opts := Options{
		Command: "echo $$ > shell.pid; sleep 20",
		Targets: []core.Target{{ID: "api", RelPath: "api", AbsPath: targetDir, Selected: true}},
		programOptions: []tea.ProgramOption{
			tea.WithInput(reader),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(120, 26),
		},
	}

	done := make(chan error, 1)
	go func() { done <- runProgram(ctx, opts) }()
	if _, err := writer.Write([]byte{'\r'}); err != nil {
		t.Fatal(err)
	}
	var pid int
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidPath)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("shell pid was not written")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runProgram did not wait for cleanup")
	}
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("shell process %d still alive after runProgram returned", pid)
	}
}
