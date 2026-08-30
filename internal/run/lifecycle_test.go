package run

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/theopoc/runny/internal/core"
)

func testTargets(ids ...string) []core.Target {
	targets := make([]core.Target, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, core.Target{ID: id, RelPath: id, AbsPath: "/tmp/" + id})
	}
	return targets
}

func testDependencies(execute executeFunc) dependencies {
	return dependencies{
		execute: execute,
		now:     time.Now,
		newID: func(time.Time) (ID, error) {
			return ID("test-run"), nil
		},
	}
}

func startTestRun(t *testing.T, spec Spec, deps dependencies) *Run {
	t.Helper()
	r, err := start(context.Background(), spec, deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func collectEvents(t *testing.T, r *Run) []Event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var events []Event
	for {
		event, err := r.Next(ctx)
		if errors.Is(err, io.EOF) {
			return events
		}
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, event)
	}
}

func eventKinds(events []Event) []EventKind {
	kinds := make([]EventKind, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func TestRunEmitsStrictPerTargetLifecycle(t *testing.T) {
	deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
		req.OnOutput([]byte("first"))
		req.OnOutput([]byte(" second"))
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api"), Mode: core.ModeSerial}, deps)
	events := collectEvents(t, r)
	want := []EventKind{
		EventTargetQueued,
		EventTargetStarted,
		EventTargetOutputChanged,
		EventTargetFinished,
		EventCompleted,
	}
	if got := eventKinds(events); !slices.Equal(got, want) {
		t.Fatalf("event kinds = %v, want %v", got, want)
	}
	if got := events[2].Target.OutputTail; got != "first second" {
		t.Fatalf("output tail = %q", got)
	}
	if events[3].Target.Status != core.StatusSucceeded {
		t.Fatalf("finished target = %#v", events[3].Target)
	}
	completed := events[4].Run
	if completed.Succeeded != 1 || len(completed.Targets) != 1 {
		t.Fatalf("completed snapshot = %#v", completed)
	}
}

func TestRunCoalescesUnreadOutputNotifications(t *testing.T) {
	deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
		for range 1_000 {
			req.OnOutput([]byte("x"))
		}
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api")}, deps)
	<-r.completed
	events := collectEvents(t, r)
	var outputs []Event
	for _, event := range events {
		if event.Kind == EventTargetOutputChanged {
			outputs = append(outputs, event)
		}
	}
	if len(outputs) != 1 {
		t.Fatalf("output event count = %d, want 1", len(outputs))
	}
	if len(outputs[0].Target.OutputTail) != 1_000 {
		t.Fatalf("output tail length = %d", len(outputs[0].Target.OutputTail))
	}
}

func TestRunHonorsWorkerLimit(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	var active atomic.Int32
	var maximum atomic.Int32
	deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
		current := active.Add(1)
		for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
		}
		started <- req.Target.ID
		<-release
		active.Add(-1)
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{
		Command: "test",
		Targets: testTargets("a", "b", "c"),
		Mode:    core.ModeParallel,
		Workers: 2,
	}, deps)
	<-started
	<-started
	select {
	case id := <-started:
		t.Fatalf("third target %q started before worker released", id)
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	collectEvents(t, r)
	if maximum.Load() != 2 {
		t.Fatalf("maximum concurrency = %d, want 2", maximum.Load())
	}
}

func TestRunSerialModeOverridesWorkerCount(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	deps := testDependencies(func(_ context.Context, _ executionRequest) executionOutcome {
		current := active.Add(1)
		for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
		}
		time.Sleep(5 * time.Millisecond)
		active.Add(-1)
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{
		Command: "test",
		Targets: testTargets("a", "b", "c"),
		Mode:    core.ModeSerial,
		Workers: 8,
	}, deps)
	collectEvents(t, r)
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrency = %d, want 1", maximum.Load())
	}
}

