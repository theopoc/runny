package run

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/theopoc/runny/internal/core"
)

type executionRequest struct {
	Command  string
	Target   core.Target
	OnOutput func([]byte)
}

type executionOutcome struct {
	Status   core.Status
	ExitCode int
	Error    string
	Started  time.Time
	Ended    time.Time
}

type executeFunc func(context.Context, executionRequest) executionOutcome
type recordCommandFunc func(string, time.Time) error
type archiveFunc func(context.Context, Snapshot, Spec) error

type dependencies struct {
	execute       executeFunc
	recordCommand recordCommandFunc
	archive       archiveFunc
	now           func() time.Time
	newID         func(time.Time) (ID, error)
}

type targetState struct {
	snapshot       TargetSnapshot
	output         tailBuffer
	done           bool
	cancelAccepted bool
	cancel         context.CancelFunc
}

// Run owns canonical state and event delivery for one accepted Run.
type Run struct {
	ctx    context.Context
	cancel context.CancelFunc
	spec   Spec
	deps   dependencies
	events *eventQueue
	onDone func()

	mu        sync.Mutex
	nextMu    sync.Mutex
	snapshot  Snapshot
	order     []string
	targets   map[string]*targetState
	completed chan struct{}
}

func start(ctx context.Context, spec Spec, deps dependencies, onDone func()) (*Run, error) {
	if ctx == nil {
		return nil, errors.New("run context is required")
	}
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if deps.execute == nil || deps.now == nil || deps.newID == nil {
		return nil, errors.New("incomplete run dependencies")
	}
	if spec.Mode == "" {
		spec.Mode = core.ModeParallel
	}
	spec.Targets = cloneTargets(spec.Targets)
	accepted := deps.now()
	id, err := deps.newID(accepted)
	if err != nil {
		return nil, fmt.Errorf("create run id: %w", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	r := &Run{
		ctx:       runCtx,
		cancel:    cancel,
		spec:      spec,
		deps:      deps,
		events:    newEventQueue(),
		onDone:    onDone,
		targets:   make(map[string]*targetState, len(spec.Targets)),
		completed: make(chan struct{}),
		snapshot: Snapshot{
			ID:       id,
			Command:  spec.Command,
			Accepted: accepted,
			Total:    len(spec.Targets),
		},
	}
	if spec.SaveLogs && !spec.DisableLogging {
		r.snapshot.LogID = id
	}
	for _, target := range spec.Targets {
		r.order = append(r.order, target.ID)
		r.targets[target.ID] = &targetState{snapshot: TargetSnapshot{
			Target: target,
			Status: core.StatusQueued,
		}}
	}
	for _, id := range r.order {
		target := r.targetSnapshotLocked(id)
		r.events.push(r.eventLocked(EventTargetQueued, &target, nil, false))
	}
	go r.execute()
	return r, nil
}

func validateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Command) == "" {
		return errors.New("command is required")
	}
	if len(spec.Targets) == 0 {
		return errors.New("at least one target is required")
	}
	if spec.Mode != "" && spec.Mode != core.ModeParallel && spec.Mode != core.ModeSerial {
		return fmt.Errorf("invalid execution mode %q", spec.Mode)
	}
	if spec.Workers < 0 {
		return errors.New("workers cannot be negative")
	}
	seen := make(map[string]struct{}, len(spec.Targets))
	for _, target := range spec.Targets {
		if target.ID == "" {
			return errors.New("target id is required")
		}
		if _, exists := seen[target.ID]; exists {
			return fmt.Errorf("duplicate target id %q", target.ID)
		}
		seen[target.ID] = struct{}{}
	}
	return nil
}

