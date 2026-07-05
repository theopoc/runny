package tui

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
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
	defer writer.Close()
	program := tea.NewProgram(
		model,
		tea.WithInput(reader),
		tea.WithOutput(&out),
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
	time.Sleep(100 * time.Millisecond)
	program.Send(tea.KeyPressMsg(tea.Key{Code: 'q'}))
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		finalModel := result.model
		final := finalModel.(Model)
		if final.Workers != 1 {
			t.Fatalf("workers = %d, want 1", final.Workers)
		}
		if final.Status["api"] != core.StatusSucceeded || final.Status["web"] != core.StatusSucceeded {
			t.Fatalf("statuses = %#v", final.Status)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("program did not quit")
	}
	rendered := stripANSI(out.String())
	for _, want := range []string{"runny", "Tasks", "Output", "workers 1", "succeeded"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered output should contain %q:\n%s", want, rendered)
		}
	}
}