func TestRunCancelsQueuedTargetWithoutStartingIt(t *testing.T) {
	firstStarted := make(chan struct{})
	release := make(chan struct{})
	var executedMu sync.Mutex
	var executed []string
	deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
		executedMu.Lock()
		executed = append(executed, req.Target.ID)
		executedMu.Unlock()
		if req.Target.ID == "a" {
			close(firstStarted)
			<-release
		}
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{
		Command: "test",
		Targets: testTargets("a", "b"),
		Mode:    core.ModeSerial,
	}, deps)
	<-firstStarted
	cancelled := r.Cancel(TargetIDs("b", "b"))
	if !slices.Equal(cancelled.Accepted, []string{"b"}) {
		t.Fatalf("accepted = %v", cancelled.Accepted)
	}
	close(release)
	events := collectEvents(t, r)
	executedMu.Lock()
	defer executedMu.Unlock()
	if !slices.Equal(executed, []string{"a"}) {
		t.Fatalf("executed = %v", executed)
	}
	var bKinds []EventKind
	for _, event := range events {
		if event.Target != nil && event.Target.Target.ID == "b" {
			bKinds = append(bKinds, event.Kind)
		}
	}
	if !slices.Equal(bKinds, []EventKind{EventTargetQueued, EventTargetFinished}) {
		t.Fatalf("b events = %v", bKinds)
	}
}

func TestRunCancellationIsIdempotentForRunningTarget(t *testing.T) {
	started := make(chan struct{})
	deps := testDependencies(func(ctx context.Context, _ executionRequest) executionOutcome {
		close(started)
		<-ctx.Done()
		return executionOutcome{Status: core.StatusCancelled, Error: ctx.Err().Error()}
	})
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api")}, deps)
	<-started
	if got := r.Cancel(TargetIDs("api")).Accepted; !slices.Equal(got, []string{"api"}) {
		t.Fatalf("first accepted = %v", got)
	}
	if got := r.Cancel(TargetIDs("api")).Accepted; len(got) != 0 {
		t.Fatalf("second accepted = %v", got)
	}
	events := collectEvents(t, r)
	for _, event := range events {
		if event.Kind == EventTargetFinished && event.Target.Status != core.StatusCancelled {
			t.Fatalf("finished target = %#v", event.Target)
		}
	}
}

func TestRunFailFastCancelsEveryLaterTarget(t *testing.T) {
	var executed []string
	deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
		executed = append(executed, req.Target.ID)
		return executionOutcome{Status: core.StatusFailed, ExitCode: 7, Error: "exit 7"}
	})
	r := startTestRun(t, Spec{
		Command:  "test",
		Targets:  testTargets("a", "b", "c"),
		Mode:     core.ModeSerial,
		FailFast: true,
	}, deps)
	events := collectEvents(t, r)
	if !slices.Equal(executed, []string{"a"}) {
		t.Fatalf("executed = %v", executed)
	}
	completed := events[len(events)-1].Run
	if completed.Failed != 1 || completed.Cancelled != 2 {
		t.Fatalf("completed snapshot = %#v", completed)
	}
}

func TestRunArchiveFailureDoesNotRewriteTargetOutcome(t *testing.T) {
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		return executionOutcome{Status: core.StatusSucceeded}
	})
	deps.archive = func(context.Context, Snapshot, Spec) error {
		return errors.New("disk full")
	}
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api")}, deps)
	events := collectEvents(t, r)
	if got := eventKinds(events); !slices.Contains(got, EventArchiveFailed) {
		t.Fatalf("events = %v", got)
	}
	completed := events[len(events)-1]
	if completed.Kind != EventCompleted || completed.Run.Succeeded != 1 || completed.Run.Failed != 0 {
		t.Fatalf("completed = %#v", completed)
	}
}