func (r *Run) execute() {
	defer r.finishRuntime()

	if r.deps.recordCommand != nil {
		if err := r.deps.recordCommand(r.spec.Command, r.snapshot.Accepted); err != nil {
			r.events.push(r.event(EventCommandHistoryFailed, nil, err, false))
		}
	}
	watchDone := make(chan struct{})
	go func() {
		select {
		case <-r.ctx.Done():
			r.Cancel(AllTargets())
		case <-watchDone:
		}
	}()
	if r.ctx.Err() != nil {
		r.Cancel(AllTargets())
	}

	jobs := make(chan string, len(r.order))
	for _, id := range r.order {
		jobs <- id
	}
	close(jobs)
	var workers sync.WaitGroup
	for range r.workerCount() {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range jobs {
				r.executeTarget(id)
			}
		}()
	}
	workers.Wait()
	close(watchDone)

	r.mu.Lock()
	r.snapshot.Ended = r.deps.now()
	completed := r.snapshotLocked(true)
	r.mu.Unlock()
	if r.deps.archive != nil {
		if err := r.deps.archive(context.WithoutCancel(r.ctx), completed, r.spec); err != nil {
			r.events.push(r.event(EventArchiveFailed, nil, err, false))
		}
	}
	r.events.push(r.event(EventCompleted, nil, nil, true))
	r.events.close()
}

func (r *Run) executeTarget(id string) {
	r.mu.Lock()
	state := r.targets[id]
	if state == nil || state.done || state.snapshot.Status != core.StatusQueued {
		r.mu.Unlock()
		return
	}
	targetCtx, cancel := context.WithCancel(r.ctx)
	state.cancel = cancel
	state.snapshot.Status = core.StatusRunning
	state.snapshot.Started = r.deps.now()
	if r.snapshot.Started.IsZero() {
		r.snapshot.Started = state.snapshot.Started
	}
	target := r.targetSnapshotLocked(id)
	event := r.eventLocked(EventTargetStarted, &target, nil, false)
	r.mu.Unlock()
	r.events.push(event)

	outcome := r.deps.execute(targetCtx, executionRequest{
		Command: r.spec.Command,
		Target:  target.Target,
		OnOutput: func(chunk []byte) {
			r.appendOutput(id, chunk)
		},
	})
	cancel()
	r.finishTarget(id, outcome)
}

func (r *Run) appendOutput(id string, chunk []byte) {
	if r.spec.DisableLogging || len(chunk) == 0 {
		return
	}
	r.mu.Lock()
	state := r.targets[id]
	if state == nil || state.done {
		r.mu.Unlock()
		return
	}
	state.output.Append(chunk)
	state.snapshot.OutputTail = state.output.String()
	state.snapshot.OutputTruncated = state.output.truncated
	if state.cancelAccepted {
		r.mu.Unlock()
		return
	}
	target := r.targetSnapshotLocked(id)
	event := r.eventLocked(EventTargetOutputChanged, &target, nil, false)
	r.mu.Unlock()
	r.events.pushOutput(id, event)
}

func (r *Run) finishTarget(id string, outcome executionOutcome) {
	var (
		queuedEvents []Event
		cancels      []context.CancelFunc
		failFast     bool
	)
	r.mu.Lock()
	state := r.targets[id]
	if state == nil || state.done {
		r.mu.Unlock()
		return
	}
	state.done = true
	state.cancel = nil
	if !outcome.Started.IsZero() {
		state.snapshot.Started = outcome.Started
	}
	if outcome.Ended.IsZero() {
		state.snapshot.Ended = r.deps.now()
	} else {
		state.snapshot.Ended = outcome.Ended
	}
	if state.cancelAccepted {
		state.snapshot.Status = core.StatusCancelled
		if outcome.Error == "" {
			state.snapshot.Error = context.Canceled.Error()
		} else {
			state.snapshot.Error = outcome.Error
		}
	} else {
		state.snapshot.Status = outcome.Status
		if !state.snapshot.Status.Terminal() {
			state.snapshot.Status = core.StatusFailed
			if outcome.Error == "" {
				outcome.Error = "executor returned non-terminal status"
			}
		}
		state.snapshot.Error = outcome.Error
	}
	state.snapshot.ExitCode = outcome.ExitCode
	target := r.targetSnapshotLocked(id)
	finished := r.eventLocked(EventTargetFinished, &target, nil, false)
	failFast = r.spec.FailFast && target.Status == core.StatusFailed
	if failFast {
		queuedEvents, cancels, _ = r.cancelLocked(AllTargets())
	}
	r.mu.Unlock()

	r.events.pushFinished(id, finished)
	for _, event := range queuedEvents {
		r.events.pushFinished(event.Target.Target.ID, event)
	}
	for _, cancel := range cancels {
		cancel()
	}
}

