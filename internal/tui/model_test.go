package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/saewyn/runny/internal/core"
)

func TestModelToggleSelectAllAndFilter(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, " ")
	if model.Targets[0].Selected {
		t.Fatal("focused target should be deselected")
	}
	model, _ = updateKey(model, "a")
	if !model.Targets[0].Selected || !model.Targets[1].Selected {
		t.Fatal("all targets should be selected")
	}
	model, _ = updateKey(model, "/")
	if model.Focus != FocusFilter {
		t.Fatal("filter should be focused")
	}
}

func TestModelMovesCursorWithArrowKeys(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Cursor != 0 {
		t.Fatalf("cursor = %d, want 0", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	model, _ = updateKey(model, " ")
	if model.Targets[1].Selected {
		t.Fatal("second target should be deselected after moving cursor")
	}
}

func TestModelFilterTextLimitsVisibleCursor(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model, _ = updateKey(model, "w")
	if model.Filter != "w" {
		t.Fatalf("filter = %q", model.Filter)
	}
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want 1", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyBackspace)
	if model.Filter != "" {
		t.Fatalf("filter = %q", model.Filter)
	}
	model, _ = updateKey(model, "a")
	model, _ = updateKey(model, "p")
	model, _ = updateKey(model, "i")
	model.Targets[1].Selected = false
	model, _ = updateKey(model, "a")
	if !model.Targets[0].Selected {
		t.Fatal("visible api target should be selected")
	}
	if model.Targets[1].Selected {
		t.Fatal("hidden web target should stay unselected")
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.Focus != FocusTargets {
		t.Fatal("escape should leave filter focus")
	}
}

func TestModelOverlaysAndCancelSelection(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusRunning
	model, _ = updateKey(model, "?")
	if !model.ShowHelp {
		t.Fatal("help should show")
	}
	model, _ = updateKey(model, "H")
	if !model.ShowHistory {
		t.Fatal("history should show")
	}
	model, _ = updateKey(model, "delete")
	if model.Status["api"] != core.StatusCancelled {
		t.Fatalf("status = %s", model.Status["api"])
	}
}

func TestViewUsesAltScreenAndTUIPanels(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})

	view := model.View()
	if !view.AltScreen {
		t.Fatal("view should use alt screen")
	}
	for _, want := range []string{"Directories", "Logs", "Shortcuts"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("view content should contain %q:\n%s", want, view.Content)
		}
	}
}

func updateKey(model Model, key string) (Model, tea.Cmd) {
	msg := tea.KeyPressMsg(tea.Key{Text: key})
	if key == "delete" {
		msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyDelete})
	}
	updated, cmd := model.Update(msg)
	return updated.(Model), cmd
}

func updateSpecialKey(model Model, key rune) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: key}))
	return updated.(Model), cmd
}
