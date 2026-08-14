package tui

import (
	"strings"
	"testing"

	"github.com/theopoc/runny/internal/core"
)

func TestRunningStatusSpinnerAdvancesOnTick(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Running = true
	model.Status["api"] = core.StatusRunning

	before := model.statusLabel(core.StatusRunning)
	updated, cmd := model.Update(spinnerTickMsg{})
	after := updated.(Model).statusLabel(core.StatusRunning)

	if before == after {
		t.Fatalf("running status spinner did not advance: %q", after)
	}
	if !strings.HasSuffix(after, " running") {
		t.Fatalf("running status = %q, want spinner followed by running label", after)
	}
	if cmd == nil {
		t.Fatal("running spinner tick should schedule next frame")
	}
}

func TestRunningStatusSpinnerStopsWhenRunIsIdle(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})

	updated, cmd := model.Update(spinnerTickMsg{})

	if updated.(Model).spinnerFrame != 0 {
		t.Fatal("idle spinner tick should not advance frame")
	}
	if cmd != nil {
		t.Fatal("idle spinner tick should not schedule another frame")
	}
}
