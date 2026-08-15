package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
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
	runHistoryPath := filepath.Join(t.TempDir(), "runs.jsonl")
	model := NewModel(Options{
		Command:        "echo ok",
		RunHistoryPath: runHistoryPath,
		Targets: []core.Target{
			{ID: "api", RelPath: "api", Selected: true},
			{ID: "web", RelPath: "web", Selected: true},
		},
	})
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		return []core.RunResult{
			{Target: req.Targets[0], Status: core.StatusSucceeded, Output: req.Targets[0].ID + " ok\n"},
		}, nil
	}

	reader, writer := io.Pipe()
	defer writer.Close()
	program := tea.NewProgram(
		model,
		tea.WithInput(reader),
		tea.WithOutput(io.Discard),
		tea.WithWindowSize(120, 26),
		tea.WithoutSignals(),
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
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	program.Send(tea.KeyPressMsg(tea.Key{Text: ":"}))
	for _, char := range "workers 1" {
		program.Send(tea.KeyPressMsg(tea.Key{Text: string(char)}))
	}
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: ' '}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: 'a'}))
	program.Send(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	waitForCompletedRun(t, runHistoryPath, 2)
	program.Send(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	program.Send(tea.KeyPressMsg(tea.Key{Text: "y"}))
	var final Model
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		final = result.model.(Model)
		if final.Workers != 1 {
			t.Fatalf("workers = %d, want 1", final.Workers)
		}
		if final.Status["api"] != core.StatusSucceeded || final.Status["web"] != core.StatusSucceeded {
			t.Fatalf("statuses = %#v", final.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit")
	}
	rendered := stripANSI(final.View().Content)
	for _, want := range []string{"runny", "Tasks", "parallel×1", "2 ok"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered output should contain %q:\n%s", want, rendered)
		}
	}
}

func TestRunProgramDeliversLiveOutputBeforeTargetCompletes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release := make(chan struct{})
	var screen synchronizedBuffer
	reader, writer := io.Pipe()
	defer writer.Close()
	opts := Options{
		Command: "echo live",
		Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}},
		runFunc: func(_ context.Context, req core.RunRequest) ([]core.RunResult, error) {
			req.OnEvent(core.Event{Type: core.EventOutput, TargetID: "api", Output: "live now\n"})
			<-release
			return []core.RunResult{{Target: req.Targets[0], Status: core.StatusSucceeded, Output: "live now\n"}}, nil
		},
		programOptions: []tea.ProgramOption{
			tea.WithInput(reader),
			tea.WithOutput(&screen),
			tea.WithWindowSize(120, 26),
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- runProgram(ctx, opts)
	}()
	if _, err := writer.Write([]byte{'\r'}); err != nil {
		t.Fatalf("write enter: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(screen.String(), "live now") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(screen.String(), "live now") {
		t.Fatalf("program did not render live output before target completion:\n%s", screen.String())
	}

	close(release)
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

func waitForCompletedRun(t *testing.T, path string, total int) {
	t.Helper()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(3 * time.Second)
	defer timeout.Stop()
	for {
		runs, err := history.ReadRuns(path)
		if err == nil && len(runs) == 1 && runs[0].Total == total {
			return
		}
		select {
		case <-ticker.C:
		case <-timeout.C:
			t.Fatalf("run history did not reach total %d: runs=%#v err=%v", total, runs, err)
		}
	}
}

func TestRunProgramWaitsForSignalCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	cancelled := make(chan struct{})
	release := make(chan struct{})
	reader, writer := io.Pipe()
	defer writer.Close()
	opts := Options{
		Command: "echo ok",
		Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}},
		runFunc: func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
			close(started)
			<-ctx.Done()
			close(cancelled)
			<-release
			return []core.RunResult{{Target: req.Targets[0], Status: core.StatusCancelled}}, nil
		},
		programOptions: []tea.ProgramOption{
			tea.WithInput(reader),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(120, 26),
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- runProgram(ctx, opts)
	}()
	writeDone := make(chan error, 1)
	go func() {
		_, err := writer.Write([]byte{'\r'})
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("write enter: %v", err)
		}
	case err := <-done:
		t.Fatalf("runProgram returned before consuming input: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("program did not consume input")
	}
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("runProgram returned before run started: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("run did not start")
	}

	cancel()
	select {
	case <-cancelled:
	case err := <-done:
		t.Fatalf("runProgram returned before runner observed cancellation: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not observe cancellation")
	}
	select {
	case err := <-done:
		t.Fatalf("runProgram returned before runner cleanup completed: %v", err)
	default:
	}

	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runProgram did not wait for runner cleanup")
	}
}

func TestRunProgramCancelsActiveRunWhenProgramExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	started := make(chan struct{})
	cancelled := make(chan struct{})
	reader, writer := io.Pipe()
	opts := Options{
		Command: "echo ok",
		Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}},
		runFunc: func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
			close(started)
			<-ctx.Done()
			close(cancelled)
			return []core.RunResult{{Target: req.Targets[0], Status: core.StatusCancelled}}, nil
		},
		programOptions: []tea.ProgramOption{
			tea.WithInput(reader),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(120, 26),
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- runProgram(ctx, opts)
	}()
	if _, err := writer.Write([]byte{'\r'}); err != nil {
		t.Fatalf("write enter: %v", err)
	}
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("runProgram returned before run started: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("run did not start")
	}

	if err := writer.CloseWithError(io.ErrUnexpectedEOF); err != nil {
		t.Fatalf("close input: %v", err)
	}
	select {
	case <-cancelled:
	case err := <-done:
		t.Fatalf("runProgram returned before runner observed cancellation: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("runner did not observe cancellation after program exit")
	}
	select {
	case err := <-done:
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("runProgram error = %v, want unexpected EOF", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runProgram did not return after runner cancellation")
	}
}

func TestRunProgramImmediateShutdownSkipsUnstartedCommands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batchDropped := make(chan struct{}, 1)
	started := make(chan struct{}, 2)
	reader, writer := io.Pipe()
	defer writer.Close()
	opts := Options{
		Command: "echo ok",
		Workers: 2,
		Targets: []core.Target{
			{ID: "api", RelPath: "api", Selected: true},
			{ID: "web", RelPath: "web", Selected: true},
		},
		runFunc: func(context.Context, core.RunRequest) ([]core.RunResult, error) {
			started <- struct{}{}
			return nil, nil
		},
		programOptions: []tea.ProgramOption{
			tea.WithInput(reader),
			tea.WithOutput(io.Discard),
			tea.WithWindowSize(120, 26),
			tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
				if _, ok := msg.(tea.BatchMsg); ok {
					batchDropped <- struct{}{}
					return nil
				}
				return msg
			}),
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- runProgram(ctx, opts)
	}()
	writeDone := make(chan error, 1)
	go func() {
		_, err := writer.Write([]byte{'\r'})
		writeDone <- err
	}()

	select {
	case err := <-writeDone:
		if err != nil {
			t.Fatalf("write enter: %v", err)
		}
	case err := <-done:
		t.Fatalf("runProgram returned before consuming input: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("program did not consume input")
	}
	select {
	case <-batchDropped:
	case err := <-done:
		t.Fatalf("runProgram returned before scheduling batch: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("program did not schedule target batch")
	}
	select {
	case <-started:
		t.Fatal("runFunc should not start before immediate shutdown")
	default:
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runProgram waited for commands Bubble Tea never started")
	}
	select {
	case <-started:
		t.Fatal("runFunc should remain unstarted after shutdown")
	default:
	}
}
