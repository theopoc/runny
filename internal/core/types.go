package core

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
