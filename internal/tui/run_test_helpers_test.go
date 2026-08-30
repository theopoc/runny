package tui

import (
	"context"
	"io"
	"sync"

	"github.com/theopoc/runny/internal/core"
	runpkg "github.com/theopoc/runny/internal/run"
)

type fakeActiveRun struct {
	mu            sync.Mutex
	events        []runpkg.Event
	cancelResults [][]string
	cancelCalls   int
}

func (r *fakeActiveRun) Next(context.Context) (runpkg.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) == 0 {
		return runpkg.Event{}, io.EOF
	}
	event := r.events[0]
	r.events = r.events[1:]
	return event, nil
}

func (r *fakeActiveRun) Cancel(runpkg.CancelScope) runpkg.Cancellation {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := r.cancelCalls
	r.cancelCalls++
	if index >= len(r.cancelResults) {
		return runpkg.Cancellation{}
	}
	return runpkg.Cancellation{Accepted: append([]string(nil), r.cancelResults[index]...)}
}

func fakeStart(run *fakeActiveRun, captured *runpkg.Spec) startRunFunc {
	return func(_ context.Context, spec runpkg.Spec) (activeRun, error) {
		if captured != nil {
			*captured = spec
		}
		return run, nil
	}
}

func targetEvent(kind runpkg.EventKind, target core.Target, status core.Status, output, targetError string) runpkg.Event {
	return runpkg.Event{
		Kind: kind,
		Target: &runpkg.TargetSnapshot{
			Target:     target,
			Status:     status,
			OutputTail: output,
			Error:      targetError,
		},
	}
}

func completedEvent(id string, command string, targets ...runpkg.TargetSnapshot) runpkg.Event {
	snapshot := runpkg.Snapshot{
		ID:      runpkg.ID(id),
		Command: command,
		Total:   len(targets),
		Targets: targets,
	}
	for _, target := range targets {
		switch target.Status {
		case core.StatusSucceeded:
			snapshot.Succeeded++
		case core.StatusFailed:
			snapshot.Failed++
		case core.StatusCancelled:
			snapshot.Cancelled++
		}
	}
	return runpkg.Event{Kind: runpkg.EventCompleted, Run: snapshot}
}
