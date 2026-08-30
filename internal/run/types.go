package run

import (
	"context"
	"time"

	"github.com/theopoc/runny/internal/core"
)

// ID identifies one accepted Run and its archived diagnostics.
type ID string

// Spec is the fixed input snapshot for one Run.
type Spec struct {
	Command        string
	Targets        []core.Target
	Mode           core.ExecutionMode
	Workers        int
	FailFast       bool
	SaveLogs       bool
	DisableLogging bool
}

type EventKind string

const (
	EventTargetQueued         EventKind = "target_queued"
	EventTargetStarted        EventKind = "target_started"
	EventTargetOutputChanged  EventKind = "target_output_changed"
	EventTargetFinished       EventKind = "target_finished"
	EventCommandHistoryFailed EventKind = "command_history_failed"
	EventArchiveFailed        EventKind = "archive_failed"
	EventCompleted            EventKind = "completed"
)

// Snapshot is an immutable view of canonical Run state at one event.
// Targets is populated only for EventCompleted.
type Snapshot struct {
	ID        ID
	LogID     ID
	Command   string
	Accepted  time.Time
	Started   time.Time
	Ended     time.Time
	Total     int
	Queued    int
	Running   int
	Succeeded int
	Failed    int
	Cancelled int
	Skipped   int
	Targets   []TargetSnapshot
}

// TargetSnapshot is an immutable view of one Target execution.
type TargetSnapshot struct {
	Target          core.Target
	Status          core.Status
	ExitCode        int
	OutputTail      string
	OutputTruncated bool
	Error           string
	Started         time.Time
	Ended           time.Time
}

// Event describes one factual Run lifecycle change.
type Event struct {
	Kind   EventKind
	At     time.Time
	Run    Snapshot
	Target *TargetSnapshot
	Err    error
}

type CancelScope struct {
	all bool
	ids []string
}

// AllTargets selects every non-terminal Target execution in the Run.
func AllTargets() CancelScope { return CancelScope{all: true} }

// TargetIDs selects a fixed list of Target executions in the Run.
func TargetIDs(ids ...string) CancelScope {
	return CancelScope{ids: append([]string(nil), ids...)}
}

// Cancellation lists Target IDs whose cancellation was newly accepted.
type Cancellation struct {
	Accepted []string
}

// StartFunc accepts one Run and returns its single-observer event stream.
type StartFunc func(context.Context, Spec) (*Run, error)
