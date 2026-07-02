package core

import "time"

type Status string

const (
	StatusIdle      Status = "idle"
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusSkipped   Status = "skipped"
)

func (s Status) Terminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled, StatusSkipped:
		return true
	default:
		return false
	}
}

type Target struct {
	ID       string
	RelPath  string
	AbsPath  string
	Name     string
	Depth    int
	ParentID string
	Children []string
	Selected bool
	Folded   bool
	Hidden   bool
	Skipped  bool
}

type ExecutionMode string

const (
	ModeParallel ExecutionMode = "parallel"
	ModeSerial   ExecutionMode = "serial"
)

type RunRequest struct {
	Command        string
	Targets        []Target
	Mode           ExecutionMode
	Workers        int
	FailFast       bool
	SaveLogs       bool
	DisableLogging bool
	LogRoot        string
}

type RunResult struct {
	Target   Target
	Status   Status
	ExitCode int
	Output   string
	Error    string
	Started  time.Time
	Ended    time.Time
}

type EventType string

const (
	EventQueued   EventType = "queued"
	EventStarted  EventType = "started"
	EventOutput   EventType = "output"
	EventFinished EventType = "finished"
)

type Event struct {
	Type     EventType
	TargetID string
	Target   Target
	Status   Status
	Line     string
	Result   RunResult
	Time     time.Time
}

func SelectedTargets(targets []Target) []Target {
	selected := make([]Target, 0, len(targets))
	for _, target := range targets {
		if target.Selected {
			selected = append(selected, target)
		}
	}
	return selected
}