// Next waits for the next event. Cancelling ctx cancels this read only.
func (r *Run) Next(ctx context.Context) (Event, error) {
	if ctx == nil {
		return Event{}, errors.New("nil next context")
	}
	if !r.nextMu.TryLock() {
		return Event{}, errors.New("concurrent Next calls are not supported")
	}
	defer r.nextMu.Unlock()
	return r.events.next(ctx)
}

// Cancel idempotently requests cancellation for a fixed scope.
func (r *Run) Cancel(scope CancelScope) Cancellation {
	r.mu.Lock()
	events, cancels, accepted := r.cancelLocked(scope)
	r.mu.Unlock()
	for _, event := range events {
		r.events.pushFinished(event.Target.Target.ID, event)
	}
	for _, cancel := range cancels {
		cancel()
	}
	return Cancellation{Accepted: accepted}
}

func (r *Run) cancelLocked(scope CancelScope) ([]Event, []context.CancelFunc, []string) {
	ids := scope.ids
	if scope.all {
		ids = r.order
	}
	seen := make(map[string]struct{}, len(ids))
	var events []Event
	var cancels []context.CancelFunc
	var accepted []string
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		state := r.targets[id]
		if state == nil || state.done || state.cancelAccepted {
			continue
		}
		switch state.snapshot.Status {
		case core.StatusQueued:
			state.cancelAccepted = true
			state.done = true
			state.snapshot.Status = core.StatusCancelled
			state.snapshot.Error = context.Canceled.Error()
			state.snapshot.Ended = r.deps.now()
			target := r.targetSnapshotLocked(id)
			events = append(events, r.eventLocked(EventTargetFinished, &target, nil, false))
			accepted = append(accepted, id)
		case core.StatusRunning:
			state.cancelAccepted = true
			state.snapshot.Status = core.StatusCancelled
			accepted = append(accepted, id)
			if state.cancel != nil {
				cancels = append(cancels, state.cancel)
			}
		}
	}
	return events, cancels, accepted
}

func (r *Run) event(kind EventKind, target *TargetSnapshot, err error, includeTargets bool) Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.eventLocked(kind, target, err, includeTargets)
}

func (r *Run) eventLocked(kind EventKind, target *TargetSnapshot, err error, includeTargets bool) Event {
	var targetCopy *TargetSnapshot
	if target != nil {
		copy := *target
		targetCopy = &copy
	}
	return Event{
		Kind:   kind,
		At:     r.deps.now(),
		Run:    r.snapshotLocked(includeTargets),
		Target: targetCopy,
		Err:    err,
	}
}

func (r *Run) snapshotLocked(includeTargets bool) Snapshot {
	snapshot := r.snapshot
	for _, state := range r.targets {
		switch state.snapshot.Status {
		case core.StatusQueued:
			snapshot.Queued++
		case core.StatusRunning:
			snapshot.Running++
		case core.StatusSucceeded:
			snapshot.Succeeded++
		case core.StatusFailed:
			snapshot.Failed++
		case core.StatusCancelled:
			snapshot.Cancelled++
		case core.StatusSkipped:
			snapshot.Skipped++
		}
	}
	if includeTargets {
		snapshot.Targets = make([]TargetSnapshot, 0, len(r.order))
		for _, id := range r.order {
			snapshot.Targets = append(snapshot.Targets, r.targetSnapshotLocked(id))
		}
	}
	return snapshot
}

func (r *Run) targetSnapshotLocked(id string) TargetSnapshot {
	snapshot := r.targets[id].snapshot
	snapshot.Target = cloneTarget(snapshot.Target)
	return snapshot
}

func cloneTargets(targets []core.Target) []core.Target {
	cloned := make([]core.Target, len(targets))
	for i, target := range targets {
		cloned[i] = cloneTarget(target)
	}
	return cloned
}

func cloneTarget(target core.Target) core.Target {
	target.Children = append([]string(nil), target.Children...)
	return target
}

func (r *Run) workerCount() int {
	if r.spec.Mode == core.ModeSerial {
		return 1
	}
	workers := r.spec.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return min(workers, len(r.order))
}

func (r *Run) finishRuntime() {
	r.cancel()
	close(r.completed)
	if r.onDone != nil {
		r.onDone()
	}
}
