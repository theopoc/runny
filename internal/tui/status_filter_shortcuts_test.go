package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/theopoc/runny/internal/core"
	runpkg "github.com/theopoc/runny/internal/run"
)

func TestTargetStatusFilterShortcuts(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "worker", RelPath: "worker"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["worker"] = core.StatusRunning
	model.Status["web"] = core.StatusSucceeded

	for _, test := range []struct {
		key  string
		want []int
	}{
		{key: "f", want: []int{0}},
		{key: "r", want: []int{1}},
		{key: "o", want: []int{2}},
	} {
		t.Run(test.key, func(t *testing.T) {
			filtered, _ := updateKey(model, test.key)
			if got := filtered.visibleTargetIndexes(); !slices.Equal(got, test.want) {
				t.Fatalf("%s visible indexes = %v, want %v", test.key, got, test.want)
			}
			if filtered.ShowOptions {
				t.Fatalf("%s should filter targets, not open options", test.key)
			}

			cleared, _ := updateKey(filtered, test.key)
			if got := cleared.visibleTargetIndexes(); !slices.Equal(got, []int{0, 1, 2}) {
				t.Fatalf("second %s visible indexes = %v, want all targets", test.key, got)
			}
		})
	}
}

func TestTargetStatusFilterCombinesWithPathFilterAndKeepsAncestors(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "services", RelPath: "services"},
		{ID: "api", RelPath: "services/api", ParentID: "services"},
		{ID: "web", RelPath: "services/web", ParentID: "services"},
		{ID: "worker", RelPath: "worker"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusSucceeded
	model.Status["worker"] = core.StatusFailed
	model.Filter = "api"

	model, _ = updateKey(model, "f")
	if got := model.visibleTargetIndexes(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("combined visible indexes = %v, want ancestor and matching target", got)
	}
}

func TestStatusFilterFocusesDirectMatchInsteadOfAncestor(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "services", RelPath: "services"},
		{ID: "api", RelPath: "services/api", ParentID: "services"},
	}})
	model.Status["api"] = core.StatusFailed

	model, _ = updateKey(model, "f")
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want direct failed target at 1", model.Cursor)
	}
}

func TestStatusFilterKeepsCursorOnVisibleMatchAsStatusesChange(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusRunning
	model, _ = updateKey(model, "r")

	model.applyTargetSnapshot(runpkg.TargetSnapshot{Target: model.Targets[0], Status: core.StatusSucceeded})
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want remaining running target at 1", model.Cursor)
	}
}

func TestStatusFilterRenderingNamesTheActiveStatus(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Status["api"] = core.StatusFailed
	model, _ = updateKey(model, "f")

	rendered := stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))
	if !strings.Contains(rendered, "filter: failed") {
		t.Fatalf("status-filtered panel should name active status:\n%s", rendered)
	}
}

func TestEmptyStatusFilterRenderingDoesNotShowEmptyPathQuery(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Status["api"] = core.StatusSucceeded
	model, _ = updateKey(model, "f")

	rendered := stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))
	if !strings.Contains(rendered, "No failed targets") || strings.Contains(rendered, "No matches for /") {
		t.Fatalf("empty status filter message should name failed status:\n%s", rendered)
	}
}

func TestEscapeClearsPathAndStatusFilters(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusSucceeded
	model.Filter = "api"
	model, _ = updateKey(model, "f")

	model, _ = updateKey(model, "esc")
	if model.Filter != "" {
		t.Fatalf("path filter = %q, want empty", model.Filter)
	}
	if got := model.visibleTargetIndexes(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("visible indexes after escape = %v, want all targets", got)
	}
}

func TestFilterEditorEscapeClearsBareRegexPrefix(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "re:"
	model.Focus = FocusFilter

	model, _ = updateKey(model, "esc")
	if model.Focus != FocusTargets || model.Filter != "" {
		t.Fatalf("focus/filter = %v/%q, want Targets and empty", model.Focus, model.Filter)
	}
}

func TestTasksEscapeClearsBareRegexPrefix(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "re:"

	model, _ = updateKey(model, "esc")
	if model.Filter != "" {
		t.Fatalf("filter = %q, want empty", model.Filter)
	}
}

func TestTasksEscapeClearsFiltersWithStaleOutputSelection(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusSucceeded
	model.Filter = "api"
	model, _ = updateKey(model, "f")
	model.outputSelection = outputSelection{targetID: "api", snapshot: "output", moved: true}

	model, _ = updateKey(model, "esc")
	if model.outputSelection.active() {
		t.Fatal("output selection should be cleared")
	}
	if model.Filter != "" {
		t.Fatalf("path filter = %q, want empty", model.Filter)
	}
	if got := model.visibleTargetIndexes(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("visible indexes = %v, want all targets", got)
	}
}

func TestFilterEditorEscapeClearsPathAndStatusFilters(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusSucceeded
	model.Filter = "api"
	model, _ = updateKey(model, "f")
	model, _ = updateKey(model, "/")

	model, _ = updateKey(model, "esc")
	if model.Focus != FocusTargets || model.Filter != "" {
		t.Fatalf("focus/filter = %v/%q, want Targets and empty", model.Focus, model.Filter)
	}
	if got := model.visibleTargetIndexes(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("visible indexes = %v, want all targets", got)
	}
}

func TestTogglingStatusFilterOffPreservesPathFilter(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusFailed
	model.Filter = "api"

	model, _ = updateKey(model, "f")
	model, _ = updateKey(model, "f")
	if model.Filter != "api" {
		t.Fatalf("path filter = %q, want api", model.Filter)
	}
	if got := model.visibleTargetIndexes(); !slices.Equal(got, []int{0}) {
		t.Fatalf("visible indexes = %v, want path-filtered target", got)
	}
}

func TestPaletteClearFilterClearsPathAndStatusFilters(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusSucceeded
	model.Filter = "api"
	model, _ = updateKey(model, "f")

	model, _ = runPaletteCommand(model, "clear-filter")
	if model.Filter != "" {
		t.Fatalf("path filter = %q, want empty", model.Filter)
	}
	if got := model.visibleTargetIndexes(); !slices.Equal(got, []int{0, 1}) {
		t.Fatalf("visible indexes = %v, want all targets", got)
	}
}

func TestOptionsShortcutMovesToUppercaseO(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Status["api"] = core.StatusSucceeded

	model, _ = updateKey(model, "o")
	if model.ShowOptions {
		t.Fatal("lowercase o should filter ok targets")
	}

	model, _ = updateKey(model, "O")
	if !model.ShowOptions {
		t.Fatal("uppercase O should open options")
	}
	model, _ = updateKey(model, "O")
	if model.ShowOptions {
		t.Fatal("uppercase O should close options")
	}
}

func TestStatusFilterShortcutsAreDiscoverable(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	help := strings.Join(model.helpRows(120), "\n")
	for _, want := range []string{"f/r/o failed/running/ok", "O open/close"} {
		if !strings.Contains(stripANSI(help), want) {
			t.Fatalf("help should contain %q:\n%s", want, stripANSI(help))
		}
	}
}
