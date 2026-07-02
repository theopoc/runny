package core

import "testing"

func TestSelectedTargets(t *testing.T) {
	targets := []Target{
		{ID: "a", RelPath: "api", Selected: true},
		{ID: "b", RelPath: "web", Selected: false},
		{ID: "c", RelPath: "worker", Selected: true},
	}

	selected := SelectedTargets(targets)
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[0].RelPath != "api" || selected[1].RelPath != "worker" {
		t.Fatalf("selected targets = %#v", selected)
	}
}

func TestStatusTerminal(t *testing.T) {
	terminal := []Status{StatusSucceeded, StatusFailed, StatusCancelled, StatusSkipped}
	for _, status := range terminal {
		if !status.Terminal() {
			t.Fatalf("%s should be terminal", status)
		}
	}

	active := []Status{StatusQueued, StatusRunning}
	for _, status := range active {
		if status.Terminal() {
			t.Fatalf("%s should not be terminal", status)
		}
	}
}