func TestRunCommandHistoryFailureIsObservable(t *testing.T) {
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		return executionOutcome{Status: core.StatusSucceeded}
	})
	deps.recordCommand = func(string, time.Time) error { return errors.New("read only") }
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api")}, deps)
	events := collectEvents(t, r)
	if got := eventKinds(events); !slices.Contains(got, EventCommandHistoryFailed) {
		t.Fatalf("events = %v", got)
	}
}

func TestNextContextCancelsReadOnly(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		close(started)
		<-release
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api")}, deps)
	<-started
	for range 2 {
		if _, err := r.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	readCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := r.Next(readCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Next() error = %v", err)
	}
	close(release)
	events := collectEvents(t, r)
	completed := events[len(events)-1]
	if completed.Kind != EventCompleted || completed.Run.Succeeded != 1 {
		t.Fatalf("completed = %#v", completed)
	}
}

func TestStartContextOwnsRunLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	deps := testDependencies(func(ctx context.Context, _ executionRequest) executionOutcome {
		close(started)
		<-ctx.Done()
		return executionOutcome{Status: core.StatusCancelled, Error: ctx.Err().Error()}
	})
	r, err := start(ctx, Spec{Command: "test", Targets: testTargets("api")}, deps, nil)
	if err != nil {
		t.Fatal(err)
	}
	<-started
	cancel()
	events := collectEvents(t, r)
	completed := events[len(events)-1]
	if completed.Run.Cancelled != 1 {
		t.Fatalf("completed = %#v", completed)
	}
}

func TestStartRejectsInvalidSpecBeforeExecution(t *testing.T) {
	var calls atomic.Int32
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		calls.Add(1)
		return executionOutcome{Status: core.StatusSucceeded}
	})
	_, err := start(context.Background(), Spec{Command: "", Targets: testTargets("api")}, deps, nil)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if calls.Load() != 0 {
		t.Fatalf("execute calls = %d", calls.Load())
	}
}

func TestStartDeepCopiesTargetSnapshot(t *testing.T) {
	executed := make(chan core.Target, 1)
	release := make(chan struct{})
	deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
		executed <- req.Target
		<-release
		return executionOutcome{Status: core.StatusSucceeded}
	})
	targets := []core.Target{{ID: "api", Children: []string{"api/cmd"}}}
	r := startTestRun(t, Spec{Command: "test", Targets: targets}, deps)
	targets[0].Children[0] = "mutated"
	target := <-executed
	close(release)
	events := collectEvents(t, r)
	if target.Children[0] != "api/cmd" {
		t.Fatalf("executed target children = %v", target.Children)
	}
	completed := events[len(events)-1]
	if got := completed.Run.Targets[0].Target.Children[0]; got != "api/cmd" {
		t.Fatalf("completed target child = %q", got)
	}
	completed.Run.Targets[0].Target.Children[0] = "observer mutation"
	if target.Children[0] != "api/cmd" {
		t.Fatalf("observer mutated canonical target = %v", target.Children)
	}
}

func TestRunRejectsConcurrentNextCalls(t *testing.T) {
	release := make(chan struct{})
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		<-release
		return executionOutcome{Status: core.StatusSucceeded}
	})
	r := startTestRun(t, Spec{Command: "test", Targets: testTargets("api")}, deps)
	for range 2 {
		if _, err := r.Next(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	first := make(chan error, 1)
	go func() {
		_, err := r.Next(context.Background())
		first <- err
	}()
	time.Sleep(10 * time.Millisecond)
	if _, err := r.Next(context.Background()); err == nil || !strings.Contains(err.Error(), "concurrent Next") {
		t.Fatalf("concurrent Next() error = %v", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatalf("first Next() error = %v", err)
	}
	collectEvents(t, r)
}

func TestStartRejectsNilContext(t *testing.T) {
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		return executionOutcome{Status: core.StatusSucceeded}
	})
	if _, err := start(nil, Spec{Command: "test", Targets: testTargets("api")}, deps, nil); err == nil {
		t.Fatal("expected nil context error")
	}
}
