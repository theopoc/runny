package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/saewyn/runny/internal/core"
)

func TestRunnyTUIProgramEndToEnd(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		return []core.RunResult{
			{Target: req.Targets[0], Status: core.StatusSucceeded, Output: req.Targets[0].ID + " ok\n"},
		}, nil
	}

	var out bytes.Buffer
	reader, writer := io.Pipe()
	program := tea.NewProgram(
		model,
		tea.WithInput(reader),
		tea.WithOutput(&out),
		tea.WithWindowSize(100, 26),
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
	if _, err := writer.Write([]byte("\r")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		finalModel := result.model
		final := finalModel.(Model)
		if final.Status["api"] != core.StatusSucceeded || final.Status["web"] != core.StatusSucceeded {
			t.Fatalf("statuses = %#v", final.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit")
	}
	_ = writer.Close()
	rendered := stripANSI(out.String())
	for _, want := range []string{"runny", "Directories", "Logs", "succeeded"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered output should contain %q:\n%s", want, rendered)
		}
	}
}
