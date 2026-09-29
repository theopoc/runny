package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
)

func TestStatusFilterShortcutsToggleFailedRunningAndOK(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "failed", RelPath: "failed"},
		{ID: "running", RelPath: "running"},
		{ID: "ok", RelPath: "ok"},
	}})
	model.Status["failed"] = core.StatusFailed
	model.Status["running"] = core.StatusRunning
	model.Status["ok"] = core.StatusSucceeded

	for _, test := range []struct {
		key    string
		status core.Status
		path   string
	}{
		{key: "F", status: core.StatusFailed, path: "failed"},
		{key: "r", status: core.StatusRunning, path: "running"},
		{key: "O", status: core.StatusSucceeded, path: "ok"},
	} {
		model, _ = updateKey(model, test.key)
		if model.StatusFilter != test.status {
			t.Fatalf("%s status filter = %q, want %q", test.key, model.StatusFilter, test.status)
		}
		indexes := model.matchingTargetIndexes()
		if len(indexes) != 1 || model.Targets[indexes[0]].RelPath != test.path {
			t.Fatalf("%s matching targets = %#v, want only %q", test.key, indexes, test.path)
		}
	}

	model, _ = updateKey(model, "O")
	if model.StatusFilter != "" {
		t.Fatalf("second O should clear status filter, got %q", model.StatusFilter)
	}
}

func TestStatusFilterCombinesWithPathFilterAndEscapeClearsBoth(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "services/api"},
		{ID: "web", RelPath: "services/web"},
		{ID: "worker", RelPath: "worker"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusSucceeded
	model.Status["worker"] = core.StatusFailed
	model.Filter = "services"

	model, _ = updateKey(model, "F")
	indexes := model.matchingTargetIndexes()
	if len(indexes) != 1 || model.Targets[indexes[0]].ID != "api" {
		t.Fatalf("combined matching targets = %#v, want api", indexes)
	}
	if rendered := stripANSI(model.View().Content); !strings.Contains(rendered, "status filter: failed") {
		t.Fatalf("active status filter should be visible:\n%s", rendered)
	}

	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.Filter != "" || model.StatusFilter != "" {
		t.Fatalf("escape filters = %q/%q, want cleared", model.Filter, model.StatusFilter)
	}
}

func TestStatusFilterTracksLiveStatusChanges(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusFailed
	model, _ = updateKey(model, "r")

	model.Status["api"] = core.StatusSucceeded
	model.Status["web"] = core.StatusRunning
	model.ensureCursorVisible()
	indexes := model.matchingTargetIndexes()
	if len(indexes) != 1 || model.Targets[indexes[0]].ID != "web" || model.Cursor != 1 {
		t.Fatalf("live running targets = %#v cursor=%d, want web", indexes, model.Cursor)
	}
}
