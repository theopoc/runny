package core

import "testing"

func TestStatusTerminal(t *testing.T) {
	terminal := []Status{StatusSucceeded, StatusFailed, StatusCancelled, StatusSkipped}
	for _, status := range terminal {
		if !status.Terminal() {
			t.Fatalf("%s should be terminal", status)
		}
	}

	active := []Status{StatusIdle, StatusQueued, StatusRunning}
	for _, status := range active {
		if status.Terminal() {
			t.Fatalf("%s should not be terminal", status)
		}
	}
}
