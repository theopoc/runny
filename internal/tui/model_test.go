package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
	runpkg "github.com/theopoc/runny/internal/run"
)

func TestModelToggleAllWithLowercaseAAndFilter(t *testing.T) {
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
		t.Fatal("a should select all visible targets when one is unselected")
	}
	model, _ = updateKey(model, "a")
	if model.Targets[0].Selected || model.Targets[1].Selected {
		t.Fatal("a should deselect all visible targets when all are selected")
	}
	model, _ = updateKey(model, "A")
	if model.Targets[0].Selected || model.Targets[1].Selected {
		t.Fatal("uppercase A should not toggle bulk selection")
	}

	model.Filter = "api"
	model, _ = updateKey(model, "a")
	if !model.Targets[0].Selected {
		t.Fatal("a should select matching targets when filtered")
	}
	if model.Targets[1].Selected {
		t.Fatal("a should leave non-matching targets unchanged")
	}
	model, _ = updateKey(model, "/")
	if model.Focus != FocusFilter {
		t.Fatal("filter should be focused")
	}
}

func TestModelToggleSelectsTargetSubtree(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Children: []string{"api/cmd", "api/pkg"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Children: []string{"api/cmd/foo"}, Folded: true},
		{ID: "api/cmd/foo", RelPath: "api/cmd/foo", ParentID: "api/cmd", Depth: 3},
		{ID: "api/pkg", RelPath: "api/pkg", ParentID: "api", Depth: 2},
		{ID: "web", RelPath: "web"},
	}})

	model, _ = updateKey(model, " ")
	for _, target := range model.Targets[:4] {
		if !target.Selected {
			t.Fatalf("%s should be selected with subtree: %#v", target.ID, model.Targets)
		}
	}
	if model.Targets[4].Selected {
		t.Fatalf("sibling target should not be selected: %#v", model.Targets)
	}

	model.Targets[1].Selected = false
	model.Targets[2].Selected = false
	model, _ = updateKey(model, " ")
	for _, target := range model.Targets[:4] {
		if target.Selected {
			t.Fatalf("%s should be deselected with subtree: %#v", target.ID, model.Targets)
		}
	}
}

func TestModelTogglePartiallySelectedFolderSelectsEntireSubtree(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Children: []string{"api/cmd", "api/pkg"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
		{ID: "api/pkg", RelPath: "api/pkg", ParentID: "api", Depth: 2},
	}})

	model, _ = updateKey(model, " ")
	for _, target := range model.Targets {
		if !target.Selected {
			t.Fatalf("%s should be selected from partial subtree: %#v", target.ID, model.Targets)
		}
	}
}

func TestModelToggleAcceptsNamedSpaceKey(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api"}}})

	model, _ = updateNamedKey(model, "space")
	if !model.Targets[0].Selected {
		t.Fatalf("named space key should toggle selection: %#v", model.Targets)
	}
}

func TestModelEscapeClearsFilterActivatedWithEnterAndRestoresTree(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model = typeText(model, "api")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.Filter != "api" || model.Focus != FocusTargets {
		t.Fatalf("enter should activate filter and return to tasks, filter/focus = %q/%v", model.Filter, model.Focus)
	}
	if view := stripANSI(model.View().Content); !strings.Contains(view, "showing 1-1 of 1") || strings.Contains(view, "web") {
		t.Fatalf("activated filter should show only matching target:\n%s", view)
	}

	model, _ = updateSpecialKey(model, tea.KeyEsc)

	if model.Filter != "" || model.Focus != FocusTargets {
		t.Fatalf("escape should clear activated filter and keep tasks focus, filter/focus = %q/%v", model.Filter, model.Focus)
	}
	if !model.Targets[0].Selected || !model.Targets[1].Selected {
		t.Fatalf("escape should not change selection while clearing filter: %#v", model.Targets)
	}
	if model.Notice != "filter cleared" {
		t.Fatalf("notice = %q, want filter cleared", model.Notice)
	}
	if view := stripANSI(model.View().Content); !strings.Contains(view, "showing 1-2 of 2") || !strings.Contains(view, "web") {
		t.Fatalf("escape should restore unfiltered directory tree:\n%s", view)
	}
}

func TestModelSpaceKeepsSelectionBehaviorWithActivatedFilter(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Filter = "api"

	model, _ = updateKey(model, " ")

	if model.Filter != "api" {
		t.Fatalf("space should not clear activated filter, filter = %q", model.Filter)
	}
	if model.Targets[0].Selected || !model.Targets[1].Selected {
		t.Fatalf("space should toggle only focused filtered target: %#v", model.Targets)
	}
}

func TestFilterFocusSpaceRemainsPartOfQuery(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Focus = FocusFilter
	model.Filter = "api"

	model, _ = updateKey(model, " ")

	if model.Filter != "api " || model.Focus != FocusFilter {
		t.Fatalf("space should remain editable before filter activation, filter/focus = %q/%v", model.Filter, model.Focus)
	}
}

func TestModelStartsWithTasksFocusWhenCommandIsEmpty(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	if model.Focus != FocusTargets {
		t.Fatalf("focus = %v, want tasks", model.Focus)
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

func TestOutputFocusIgnoresTaskArrowNavigation(t *testing.T) {
	for _, key := range []rune{tea.KeyUp, tea.KeyDown} {
		t.Run(tea.KeyPressMsg(tea.Key{Code: key}).String(), func(t *testing.T) {
			model := NewModel(Options{Command: "test", Targets: []core.Target{
				{ID: "api", RelPath: "api", Selected: true},
				{ID: "web", RelPath: "web", Selected: true},
			}})
			model.Focus = FocusLogs

			model, _ = updateSpecialKey(model, key)

			if model.Cursor != 0 {
				t.Fatalf("cursor = %d, want unchanged 0 while output is focused", model.Cursor)
			}
		})
	}
}

func TestOutputFocusIgnoresTaskRunShortcuts(t *testing.T) {
	t.Run("run", func(t *testing.T) {
		var spec runpkg.Spec
		model := NewModel(Options{
			Command:  "test",
			Targets:  []core.Target{{ID: "api", RelPath: "api", Selected: true}},
			startRun: fakeStart(&fakeActiveRun{}, &spec),
		})
		model.Focus = FocusLogs

		model, cmd := updateSpecialKey(model, tea.KeyEnter)

		if cmd != nil || model.Running || spec.Command != "" {
			t.Fatalf("output-focused run changed state: cmd=%v running=%t spec=%#v", cmd != nil, model.Running, spec)
		}
	})

	t.Run("rerun failed", func(t *testing.T) {
		model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
		model.Focus = FocusLogs
		model.Status["api"] = core.StatusFailed

		model, _ = updateKey(model, "R")

		if model.ConfirmRun {
			t.Fatal("output-focused rerun should not open task run confirmation")
		}
	})
}

func TestModelFocusAndFilteredMatchNavigation(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	model, _ = updateKey(model, "tab")
	if model.Focus != FocusLogs {
		t.Fatalf("focus = %v, want preview", model.Focus)
	}
	model, _ = updateKey(model, "shift+tab")
	if model.Focus != FocusTargets {
		t.Fatalf("focus = %v, want targets", model.Focus)
	}

	model, _ = updateKey(model, "/")
	model, _ = updateKey(model, "W")
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want first filtered match", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model, _ = updateKey(model, "n")
	if model.Cursor != 2 {
		t.Fatalf("cursor = %d, want next filtered match", model.Cursor)
	}
	model, _ = updateKey(model, "N")
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want previous filtered match", model.Cursor)
	}
	if view := model.renderTargetRow(model.Cursor, model.Targets[model.Cursor], 60); !strings.Contains(view, "\x1b[") {
		t.Fatalf("filtered match should be highlighted with ANSI styling:\n%s", view)
	}

	model, _ = updateKey(model, "c")
	if model.Focus != FocusTargets || model.ShowCommand {
		t.Fatalf("c should have no action, focus/overlay = %v/%t", model.Focus, model.ShowCommand)
	}
	model, _ = updateKey(model, ":")
	if model.Focus != FocusCommand || !model.ShowCommand {
		t.Fatalf(": should open command overlay, focus/overlay = %v/%t", model.Focus, model.ShowCommand)
	}
}

func TestCommandOverlayCancelsDraftAndRestoresFocus(t *testing.T) {
	model := NewModel(Options{Command: "go test ./...", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusLogs
	model, _ = updateKey(model, ":")
	model = typeText(model, " -race")
	if model.Command != "go test ./... -race" {
		t.Fatalf("edited command = %q", model.Command)
	}

	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowCommand || model.Focus != FocusLogs || model.Command != "go test ./..." {
		t.Fatalf("cancel state = overlay %t, focus %v, command %q", model.ShowCommand, model.Focus, model.Command)
	}
}

func TestCommandOverlayEnterRunsAndCloses(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateKey(model, ":")
	model = typeText(model, "echo ok")
	model, cmd := updateSpecialKey(model, tea.KeyEnter)
	if cmd == nil || model.ShowCommand || model.Focus != FocusTargets {
		t.Fatalf("submit state = cmd %v, overlay %t, focus %v", cmd, model.ShowCommand, model.Focus)
	}
}

func TestCommandOverlayRejectsEmptyCommandWithoutClosing(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateKey(model, ":")
	model, cmd := updateSpecialKey(model, tea.KeyEnter)
	if cmd != nil || !model.ShowCommand || model.RunError != "command is empty" {
		t.Fatalf("empty submit = cmd %v, overlay %t, error %q", cmd, model.ShowCommand, model.RunError)
	}
}

func TestModifiedKeyTextPreservesCommandOverlayBindings(t *testing.T) {
	model := NewModel(Options{Command: "echo original", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: 'P', Text: "P", Mod: tea.ModCtrl}))
	model = updated.(Model)
	if !model.ShowPalette {
		t.Fatal("ctrl+p with text payload should open command palette")
	}

	model.ShowPalette = false
	model.openCommandOverlay()
	updated, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'U', Text: "U", Mod: tea.ModCtrl}))
	model = updated.(Model)
	if model.Command != "" {
		t.Fatalf("ctrl+u with text payload should clear command, got %q", model.Command)
	}
}

func TestMouseWheelMovesFocusedTaskSelectionOneVisibleTarget(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "hidden", RelPath: "hidden", ParentID: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Targets[0].Children = []string{"hidden"}
	model.Targets[0].Folded = true

	updated, _ := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(Model)
	if model.Cursor != 2 {
		t.Fatalf("cursor after wheel down = %d, want next visible target 2", model.Cursor)
	}

	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	model = updated.(Model)
	if model.Cursor != 0 {
		t.Fatalf("cursor after wheel up = %d, want previous visible target 0", model.Cursor)
	}

	targets := make([]core.Target, 20)
	for i := range targets {
		targets[i] = core.Target{ID: fmt.Sprintf("target-%d", i), RelPath: fmt.Sprintf("target-%d", i)}
	}
	model = NewModel(Options{Targets: targets})
	model.Height = 20
	model.Cursor = 10
	model.ensureDirectoryOffset()
	initialOffset := model.DirectoryOffset
	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(Model)
	if model.Cursor != 11 || model.DirectoryOffset < initialOffset {
		t.Fatalf("wheel beyond viewport = cursor %d, offset %d; want cursor 11 and offset >= %d", model.Cursor, model.DirectoryOffset, initialOffset)
	}
}

func TestMouseWheelScrollsFocusedOutputThreeLines(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusLogs
	model.Height = 32
	model.LogFollow = true
	lines := make([]string, 80)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i+1)
	}
	model.Logs["api"] = strings.Join(lines, "\n")

	updated, _ := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	model = updated.(Model)
	if model.outputViewport.YOffset() != 49 || model.LogFollow {
		t.Fatalf("wheel up output state = offset %d, follow %t; want 49, false", model.outputViewport.YOffset(), model.LogFollow)
	}

	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(Model)
	if model.outputViewport.YOffset() != 52 {
		t.Fatalf("wheel down output offset = %d, want 52", model.outputViewport.YOffset())
	}

	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	model = updated.(Model)
	if model.outputViewport.YOffset() != 49 {
		t.Fatalf("wheel up after overscroll offset = %d, want 49", model.outputViewport.YOffset())
	}
}

func TestMouseWheelIsIgnoredOutsidePaneInteraction(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
	}{
		{name: "command input", setup: func(m *Model) { m.Focus = FocusCommand }},
		{name: "filter input", setup: func(m *Model) { m.Focus = FocusFilter }},
		{name: "help overlay", setup: func(m *Model) { m.ShowHelp = true }},
		{name: "history overlay", setup: func(m *Model) { m.ShowHistory = true }},
		{name: "palette", setup: func(m *Model) { m.ShowPalette = true }},
		{name: "options", setup: func(m *Model) { m.ShowOptions = true }},
		{name: "run confirmation", setup: func(m *Model) { m.ConfirmRun = true }},
		{name: "cancel confirmation", setup: func(m *Model) { m.ConfirmCancelAll = true }},
		{name: "quit confirmation", setup: func(m *Model) { m.ConfirmQuit = true }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(Options{Targets: []core.Target{
				{ID: "api", RelPath: "api", Selected: true},
				{ID: "web", RelPath: "web", Selected: true},
			}})
			model.outputViewport.SetContentLines(strings.Split(strings.Repeat("line\n", 20), "\n"))
			model.outputViewport.SetHeight(5)
			model.outputViewport.SetYOffset(6)
			tt.setup(&model)

			updated, _ := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
			model = updated.(Model)
			if model.Cursor != 0 || model.outputViewport.YOffset() != 6 {
				t.Fatalf("wheel changed state: cursor %d, offset %d", model.Cursor, model.outputViewport.YOffset())
			}
		})
	}
}

func TestKeyboardOutputScrollKeepsExistingTailBehavior(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusLogs
	model.LogFollow = true
	model.Logs["api"] = strings.Repeat("line\n", 80)

	model, _ = updateKey(model, "pagedown")
	if model.outputViewport.YOffset() != 5 || model.LogFollow {
		t.Fatalf("keyboard scroll state = offset %d, follow %t; want 5, false", model.outputViewport.YOffset(), model.LogFollow)
	}
}

func TestPhysicalPageKeysPauseOutputFollow(t *testing.T) {
	for _, key := range []tea.KeyPressMsg{
		{Code: tea.KeyPgUp},
		{Code: tea.KeyPgDown},
	} {
		model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
		model.Focus = FocusLogs
		model.LogFollow = true
		model.Logs["api"] = strings.Repeat("line\n", 80)

		updated, _ := model.Update(key)
		model = updated.(Model)
		if model.LogFollow {
			t.Fatalf("physical %q should pause output follow", key.String())
		}
	}
}

func TestTabFocusTogglesVisiblePanels(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	model.Focus = FocusCommand
	model, _ = updateKey(model, "tab")
	if model.Focus != FocusTargets {
		t.Fatalf("tab from command focus = %v, want tasks", model.Focus)
	}

	model.Focus = FocusFilter
	model, _ = updateKey(model, "tab")
	if model.Focus != FocusTargets {
		t.Fatalf("tab from filter focus = %v, want tasks", model.Focus)
	}

	model.Focus = FocusTargets
	model, _ = updateKey(model, "tab")
	if model.Focus != FocusLogs {
		t.Fatalf("tab from tasks focus = %v, want preview", model.Focus)
	}

	model, _ = updateKey(model, "tab")
	if model.Focus != FocusTargets {
		t.Fatalf("tab from preview focus = %v, want tasks", model.Focus)
	}
}

func TestMouseClickFocusesSplitPaneWithoutChangingTaskState(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web"},
	}})
	model.Width = 120
	model.Height = 26
	model.Cursor = 1
	model.Notice = "keep me"

	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	updated, _ := model.Update(tea.MouseClickMsg{X: 71, Y: panelTop, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.Focus != FocusLogs {
		t.Fatalf("output border click focus = %v, want output", model.Focus)
	}
	if model.Cursor != 1 || !model.Targets[0].Selected || model.Targets[1].Selected {
		t.Fatalf("output click changed task state: cursor=%d targets=%#v", model.Cursor, model.Targets)
	}
	if model.Notice != "keep me" {
		t.Fatalf("output click notice = %q, want unchanged", model.Notice)
	}

	updated, _ = model.Update(tea.MouseClickMsg{X: 0, Y: 23, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.Focus != FocusTargets {
		t.Fatalf("tasks border click focus = %v, want tasks", model.Focus)
	}
}

func TestPaneFocusAccountsForPersistentContextAndOptionalFilterRows(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Width = 120
	model.Height = 26

	if _, hit := model.paneFocusAt(1, 0); hit {
		t.Fatal("persistent context row should not hit background panel")
	}
	if focus, hit := model.paneFocusAt(1, 1); !hit || focus != FocusTargets {
		t.Fatalf("first panel row below context = (%v, %t), want tasks hit", focus, hit)
	}

	model.Focus = FocusFilter
	if _, hit := model.paneFocusAt(1, 0); hit {
		t.Fatal("filter row should not hit background panel")
	}
	if focus, hit := model.paneFocusAt(1, 4); !hit || focus != FocusTargets {
		t.Fatalf("first panel row below filter = (%v, %t), want tasks hit", focus, hit)
	}
}

func TestMouseClickIgnoresUnavailablePanes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*Model)
		click tea.MouseClickMsg
		want  Focus
	}{
		{
			name:  "right button",
			click: tea.MouseClickMsg{X: 80, Y: 10, Button: tea.MouseRight},
			want:  FocusTargets,
		},
		{
			name:  "panel gap",
			click: tea.MouseClickMsg{X: 51, Y: 10, Button: tea.MouseLeft},
			want:  FocusTargets,
		},
		{
			name: "overlay",
			setup: func(m *Model) {
				m.ShowHelp = true
			},
			click: tea.MouseClickMsg{X: 80, Y: 10, Button: tea.MouseLeft},
			want:  FocusTargets,
		},
		{
			name: "compact output remains visible",
			setup: func(m *Model) {
				m.Width = 95
				m.Focus = FocusLogs
			},
			click: tea.MouseClickMsg{X: 0, Y: 5, Button: tea.MouseLeft},
			want:  FocusLogs,
		},
		{
			name: "compact tasks receives focus from command",
			setup: func(m *Model) {
				m.Width = 95
				m.Focus = FocusCommand
			},
			click: tea.MouseClickMsg{X: 50, Y: 10, Button: tea.MouseLeft},
			want:  FocusTargets,
		},
		{
			name: "zoomed output remains visible",
			setup: func(m *Model) {
				m.Zoom = true
				m.Focus = FocusLogs
			},
			click: tea.MouseClickMsg{X: 0, Y: 5, Button: tea.MouseLeft},
			want:  FocusLogs,
		},
		{
			name: "zoomed tasks receives focus from filter",
			setup: func(m *Model) {
				m.Zoom = true
				m.Focus = FocusFilter
			},
			click: tea.MouseClickMsg{X: 50, Y: 10, Button: tea.MouseLeft},
			want:  FocusTargets,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api"}}})
			model.Width = 120
			model.Height = 26
			if tt.setup != nil {
				tt.setup(&model)
			}

			updated, _ := model.Update(tt.click)
			if got := updated.(Model).Focus; got != tt.want {
				t.Fatalf("focus = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestViewEnablesMouseCellMotion(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	view := model.View()
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("mouse mode = %v, want cell motion", view.MouseMode)
	}
}

func TestFooterIsContextual(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	tasksFooter := stripANSI(model.renderFooter(120))
	if got := len(strings.Split(tasksFooter, "\n")); got != 1 {
		t.Fatalf("tasks footer lines = %d, want 1:\n%s", got, tasksFooter)
	}
	wantTasksFooter := "[:] Command  [space] Select  [/] Filter  [o] Options  [x] Cancel  [tab] Output  [?] Help  [q] Quit"
	if got := strings.TrimSpace(tasksFooter); got != wantTasksFooter {
		t.Fatalf("tasks footer = %q, want %q", got, wantTasksFooter)
	}
	normalizedTasksFooter := strings.Join(strings.Fields(tasksFooter), " ")
	for _, want := range []string{"[:] Command", "[space] Select", "[/] Filter", "[o] Options", "[x] Cancel", "[tab] Output", "[?] Help", "[q] Quit"} {
		if !strings.Contains(normalizedTasksFooter, want) {
			t.Fatalf("tasks footer should contain %q:\n%s", want, tasksFooter)
		}
	}
	for _, hidden := range []string{"tab focus", "c Command", "a Toggle", "h/l Fold", "a Select All", "←/→ Fold"} {
		if strings.Contains(tasksFooter, hidden) {
			t.Fatalf("tasks footer should not use stale label %q:\n%s", hidden, tasksFooter)
		}
	}
	compactTasksFooter := stripANSI(model.renderFooter(80))
	if got := len(strings.Split(compactTasksFooter, "\n")); got != 1 {
		t.Fatalf("compact tasks footer lines = %d, want 1:\n%s", got, compactTasksFooter)
	}
	wantCompactTasksFooter := "[:] Cmd  [space] Sel  [o] Opts  [x] Stop  [tab] Pane  [?] Help  [q] Quit"
	if got := strings.TrimSpace(compactTasksFooter); got != wantCompactTasksFooter {
		t.Fatalf("80-column tasks footer = %q, want %q", got, wantCompactTasksFooter)
	}
	if got := maxLineWidth(compactTasksFooter); got > 80 {
		t.Fatalf("compact tasks footer width = %d:\n%s", got, compactTasksFooter)
	}
	narrowTasksFooter := stripANSI(model.renderFooter(60))
	normalizedNarrowFooter := strings.Join(strings.Fields(narrowTasksFooter), " ")
	wantNarrowFooter := "[:] Cmd  [space] Sel  [o] Op  [tab] Out  [?] Help  [q] Quit"
	if got := strings.TrimSpace(narrowTasksFooter); got != wantNarrowFooter {
		t.Fatalf("60-column tasks footer = %q, want %q", got, wantNarrowFooter)
	}
	for _, want := range []string{"[:] Cmd", "[space] Sel", "[o] Op", "[tab] Out", "[?] Help", "[q] Quit"} {
		if !strings.Contains(normalizedNarrowFooter, want) {
			t.Fatalf("narrow tasks footer should contain %q:\n%s", want, narrowTasksFooter)
		}
	}
	if strings.Contains(narrowTasksFooter, "~") {
		t.Fatalf("narrow tasks footer should not expose clipped labels:\n%s", narrowTasksFooter)
	}
	if got := maxLineWidth(narrowTasksFooter); got > 60 {
		t.Fatalf("narrow tasks footer width = %d:\n%s", got, narrowTasksFooter)
	}

	model.Status["api"] = core.StatusFailed
	failedFooter := stripANSI(model.renderFooter(120))
	if !strings.Contains(strings.Join(strings.Fields(failedFooter), " "), "[R] Failed") {
		t.Fatalf("failed footer should expose rerun failed shortcut:\n%s", failedFooter)
	}

	model.Status["api"] = core.StatusRunning
	model.Targets = append(model.Targets, core.Target{ID: "web", RelPath: "web", Selected: true})
	model.Status["web"] = core.StatusFailed
	activeFooter := stripANSI(model.renderFooter(120))
	if strings.Contains(strings.Join(strings.Fields(activeFooter), " "), "[R] Failed") {
		t.Fatalf("active footer should hide rerun failed while work is active:\n%s", activeFooter)
	}
	if !strings.Contains(strings.Join(strings.Fields(activeFooter), " "), "[q] Quit") {
		t.Fatalf("active footer should keep quit hint visible:\n%s", activeFooter)
	}

	filterModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	filterModel.Focus = FocusFilter
	filterFooter := stripANSI(filterModel.renderFooter(120))
	for _, want := range []string{"[type] Fuzzy", "['] Exact", "[n/N] Matches", "[ctrl+u] Clear", "[enter/esc] Tasks", "[?] Help"} {
		if !strings.Contains(strings.Join(strings.Fields(filterFooter), " "), want) {
			t.Fatalf("filter footer should contain %q:\n%s", want, filterFooter)
		}
	}
	compactFilterFooter := stripANSI(filterModel.renderFooter(80))
	for _, want := range []string{"[type]", "[']", "[n/N]", "[ctrl+u]", "[enter/esc]", "[?]"} {
		if !strings.Contains(strings.Join(strings.Fields(compactFilterFooter), " "), want) {
			t.Fatalf("compact filter footer should contain %q:\n%s", want, compactFilterFooter)
		}
	}
	if got := maxLineWidth(compactFilterFooter); got > 80 {
		t.Fatalf("compact filter footer width = %d:\n%s", got, compactFilterFooter)
	}
	filterModel.Focus = FocusTargets
	filterModel.Filter = "api"
	filteredTasksFooter := stripANSI(filterModel.renderFooter(120))
	if !strings.Contains(strings.Join(strings.Fields(filteredTasksFooter), " "), "[esc] Clear filter") {
		t.Fatalf("filtered task footer should expose filter clearing shortcut:\n%s", filteredTasksFooter)
	}
	if !strings.Contains(strings.Join(strings.Fields(filteredTasksFooter), " "), "[space] Select") {
		t.Fatalf("filtered task footer should preserve selection shortcut:\n%s", filteredTasksFooter)
	}

	historyModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	historyModel.ShowHistory = true
	emptyHistoryFooter := stripANSI(historyModel.renderFooter(120))
	normalizedEmptyHistoryFooter := strings.Join(strings.Fields(emptyHistoryFooter), " ")
	if !strings.Contains(normalizedEmptyHistoryFooter, "No run") || strings.Contains(normalizedEmptyHistoryFooter, "[enter] Inspect") {
		t.Fatalf("empty history footer should not promise reuse:\n%s", emptyHistoryFooter)
	}
	historyModel.History = []string{"go test"}
	historyModel.HistoryTab = historyTabCommands
	historyFooter := stripANSI(historyModel.renderFooter(120))
	if !strings.Contains(strings.Join(strings.Fields(historyFooter), " "), "[enter] Reuse") {
		t.Fatalf("history footer should expose reuse when selectable:\n%s", historyFooter)
	}
	for _, line := range strings.Split(stripANSIForWidth(historyModel.renderFooter(140)), "\n") {
		if got := lipgloss.Width(line); got != 140 {
			t.Fatalf("history footer line width = %d, want 140:\n%s", got, line)
		}
	}

	commandModel := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	commandModel.openCommandOverlay()
	commandFooter := stripANSI(commandModel.renderFooter(80))
	for _, want := range []string{"enter", "esc", "up/down", "?"} {
		if !strings.Contains(strings.Join(strings.Fields(commandFooter), " "), want) {
			t.Fatalf("compact command footer should contain %q:\n%s", want, commandFooter)
		}
	}
	narrowCommandFooter := stripANSI(commandModel.renderFooter(60))
	if strings.Contains(narrowCommandFooter, "~") {
		t.Fatalf("narrow command footer should not clip labels:\n%s", narrowCommandFooter)
	}
	if got := maxLineWidth(commandFooter); got > 80 {
		t.Fatalf("compact command footer width = %d:\n%s", got, commandFooter)
	}

	logsModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	logsModel.Focus = FocusLogs
	logsFooter := stripANSI(logsModel.renderFooter(80))
	for _, want := range []string{"pgup/pgdn", "f", "tab", "?", "q"} {
		if !strings.Contains(strings.Join(strings.Fields(logsFooter), " "), want) {
			t.Fatalf("compact logs footer should contain %q:\n%s", want, logsFooter)
		}
	}
	if got := maxLineWidth(logsFooter); got > 80 {
		t.Fatalf("compact logs footer width = %d:\n%s", got, logsFooter)
	}
}

func assertFooterColumnsAligned(t *testing.T, footer string) {
	t.Helper()
	lines := strings.Split(footer, "\n")
	if len(lines) != 3 {
		t.Fatalf("footer lines = %d, want 3:\n%s", len(lines), footer)
	}
	for _, group := range [][]string{
		{"Keymap", "Search", "Command"},
		{"History", "Panels", "Maximize"},
		{"Select", "Select All", "Fold"},
	} {
		column := footerTextColumn(lines[0], group[0])
		if column < 0 {
			t.Fatalf("missing %q in footer:\n%s", group[0], footer)
		}
		for i := 1; i < len(group); i++ {
			if got := footerTextColumn(lines[i], group[i]); got != column {
				t.Fatalf("footer column for %q = %d, want %d:\n%s", group[i], got, column, footer)
			}
		}
	}
	for _, group := range [][]string{
		{"?", "/", ":"},
		{"H", "tab", "z"},
		{"space", "a", "←/→"},
	} {
		column := footerTokenColumn(lines[0], group[0])
		if column < 0 {
			t.Fatalf("missing %q in footer:\n%s", group[0], footer)
		}
		for i := 1; i < len(group); i++ {
			if got := footerTokenColumn(lines[i], group[i]); got != column {
				t.Fatalf("footer key column for %q = %d, want %d:\n%s", group[i], got, column, footer)
			}
		}
	}
}

func footerTokenColumn(line string, token string) int {
	fields := strings.Fields(line)
	offset := 0
	for _, field := range fields {
		index := strings.Index(line[offset:], field)
		if index < 0 {
			return -1
		}
		offset += index
		if field == token {
			return offset
		}
		offset += len(field)
	}
	return -1
}

func footerTextColumn(line string, text string) int {
	index := strings.Index(line, text)
	if index < 0 {
		return -1
	}
	return lipgloss.Width(line[:index])
}

func TestFooterShortcutColorsUseTrueColorAndInheritedBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	footer := model.renderFooter(80)

	for _, want := range []string{
		"\x1b[38;2;224;224;224m",
		"\x1b[38;2;196;181;253m",
		"\x1b[1m[space]",
	} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer should contain ANSI sequence %q:\n%q", want, footer)
		}
	}
	if containsANSIBackground(footer) {
		t.Fatalf("footer should inherit terminal background:\n%q", footer)
	}
}

func TestNonSelectionChromeInheritsTerminalBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	styles := map[string]lipgloss.Style{
		"command prompt": commandPromptStyle,
		"overlay title":  overlayTitleStyle,
		"error message":  errorBarStyle,
		"running row":    rowRunningStyle,
	}
	for name, style := range styles {
		background := style.GetBackground()
		if _, inherited := background.(lipgloss.NoColor); !inherited {
			t.Errorf("%s defines explicit background %T %v", name, background, background)
		}
	}

	model := NewModel(Options{Command: "echo ok"})
	model.Width = 100
	model.Height = 24
	model.Focus = FocusFilter

	views := map[string]string{
		"filter row":      model.renderSubHeader(100),
		"footer":          model.renderFooter(100),
		"command overlay": model.renderCommandOverlay(100, 20),
	}
	model.Focus = FocusTargets
	model.RunError = "boom"
	views["error message"] = model.renderDashboard(100)
	model.RunError = ""
	model.Notice = "saved"
	views["notice message"] = model.renderDashboard(100)

	for name, rendered := range views {
		if containsANSIBackground(rendered) {
			t.Errorf("%s paints ANSI background: %q", name, rendered)
		}
	}
}

var ansiSGRPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

func containsANSIBackground(value string) bool {
	for _, match := range ansiSGRPattern.FindAllStringSubmatch(value, -1) {
		for _, parameter := range strings.Split(match[1], ";") {
			code, err := strconv.Atoi(parameter)
			if err != nil {
				continue
			}
			if code == 48 || code >= 40 && code <= 47 || code >= 100 && code <= 107 {
				return true
			}
		}
	}
	return false
}

func TestContainsANSIBackground(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "foreground", value: "\x1b[38;2;196;181;253mtext\x1b[0m"},
		{name: "basic", value: "\x1b[44mtext\x1b[0m", want: true},
		{name: "bright", value: "\x1b[104mtext\x1b[0m", want: true},
		{name: "indexed", value: "\x1b[48;5;17mtext\x1b[0m", want: true},
		{name: "true color with bold", value: "\x1b[1;48;2;1;2;3mtext\x1b[0m", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := containsANSIBackground(test.value); got != test.want {
				t.Fatalf("containsANSIBackground() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestFooterBracketsRemainVisibleWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	footer := strings.TrimSpace(model.renderFooter(80))
	want := "[:] Cmd  [space] Sel  [o] Opts  [x] Stop  [tab] Pane  [?] Help  [q] Quit"
	if footer != want {
		t.Fatalf("plain footer = %q, want %q", footer, want)
	}
}

func TestRerunFailedShortcutFeedback(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateKey(model, "R")
	if model.ConfirmRun {
		t.Fatal("R should not confirm when no failures exist")
	}
	if model.Notice != "no failed targets to rerun" {
		t.Fatalf("notice = %q", model.Notice)
	}

	model.Status["api"] = core.StatusFailed
	model, _ = updateKey(model, "R")
	if !model.ConfirmRun {
		t.Fatal("R should open confirmation when failures exist")
	}

	model = NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusQueued
	model, _ = updateKey(model, "R")
	if model.ConfirmRun {
		t.Fatal("R should not confirm while queued work exists")
	}
	if model.Notice != "finish or cancel active run before rerun failed" {
		t.Fatalf("notice = %q", model.Notice)
	}
}

func TestHelpMentionsPaletteCommands(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	help := stripANSI(strings.Join(model.helpRows(), "\n"))
	for _, want := range []string{"run", "options", "workers N|auto", "rerun-failed", "cancel", "cancel-all", "history"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help should mention palette command %q:\n%s", want, help)
		}
	}
}

func TestHelpMentionsHistoryKeys(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	help := stripANSI(strings.Join(model.helpRows(), "\n"))
	for _, want := range []string{"H open history", "[/] switch runs/commands", "/ search active tab", "enter inspect run or logs", "R rerun historical failures"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help should mention history key %q:\n%s", want, help)
		}
	}
}

func TestHelpRowsPutActiveContextFirst(t *testing.T) {
	targets := []core.Target{{ID: "api", RelPath: "api", Selected: true}}
	cases := []struct {
		name  string
		setup func(*Model)
		want  string
	}{
		{name: "tasks", want: "Tasks"},
		{name: "filter", setup: func(m *Model) { m.Focus = FocusFilter }, want: "Input and filter"},
		{name: "command", setup: func(m *Model) { m.Focus = FocusCommand }, want: "Input and filter"},
		{name: "output", setup: func(m *Model) { m.Focus = FocusLogs }, want: "Output"},
		{name: "palette", setup: func(m *Model) { m.ShowPalette = true }, want: "Palette"},
		{name: "options", setup: func(m *Model) { m.ShowOptions = true }, want: "Options"},
		{name: "history", setup: func(m *Model) { m.ShowHistory = true }, want: "History"},
		{name: "run", setup: func(m *Model) { m.Status["api"] = core.StatusRunning }, want: "Runs and status"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := NewModel(Options{Command: "test", Targets: targets})
			if tc.setup != nil {
				tc.setup(&model)
			}
			rows := strings.Split(stripANSI(strings.Join(model.helpRows(), "\n")), "\n")
			if len(rows) == 0 || rows[0] != tc.want {
				t.Fatalf("first help row = %q, want %q\n%s", rows[0], tc.want, strings.Join(rows, "\n"))
			}
		})
	}
}

func TestHelpRowsUseNarrowColumnLayout(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	rows := stripANSI(strings.Join(model.helpRows(80), "\n"))
	if got := maxLineWidth(rows); got > 76 {
		t.Fatalf("narrow help rows width = %d, want <= 76:\n%s", got, rows)
	}
}

func TestFooterReflectsActiveOverlay(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	model.ShowHelp = true
	helpFooter := stripANSI(model.renderFooter(120))
	normalizedHelpFooter := normalizeFooterText(helpFooter)
	for _, want := range []string{"[?] Close", "[esc] Close", "[H] History"} {
		if !strings.Contains(normalizedHelpFooter, want) {
			t.Fatalf("help footer should contain %q:\n%s", want, helpFooter)
		}
	}
	if strings.Contains(normalizedHelpFooter, "[space] Select") {
		t.Fatalf("help footer should not show task actions:\n%s", helpFooter)
	}

	model.ShowHelp = false
	model.ShowPalette = true
	paletteFooter := stripANSI(model.renderFooter(120))
	normalizedPaletteFooter := normalizeFooterText(paletteFooter)
	for _, want := range []string{"[enter] Choose", "[up/down] Choose", "[ctrl+u] Clear", "[esc] Close"} {
		if !strings.Contains(normalizedPaletteFooter, want) {
			t.Fatalf("palette footer should contain %q:\n%s", want, paletteFooter)
		}
	}

	model.ShowPalette = false
	model.ShowHistory = true
	model.History = []string{"go test"}
	model.RunHistory = []history.RunEntry{{Command: "go test", Total: 1}}
	historyFooter := stripANSI(model.renderFooter(120))
	normalizedHistoryFooter := normalizeFooterText(historyFooter)
	for _, want := range []string{"[enter] Inspect", "[r] Reuse", "[R] Rerun failed"} {
		if !strings.Contains(normalizedHistoryFooter, want) {
			t.Fatalf("history footer should contain %q:\n%s", want, historyFooter)
		}
	}

	model.ShowHistory = false
	model.ConfirmCancelAll = true
	cancelFooter := stripANSI(model.renderFooter(120))
	normalizedCancelFooter := normalizeFooterText(cancelFooter)
	for _, want := range []string{"[y] Confirm", "[enter] Confirm", "[n] Cancel", "[esc] Cancel"} {
		if !strings.Contains(normalizedCancelFooter, want) {
			t.Fatalf("cancel-all footer should contain %q:\n%s", want, cancelFooter)
		}
	}

	model.ConfirmCancelAll = false
	model.ConfirmQuit = true
	quitFooter := stripANSI(model.renderFooter(120))
	normalizedQuitFooter := normalizeFooterText(quitFooter)
	for _, want := range []string{"[tab] Switch", "[enter] Choose", "[y] Yes", "[n] No", "[esc] Cancel"} {
		if !strings.Contains(normalizedQuitFooter, want) {
			t.Fatalf("quit footer should contain %q:\n%s", want, quitFooter)
		}
	}
}

func TestQuestionMarkOpensHelpFromInputModes(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Command = "go test"
	model.Focus = FocusCommand
	model, _ = updateKey(model, "?")
	if !model.ShowHelp {
		t.Fatal("? should open help from command mode")
	}
	if model.Command != "go test" {
		t.Fatalf("command = %q, want unchanged", model.Command)
	}

	model = NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusFilter
	model.Filter = "api"
	model, _ = updateKey(model, "?")
	if !model.ShowHelp {
		t.Fatal("? should open help from filter mode")
	}
	if model.Filter != "api" {
		t.Fatalf("filter = %q, want unchanged", model.Filter)
	}
}

func TestQuestionMarkOpensHelpOverPaletteAndEscReturnsToPalette(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateCtrlKey(model, 'p')
	model, _ = updateKey(model, "h")
	model, _ = updateKey(model, "?")
	if !model.ShowHelp || !model.ShowPalette {
		t.Fatalf("help/palette = %v/%v, want both active", model.ShowHelp, model.ShowPalette)
	}
	if model.Palette != "h" {
		t.Fatalf("palette = %q, want unchanged", model.Palette)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowHelp {
		t.Fatal("esc should close help")
	}
	if !model.ShowPalette {
		t.Fatal("esc from help over palette should keep palette open")
	}
}

func TestHelpOverHistoryDoesNotTriggerHiddenHistoryActions(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./...", "pnpm test"}
	model.RunHistory = []history.RunEntry{{Command: "go test ./...", Total: 1}, {Command: "pnpm test", Total: 1}}
	model, _ = updateKey(model, "H")
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.HistoryPos != 1 {
		t.Fatalf("history pos = %d, want 1", model.HistoryPos)
	}

	model, _ = updateKey(model, "?")
	if !model.ShowHelp || !model.ShowHistory {
		t.Fatalf("help/history = %v/%v, want both active", model.ShowHelp, model.ShowHistory)
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.HistoryPos != 1 {
		t.Fatalf("hidden history should not move while help is open, pos = %d", model.HistoryPos)
	}
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.ShowHelp {
		t.Fatal("enter should close help")
	}
	if !model.ShowHistory {
		t.Fatal("history should remain open after closing help")
	}
	if model.Command != "echo ok" {
		t.Fatalf("enter in help should not reuse hidden history command, command = %q", model.Command)
	}
}

func TestHelpOverConfirmationDoesNotConfirmHiddenAction(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusFailed
	model, _ = updateKey(model, "R")
	if !model.ConfirmRun {
		t.Fatal("R should open rerun confirmation")
	}
	model, _ = updateKey(model, "?")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.ShowHelp {
		t.Fatal("enter should close help")
	}
	if !model.ConfirmRun {
		t.Fatal("confirmation should remain open after closing help")
	}
	if model.Running {
		t.Fatal("enter in help should not confirm hidden rerun")
	}
}

func TestZoomTogglesFocusedPanel(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateWindowSize(model, 100, 24)
	model, _ = updateKey(model, "z")
	if !model.Zoom || model.Notice != "zoom enabled" {
		t.Fatalf("zoom/notice = %v/%q", model.Zoom, model.Notice)
	}
	tasksZoom := stripANSI(model.View().Content)
	if strings.Contains(tasksZoom, "╭─ Output") {
		t.Fatalf("tasks zoom should hide output:\n%s", tasksZoom)
	}

	model.Focus = FocusLogs
	logsZoom := stripANSI(model.View().Content)
	if !strings.Contains(logsZoom, "╔═ Output") || strings.Contains(logsZoom, "╭─ Tasks") || strings.Contains(logsZoom, "╔═ Tasks") {
		t.Fatalf("logs zoom should show only output:\n%s", logsZoom)
	}
	model, _ = updateKey(model, "z")
	if model.Zoom {
		t.Fatal("z should return to split view")
	}
}

func TestCompactModeUsesSingleFocusedPanel(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateWindowSize(model, 80, 24)
	tasks := stripANSI(model.View().Content)
	if strings.Contains(tasks, "╭─ Output") {
		t.Fatalf("compact tasks should hide output:\n%s", tasks)
	}
	model.Focus = FocusLogs
	output := stripANSI(model.View().Content)
	if !strings.Contains(output, "╔═ Output") || strings.Contains(output, "╭─ Tasks") || strings.Contains(output, "╔═ Tasks") {
		t.Fatalf("compact logs should show only output:\n%s", output)
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
	if view := stripANSI(model.renderSubHeader(100)); !strings.Contains(view, "w▌") {
		t.Fatalf("filter focus should show cursor:\n%s", view)
	}
	model, _ = updateKey(model, " ")
	model, _ = updateKey(model, "x")
	model, _ = updateKey(model, "ctrl+w")
	if model.Filter != "w" {
		t.Fatalf("filter = %q, want w after ctrl+w", model.Filter)
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
	model, _ = updateKey(model, "ctrl+u")
	if model.Filter != "" {
		t.Fatalf("filter = %q, want cleared", model.Filter)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.Focus != FocusTargets {
		t.Fatal("escape should leave filter focus")
	}
}

func TestFilterFocusArrowsNavigateMatches(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model, _ = updateKey(model, "w")
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want first match", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Cursor != 2 {
		t.Fatalf("cursor = %d, want next match from filter focus", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want previous match from filter focus", model.Cursor)
	}
	footer := stripANSI(model.renderFooter(120))
	if !strings.Contains(normalizeFooterText(footer), "[n/N] Matches") {
		t.Fatalf("filter footer should mention arrow match navigation:\n%s", footer)
	}
}

func TestTargetFooterShortcutLabels(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
	}})
	footer := stripANSI(model.renderFooter(140))

	for _, want := range []string{"[space] Select", "[/] Filter", "[x] Cancel", "[tab] Output", "[?] Help", "[q] Quit"} {
		if !strings.Contains(normalizeFooterText(footer), want) {
			t.Fatalf("footer should contain %q:\n%s", want, footer)
		}
	}
	for _, hidden := range []string{"c Command", "a Toggle", "h/l Fold"} {
		if strings.Contains(footer, hidden) {
			t.Fatalf("footer should hide stale shortcut %q:\n%s", hidden, footer)
		}
	}
}

func normalizeFooterText(footer string) string {
	return strings.Join(strings.Fields(footer), " ")
}

func TestPanelPrefixShowsPersistentRunContextOutsideFilter(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
	}})
	model.Cursor = 1
	prefix := stripANSI(model.renderPanelPrefix(140))
	for _, want := range []string{"command: test", "targets: 2/2 selected", "mode: parallel", "workers: auto"} {
		if !strings.Contains(prefix, want) {
			t.Fatalf("panel prefix should keep run context %q visible:\n%s", want, prefix)
		}
	}
	if strings.Count(prefix, "\n")+1 != 1 {
		t.Fatalf("run context should use one row outside filter mode:\n%s", prefix)
	}
}

func TestOverlaysUseFullPanelHeightWithoutRedundantRunContext(t *testing.T) {
	model := NewModel(Options{Command: "pnpm test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateWindowSize(model, 120, 40)
	model.ShowHelp = true

	if prefix := model.renderPanelPrefix(120); prefix != "" {
		t.Fatalf("overlay should replace redundant run context, got %q", stripANSI(prefix))
	}
	view := stripANSI(model.View().Content)
	for _, want := range []string{"H open history", "[/] switch runs/commands", "/ search active tab"} {
		if !strings.Contains(view, want) {
			t.Fatalf("120x40 help should retain %q when overlay is open:\n%s", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines != 40 {
		t.Fatalf("overlay view lines = %d, want 40:\n%s", lines, view)
	}
}

func TestSubHeaderIsEmptyOutsideFilter(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	if rendered := model.renderSubHeader(80); rendered != "" {
		t.Fatalf("non-filter subheader = %q, want empty", rendered)
	}
}

func TestFilterInputOmitsSlashAndPlaceholder(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateKey(model, "/")

	empty := stripANSI(model.View().Content)
	if strings.Contains(empty, "/ <filter>") {
		t.Fatalf("empty filter input should not show slash or placeholder:\n%s", empty)
	}
	if !strings.Contains(empty, "▌") {
		t.Fatalf("empty focused filter should show cursor:\n%s", empty)
	}

	model, _ = updateKey(model, "a")
	model, _ = updateKey(model, "p")
	model, _ = updateKey(model, "i")
	filled := stripANSI(model.View().Content)
	if strings.Contains(filled, "/ api▌") {
		t.Fatalf("filled filter input should not show slash:\n%s", filled)
	}
	if !strings.Contains(filled, "api▌") {
		t.Fatalf("filled filter input should show text and cursor:\n%s", filled)
	}
}

func TestFilterInputDisplayGolden(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateKey(model, "/")
	empty := model.commandInputValue()

	model, _ = updateKey(model, "a")
	model, _ = updateKey(model, "p")
	model, _ = updateKey(model, "i")
	filled := model.commandInputValue()

	want, err := os.ReadFile("testdata/TestFilterInputDisplayGolden.golden")
	if err != nil {
		t.Fatal(err)
	}
	got := "empty:\n" + empty + "\n\nfilled:\n" + filled
	if got != strings.TrimRight(string(want), "\n") {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestOperatorLayoutUsesCompactPersistentChrome(t *testing.T) {
	model := NewModel(Options{Command: "pnpm test", Workers: 4, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusFailed

	prefix := stripANSI(strings.TrimSpace(model.renderPanelPrefix(120)))
	if strings.Count(prefix, "\n") != 0 {
		t.Fatalf("operator run context should use one row:\n%s", prefix)
	}
	for _, want := range []string{"command: pnpm test", "targets: 2/2 selected", "mode: parallel", "workers: 4"} {
		if !strings.Contains(prefix, want) {
			t.Fatalf("operator run context should contain %q:\n%s", want, prefix)
		}
	}

	footer := strings.Join(strings.Fields(stripANSI(model.renderFooter(120))), " ")
	if strings.Count(stripANSI(model.renderFooter(120)), "\n") != 0 {
		t.Fatalf("footer should use one row:\n%s", footer)
	}
	for _, want := range []string{"[space] Select", "[/] Filter", "[x] Cancel", "[tab] Output", "[?] Help", "[q] Quit"} {
		if !strings.Contains(footer, want) {
			t.Fatalf("compact footer should contain %q:\n%s", want, footer)
		}
	}
}

func TestOperatorLayoutPlacesRunSummaryBelowPanelsAndFitsWindow(t *testing.T) {
	model := NewModel(Options{Command: "pnpm test", Workers: 4, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
		{ID: "docs", RelPath: "docs", Selected: true},
	}})
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusQueued
	model.Status["worker"] = core.StatusSucceeded
	model.Status["docs"] = core.StatusFailed
	model, _ = updateWindowSize(model, 120, 32)

	view := stripANSI(model.View().Content)
	lines := strings.Split(view, "\n")
	if len(lines) != 32 {
		t.Fatalf("operator layout lines = %d, want 32:\n%s", len(lines), view)
	}
	summary := "● 1 running · ◌ 1 queued · ✓ 1 ok · × 1 failed"
	summaryIndex := strings.Index(view, summary)
	panelsIndex := strings.LastIndex(view, "╯")
	if summaryIndex < 0 || panelsIndex < 0 || summaryIndex < panelsIndex {
		t.Fatalf("run summary should sit below panels:\n%s", view)
	}
}

func TestCommandFocusAcceptsSlashAndSpace(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.openCommandOverlay()
	model, _ = updateKey(model, ".")
	model, _ = updateKey(model, "/")
	model, _ = updateKey(model, "s")
	model, _ = updateKey(model, "h")
	model, _ = updateKey(model, " ")
	model, _ = updateKey(model, "-")
	model, _ = updateKey(model, "c")
	if model.Command != "./sh -c" {
		t.Fatalf("command = %q", model.Command)
	}
	model, _ = updateKey(model, "ctrl+w")
	if model.Command != "./sh" {
		t.Fatalf("command = %q, want ./sh after ctrl+w", model.Command)
	}
	model, _ = updateKey(model, " ")
	model, _ = updateKey(model, "-")
	model, _ = updateKey(model, "c")
	if model.Focus != FocusCommand {
		t.Fatalf("focus = %v, want command", model.Focus)
	}
	if view := stripANSI(model.renderCommandOverlay(100, 20)); !strings.Contains(view, "./sh -c") {
		t.Fatalf("command overlay should show command input:\n%s", view)
	}
	if view := model.renderCommandOverlay(100, 20); model.commandCursor != len([]rune(model.Command)) {
		t.Fatalf("command overlay should show terminal cursor:\n%s", stripANSI(view))
	}
	model, _ = updateKey(model, "ctrl+u")
	if model.Command != "" {
		t.Fatalf("command = %q, want cleared", model.Command)
	}
	if model.Notice != "command cleared" {
		t.Fatalf("notice = %q, want command cleared", model.Notice)
	}
}

func TestCommandFocusShowsTrailingSpaceImmediately(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.openCommandOverlay()
	model = typeText(model, "echo")
	model, _ = updateSpecialKey(model, tea.KeySpace)

	view := model.renderCommandOverlay(100, 20)
	if !strings.Contains(stripANSI(view), "echo  ") || model.Command != "echo " || model.commandCursor != len([]rune(model.Command)) {
		t.Fatalf("command focus should show cursor after trailing space:\n%s", stripANSI(view))
	}
}

func TestCommandFocusNavigatesHistory(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.openCommandOverlay()
	model.History = []string{"go test ./...", "pnpm test"}
	model = typeText(model, "draft")

	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Command != "go test ./..." {
		t.Fatalf("command = %q, want newest history", model.Command)
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Command != "pnpm test" {
		t.Fatalf("command = %q, want older history", model.Command)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Command != "go test ./..." {
		t.Fatalf("command = %q, want newer history", model.Command)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Command != "draft" {
		t.Fatalf("command = %q, want restored draft", model.Command)
	}

	model, _ = updateSpecialKey(model, tea.KeyUp)
	model, _ = updateKey(model, "x")
	if model.CommandHistoryPos != -1 || model.Command != "go test ./...x" {
		t.Fatalf("history navigation should reset after edit, pos/command = %d/%q", model.CommandHistoryPos, model.Command)
	}
}

func TestCommandHistoryNavigationResetsOnRunStart(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.openCommandOverlay()
	model.History = []string{"go test ./..."}
	model = typeText(model, "draft")
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.CommandHistoryPos != 0 {
		t.Fatalf("history pos = %d, want active navigation", model.CommandHistoryPos)
	}

	model, cmd := updateSpecialKey(model, tea.KeyEnter)
	if cmd == nil {
		t.Fatal("enter should start run")
	}
	if model.CommandHistoryPos != -1 || model.CommandDraft != "" {
		t.Fatalf("history navigation should reset on run start, pos/draft = %d/%q", model.CommandHistoryPos, model.CommandDraft)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Command != "go test ./..." {
		t.Fatalf("down after run start should not restore stale draft, command = %q", model.Command)
	}
}

func TestModelFilterKeepsParentContext(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model, _ = updateKey(model, "c")
	model, _ = updateKey(model, "m")
	model, _ = updateKey(model, "d")

	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	if strings.Count(view, "api") < 2 || !strings.Contains(view, "api/cmd") {
		t.Fatalf("filtered directory panel should keep parent context:\n%s", view)
	}
	if strings.Contains(view, "web") {
		t.Fatalf("filtered view should hide non-matching sibling:\n%s", view)
	}
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want direct matching child", model.Cursor)
	}
	if panel := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n")); !strings.Contains(panel, "showing 1-2 of 2") {
		t.Fatalf("directory panel should show filtered visible range:\n%s", panel)
	}
}

func TestFilterMatchNavigationSkipsContextParents(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd", "api/pkg"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
		{ID: "api/pkg", RelPath: "api/pkg", ParentID: "api", Depth: 2, Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model = typeText(model, "api/")
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want first direct match", model.Cursor)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model, _ = updateKey(model, "n")
	if model.Cursor != 2 {
		t.Fatalf("cursor = %d, want next direct match, skipping parent context", model.Cursor)
	}
	if panel := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n")); !strings.Contains(panel, "showing 1-3 of 3") {
		t.Fatalf("directory panel should show filtered visible range:\n%s", panel)
	}
}

func TestFilterSupportsFuzzyAndExactModes(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model = typeText(model, "acm")

	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want fuzzy child match", model.Cursor)
	}
	view := strings.Join(model.renderDirectoryPanel(80, 12), "\n")
	if !strings.Contains(stripANSI(view), "api/cmd") {
		t.Fatalf("fuzzy filter should reveal child with parent context:\n%s", stripANSI(view))
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("fuzzy match should be highlighted:\n%s", view)
	}

	exact := NewModel(Options{Command: "test", Targets: model.Targets})
	exact.Filter = "'acm"
	exact.ensureCursorVisible()
	if matches := exact.matchingTargetIndexes(); len(matches) != 0 {
		t.Fatalf("exact filter should not use fuzzy matching, got %v", matches)
	}
	exactView := stripANSI(strings.Join(exact.renderDirectoryPanel(80, 10), "\n"))
	if !strings.Contains(exactView, "No matches for /'acm") {
		t.Fatalf("exact no-match state should mention exact query:\n%s", exactView)
	}
}

func TestFilteredBulkSelectionTargetsMatchesOnly(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: false, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: false},
		{ID: "web", RelPath: "web", Selected: false},
	}})
	model, _ = updateKey(model, "/")
	model = typeText(model, "cmd")
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model, _ = updateKey(model, "a")

	if model.Targets[0].Selected {
		t.Fatal("context parent should stay unselected when selecting filtered matches")
	}
	if !model.Targets[1].Selected {
		t.Fatal("direct filtered match should be selected")
	}
	if model.Targets[2].Selected {
		t.Fatal("hidden sibling should stay unselected")
	}
	if model.Notice != "selected 1 matching target(s)" {
		t.Fatalf("notice = %q", model.Notice)
	}

	model.Targets[0].Selected = true
	model, _ = updateKey(model, "a")
	if !model.Targets[0].Selected {
		t.Fatal("context parent should stay selected when deselecting filtered matches")
	}
	if model.Targets[1].Selected {
		t.Fatal("direct filtered match should be deselected")
	}
	if model.Notice != "deselected 1 matching target(s)" {
		t.Fatalf("notice = %q", model.Notice)
	}
}

func TestFilterRevealsMatchesInsideFoldedParents(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Folded: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model, _ = updateKey(model, "/")
	model = typeText(model, "cmd")

	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	for _, want := range []string{"api", "api/cmd", "▾"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filter should reveal folded match/context %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "▸") {
		t.Fatalf("fold marker should show effective expanded state while filtering:\n%s", view)
	}
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d, want folded child match", model.Cursor)
	}
	if panel := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n")); !strings.Contains(panel, "showing 1-2 of 2") {
		t.Fatalf("directory panel should include folded child match:\n%s", panel)
	}
}

func TestDirectoryPanelShowsFilterEmptyState(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
	}})
	model.Filter = "web"
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))
	for _, want := range []string{"No matches for /web", "fuzzy filter has no visible target", "edit query, ctrl+u clears filter, esc returns to tasks"} {
		if !strings.Contains(view, want) {
			t.Fatalf("empty filter state should contain %q:\n%s", want, view)
		}
	}

	model.Filter = "'web"
	view = stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))
	if !strings.Contains(view, "exact filter has no visible target") {
		t.Fatalf("exact empty filter state should explain exact mode:\n%s", view)
	}
}

func TestDirectoryPanelShowsNoTargetsOnboarding(t *testing.T) {
	model := NewModel(Options{Command: "test"})
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))
	for _, want := range []string{"No target directories found", "child directories", "project root"} {
		if !strings.Contains(view, want) {
			t.Fatalf("no-target onboarding should contain %q:\n%s", want, view)
		}
	}
}

func TestDirectoryPanelScrollsToCursor(t *testing.T) {
	targets := make([]core.Target, 0, 18)
	for i := 0; i < 18; i++ {
		id := "svc-" + string(rune('a'+i))
		targets = append(targets, core.Target{ID: id, RelPath: id, Selected: true})
	}
	model := NewModel(Options{Command: "test", Targets: targets})
	model, _ = updateWindowSize(model, 80, 20)
	for i := 0; i < 14; i++ {
		model, _ = updateSpecialKey(model, tea.KeyDown)
	}

	view := stripANSI(model.render())
	if !strings.Contains(view, "› [●]") || !strings.Contains(view, "svc-o") {
		t.Fatalf("directory panel should scroll focused row into view:\n%s", view)
	}
	if strings.Contains(view, "svc-a") {
		t.Fatalf("directory panel should scroll past first rows:\n%s", view)
	}
	for _, want := range []string{"showing", "of 18", "↑", "↓"} {
		if !strings.Contains(view, want) {
			t.Fatalf("directory panel should show scroll marker %q:\n%s", want, view)
		}
	}
}

func TestDirectoryPanelShowsRangeWithoutScrollMarkersWhenFullyVisible(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	if !strings.Contains(view, "showing 1-2 of 2 •") {
		t.Fatalf("directory panel should show full visible range:\n%s", view)
	}
}

func TestDirectoryPanelRangeMatchesRenderedRows(t *testing.T) {
	targets := make([]core.Target, 0, 8)
	for i := 0; i < 8; i++ {
		id := "svc-" + string(rune('a'+i))
		targets = append(targets, core.Target{ID: id, RelPath: id, Selected: true})
	}
	model := NewModel(Options{Command: "test", Targets: targets})
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 8), "\n"))
	if !strings.Contains(view, "showing 1-3 of 8 ↓") {
		t.Fatalf("directory panel range should match rendered target rows:\n%s", view)
	}
	if count := strings.Count(view, "svc-"); count != 3 {
		t.Fatalf("rendered target rows = %d, want 3:\n%s", count, view)
	}
}

func TestDirectoryPanelHeaderIsSelfExplanatory(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	wide := stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))
	for _, want := range []string{"DIRECTORY", "STATUS"} {
		if !strings.Contains(wide, want) {
			t.Fatalf("wide task header should contain %q:\n%s", want, wide)
		}
	}
	for _, unwanted := range []string{"RUN", "FOLD", "SEL"} {
		if strings.Contains(wide, unwanted) {
			t.Fatalf("wide task header should not contain %q:\n%s", unwanted, wide)
		}
	}

	compact := stripANSI(model.taskHeader(46))
	for _, want := range []string{"DIRECTORY", "STATUS"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("compact task header should contain %q:\n%s", want, compact)
		}
	}
	for _, unwanted := range []string{"RUN", "FOLD", "SEL"} {
		if strings.Contains(compact, unwanted) {
			t.Fatalf("compact task header should not contain %q:\n%s", unwanted, compact)
		}
	}
	if got := maxLineWidth(compact); got > 46 {
		t.Fatalf("compact task header width = %d:\n%s", got, compact)
	}
}

func TestTargetRowsAlignStatusColumn(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusRunning
	width := 60
	header := stripANSI(model.taskHeader(width))
	row := stripANSI(model.renderTargetRow(0, model.Targets[0], width))
	headerIndex := strings.Index(header, "STATUS")
	statusIndex := strings.Index(row, model.statusLabel(core.StatusRunning))
	if headerIndex < 0 || statusIndex < 0 || lipgloss.Width(header[:headerIndex]) != lipgloss.Width(row[:statusIndex]) {
		t.Fatalf("status column header=%d row=%d\n%s\n%s", headerIndex, statusIndex, header, row)
	}
	prefix := strings.Replace(row[:statusIndex], "[●]", "", 1)
	if strings.Contains(prefix, "●") || strings.Contains(prefix, "○") {
		t.Fatalf("target row should not include status marker before status column:\n%s", row)
	}
}

func TestTargetTreeUsesDepthIndentation(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Children: []string{"api/cmd", "api/pkg"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Children: []string{"api/cmd/foo"}},
		{ID: "api/cmd/foo", RelPath: "api/cmd/foo", ParentID: "api/cmd", Depth: 3},
		{ID: "api/pkg", RelPath: "api/pkg", ParentID: "api", Depth: 2},
	}})
	if got := model.treeGuidePlain(model.Targets[2]); got != "    " {
		t.Fatalf("depth-three target indentation = %q, want four spaces", got)
	}
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	if strings.ContainsAny(view, "├└│─") {
		t.Fatalf("target tree should use indentation without connector glyphs:\n%s", view)
	}
	if strings.ContainsAny(view, "📁📂") {
		t.Fatalf("target tree should not render folder icons:\n%s", view)
	}
}

func TestTargetRowsHighlightSelectedAndPartialSubtrees(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Children: []string{"api/cmd", "api/pkg"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
		{ID: "api/pkg", RelPath: "api/pkg", ParentID: "api", Depth: 2},
		{ID: "web", RelPath: "web", Selected: true},
	}})

	partial := model.renderTargetRow(0, model.Targets[0], 70)
	selected := model.renderTargetRow(3, model.Targets[3], 70)
	if partial == stripANSI(partial) {
		t.Fatalf("partial subtree parent should be styled:\n%s", partial)
	}
	if selected == stripANSI(selected) {
		t.Fatalf("selected target row should be styled:\n%s", selected)
	}
}

func TestTargetRowsKeepSelectionHighlightUnderFocus(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})

	model.Cursor = 0
	focusedSelected := model.renderTargetRow(0, model.Targets[0], 70)
	model.Cursor = 1
	unfocusedSelected := model.renderTargetRow(0, model.Targets[0], 70)
	if focusedSelected == stripANSI(focusedSelected) {
		t.Fatalf("focused selected target should remain styled:\n%s", focusedSelected)
	}
	if !strings.Contains(focusedSelected, "\x1b[1;") && !strings.Contains(focusedSelected, "\x1b[1m") {
		t.Fatalf("focused selected target should keep selected emphasis:\n%q", focusedSelected)
	}
	if unfocusedSelected == stripANSI(unfocusedSelected) || !strings.Contains(stripANSI(unfocusedSelected), "[●]") {
		t.Fatalf("unfocused selected target should keep explicit selected marker:\n%q", unfocusedSelected)
	}
	if !noColorEnabled() {
		if selectedStyle.GetForeground() != runnyTheme.fgEmphasis {
			t.Fatalf("selected marker foreground = %v, want white emphasis %v", selectedStyle.GetForeground(), runnyTheme.fgEmphasis)
		}
		if selectedStyle.GetForeground() == runnyTheme.success {
			t.Fatal("selected marker should not use success green")
		}
	}
}

func TestTargetStatusColorsSurviveSelectionAndFocus(t *testing.T) {
	tests := []struct {
		name   string
		status core.Status
		color  string
	}{
		{name: "running", status: core.StatusRunning, color: "#FBBF24"},
		{name: "ok", status: core.StatusSucceeded, color: "#4ADE80"},
		{name: "failed", status: core.StatusFailed, color: "#FB7185"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusStyles[tt.status].GetForeground(); got != tuiColor(tt.color) {
				t.Fatalf("%s foreground = %v, want %v", tt.name, got, tuiColor(tt.color))
			}
			if background := statusStyles[tt.status].GetBackground(); background != nil {
				if _, inherited := background.(lipgloss.NoColor); !inherited {
					t.Fatalf("%s status defines background %T %v", tt.name, background, background)
				}
			}
			model := NewModel(Options{Command: "test", Targets: []core.Target{
				{ID: "focused", RelPath: "focused", Selected: true},
				{ID: "selected", RelPath: "selected", Selected: true},
			}})
			model.Status["focused"] = tt.status
			model.Status["selected"] = tt.status
			model.Cursor = 0

			for name, row := range map[string]string{
				"focused":  model.renderTargetRow(0, model.Targets[0], 70),
				"selected": model.renderTargetRow(1, model.Targets[1], 70),
			} {
				want := model.renderRowStatus(tt.status)
				if !strings.Contains(row, want) {
					t.Fatalf("%s %s row missing semantic status rendering: %q", name, tt.name, row)
				}
			}
		})
	}
}

func TestNavigationHighlightDiffersFromSelectionHighlight(t *testing.T) {
	selectedModel := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web"},
	}})
	selectedModel.Cursor = 1

	selected := selectedModel.renderTargetRow(0, selectedModel.Targets[0], 70)
	if strings.Contains(selected, ansiBackgroundHex(primaryAccentHex)) {
		t.Fatalf("selection should not paint the full row background:\n%q", selected)
	}
	if rowActiveStyle.GetBackground() != runnyTheme.bgFocus {
		t.Fatalf("navigation highlight should use focus background")
	}
	if rowActiveSelectedStyle.GetBackground() != runnyTheme.bgFocus {
		t.Fatalf("focused selected target should use navigation highlight")
	}
	if runnyTheme.bgFocus != runnyTheme.bgSelection {
		t.Fatalf("navigation highlight should use purple selection background")
	}
	if strings.Contains(selected, "\x1b[4m") || strings.Contains(selected, ";4m") {
		t.Fatalf("selected highlight should not use underline:\n%q", selected)
	}
}

func TestFoldKeysDoNotAddMarkerToLeafTarget(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api"}}})

	model, _ = updateSpecialKey(model, tea.KeyLeft)
	if model.Targets[0].Folded {
		t.Fatalf("leaf target should not become folded: %#v", model.Targets[0])
	}
	row := stripANSI(model.renderTargetRow(0, model.Targets[0], 70))
	if strings.ContainsAny(row, "▸▾") {
		t.Fatalf("leaf target should not show fold marker:\n%s", row)
	}
}

func TestTargetTreeUsesDisclosureChevrons(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "expanded", RelPath: "expanded", Children: []string{"expanded/child"}},
		{ID: "expanded/child", RelPath: "expanded/child", ParentID: "expanded", Depth: 2},
		{ID: "folded", RelPath: "folded", Folded: true, Children: []string{"folded/child"}},
	}})
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	for _, want := range []string{"▾ expanded", "▸ folded", "expanded/child"} {
		if !strings.Contains(view, want) {
			t.Fatalf("target tree missing %q:\n%s", want, view)
		}
	}
}

func TestTargetRowsRenderIndependentCursorAndSelectionMarkers(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: false},
	}})
	model.Cursor = 0
	selected := stripANSI(model.renderTargetRow(0, model.Targets[0], 70))
	unselected := stripANSI(model.renderTargetRow(1, model.Targets[1], 70))

	if !strings.Contains(selected, "› [●]") {
		t.Fatalf("focused selected row should show cursor and selection independently:\n%s", selected)
	}
	if !strings.Contains(unselected, "  [ ]") || strings.Contains(unselected, "›") {
		t.Fatalf("unfocused unselected row should show empty selection only:\n%s", unselected)
	}
}

func TestOverlaysTrapCancelKeys(t *testing.T) {
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
	if model.Status["api"] != core.StatusRunning {
		t.Fatalf("status = %s", model.Status["api"])
	}
	if model.Notice != "" {
		t.Fatalf("notice = %q, want no background cancellation", model.Notice)
	}
}

func TestModelCommandPaletteConfiguresExecution(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateCtrlKey(model, 'p')
	model = typeText(model, "HIST")
	if matches := model.filteredPaletteCommands(); len(matches) == 0 || matches[0].Name != "history" {
		t.Fatalf("palette should match case-insensitively: %#v", matches)
	}
	model.Palette = "rf"
	if matches := model.filteredPaletteCommands(); len(matches) == 0 || matches[0].Name != "rerun-failed" {
		t.Fatalf("palette should match commands fuzzily: %#v", matches)
	}
	model.Palette = "'rf"
	if matches := model.filteredPaletteCommands(); len(matches) != 0 {
		t.Fatalf("exact palette query should not use fuzzy matching: %#v", matches)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model, _ = updateCtrlKey(model, 'p')
	model, _ = updateKey(model, "q")
	if !model.ShowPalette || model.Palette != "q" {
		t.Fatalf("q should type inside command palette, show/palette = %v/%q", model.ShowPalette, model.Palette)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowPalette {
		t.Fatal("escape should close command palette")
	}

	model, _ = runPaletteCommand(model, "command")
	if model.Focus != FocusCommand || !model.ShowCommand {
		t.Fatalf("palette command should open command overlay, focus/overlay = %v/%t", model.Focus, model.ShowCommand)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model.Focus = FocusTargets

	model, _ = runPaletteCommand(model, "workers")
	if model.RunError != "usage: workers N|auto" {
		t.Fatalf("run error = %q, want workers usage", model.RunError)
	}

	model, _ = runPaletteCommand(model, "workers nope")
	if model.RunError != "workers must be >= 1 or auto" {
		t.Fatalf("run error = %q, want workers value guidance", model.RunError)
	}

	model, _ = runPaletteCommand(model, "workers 1")
	if model.ShowPalette {
		t.Fatal("command palette should close after executing command")
	}
	if model.Workers != 1 || model.Mode != core.ModeParallel {
		t.Fatalf("workers/mode = %d/%s, want 1/parallel", model.Workers, model.Mode)
	}
	if model.Notice != "workers set to 1" {
		t.Fatalf("notice = %q", model.Notice)
	}

	model, _ = runPaletteCommand(model, "workers auto")
	if model.Workers != 0 || model.Mode != core.ModeParallel {
		t.Fatalf("workers/mode = %d/%s, want auto/parallel", model.Workers, model.Mode)
	}
	if model.Notice != "workers set to auto" {
		t.Fatalf("notice = %q", model.Notice)
	}

	model, _ = runPaletteCommand(model, "serial")
	if model.Workers != 0 || model.Mode != core.ModeSerial {
		t.Fatalf("workers/mode = %d/%s, want 0/serial", model.Workers, model.Mode)
	}

	model, _ = runPaletteCommand(model, "parallel")
	if model.Mode != core.ModeParallel {
		t.Fatalf("mode = %s, want parallel", model.Mode)
	}
}

func TestPaletteEnterNoMatchKeepsPaletteOpen(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateCtrlKey(model, 'p')
	model = typeText(model, "zzzz")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if !model.ShowPalette {
		t.Fatal("no-match enter should keep palette open")
	}
	if model.RunError != "no palette command matches" {
		t.Fatalf("run error = %q", model.RunError)
	}
	view := stripANSI(model.View().Content)
	for _, want := range []string{"Command palette", "0 matches", "no palette command matches"} {
		if !strings.Contains(view, want) {
			t.Fatalf("palette no-match view should contain %q:\n%s", want, view)
		}
	}

	model, _ = updateSpecialKey(model, tea.KeyBackspace)
	if model.RunError != "" {
		t.Fatalf("editing palette should clear no-match error, got %q", model.RunError)
	}
}

func TestModelCommandPaletteRunsAndOpensOverlays(t *testing.T) {
	model := NewModel(Options{
		Command:  "echo ok",
		Targets:  []core.Target{{ID: "api", RelPath: "api", Selected: true}},
		startRun: fakeStart(&fakeActiveRun{}, nil),
	})
	model, cmd := runPaletteCommand(model, "run")
	if cmd == nil {
		t.Fatal(":run should start selected targets")
	}
	if !model.Running || model.Status["api"] != core.StatusQueued {
		t.Fatalf("running/status = %v/%s", model.Running, model.Status["api"])
	}

	model = NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Filter = "api"
	model, _ = runPaletteCommand(model, "clear-filter")
	if model.Filter != "" {
		t.Fatalf("filter = %q, want cleared", model.Filter)
	}

	model, _ = runPaletteCommand(model, "history")
	if !model.ShowHistory {
		t.Fatal(":history should open history overlay")
	}

	model = NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = runPaletteCommand(model, "hist")
	if !model.ShowHistory {
		t.Fatal(":hist should execute selected history suggestion")
	}

	model = NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Status["api"] = core.StatusFailed
	model.Status["web"] = core.StatusQueued
	model, _ = runPaletteCommand(model, "rerun-failed")
	if model.ConfirmRun {
		t.Fatal(":rerun-failed should wait until queued work is gone")
	}
}

func TestModelCommandPaletteCancelsSelectedTarget(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	run := &fakeActiveRun{cancelResults: [][]string{{"api"}}}
	model.Running = true
	model.Status["api"] = core.StatusRunning
	model.activeRun = run

	model, _ = runPaletteCommand(model, "cancel")
	if run.cancelCalls != 1 {
		t.Fatal(":cancel should call target cancel function")
	}
	if model.Status["api"] != core.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", model.Status["api"])
	}
}

func TestCancelSelectedRequiresConfirmationForMultipleActiveTargets(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "docs", RelPath: "docs", Selected: false},
	}})
	run := &fakeActiveRun{cancelResults: [][]string{{"api", "web"}}}
	model.Running = true
	model.activeRun = run
	for _, target := range model.Targets {
		model.Status[target.ID] = core.StatusRunning
	}

	model, _ = updateKey(model, "x")
	if run.cancelCalls != 0 {
		t.Fatalf("x should wait for confirmation, calls = %d", run.cancelCalls)
	}
	view := stripANSI(model.View().Content)
	for _, want := range []string{"Cancel selected", "2 selected active target(s)", "targets: api, web", "y/enter confirm", "n/esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("selected cancellation confirmation should contain %q:\n%s", want, view)
		}
	}

	model, _ = updateKey(model, "y")
	if run.cancelCalls != 1 || model.Status["api"] != core.StatusCancelled || model.Status["web"] != core.StatusCancelled {
		t.Fatalf("confirmation should cancel selected active targets only: calls=%d statuses=%#v", run.cancelCalls, model.Status)
	}
	if model.Status["docs"] != core.StatusRunning {
		t.Fatalf("unselected target status = %s, want running", model.Status["docs"])
	}
}

func TestCancelSelectedConfirmationNeverFallsBackAfterAsyncCompletion(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "docs", RelPath: "docs"},
	}})
	run := &fakeActiveRun{cancelResults: [][]string{nil}}
	model.Running = true
	model.activeRun = run
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusRunning
	model.Status["docs"] = core.StatusRunning

	model, _ = updateKey(model, "x")
	model.Status["api"] = core.StatusSucceeded
	model.Status["web"] = core.StatusSucceeded
	model.Cursor = 2
	model, _ = updateKey(model, "y")

	if run.cancelCalls != 1 || model.Status["docs"] != core.StatusRunning {
		t.Fatalf("stale selected confirmation must not cancel focused unselected target: calls=%d status=%s", run.cancelCalls, model.Status["docs"])
	}
}

func TestModelCommandPaletteConfirmsCancelAll(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	run := &fakeActiveRun{cancelResults: [][]string{{"api", "web"}}}
	model.Running = true
	model.activeRun = run
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusQueued

	model, _ = runPaletteCommand(model, "cancel-all")
	if !model.ConfirmCancelAll {
		t.Fatal(":cancel-all should open confirmation while work is active")
	}
	if model.Status["api"] != core.StatusRunning || model.Status["web"] != core.StatusQueued {
		t.Fatalf(":cancel-all should wait for confirmation before cancelling: %#v", model.Status)
	}
	view := stripANSI(model.View().Content)
	for _, want := range []string{"Cancel all", "2 active target(s)", "breakdown: 1 running, 1 queued", "scope: running and queued targets only", "targets: api, web", "y/enter confirm", "n/esc cancel"} {
		if !strings.Contains(view, want) {
			t.Fatalf("cancel-all confirmation should contain %q:\n%s", want, view)
		}
	}

	model, _ = updateKey(model, "n")
	if run.cancelCalls != 0 || model.Status["api"] != core.StatusRunning || model.Status["web"] != core.StatusQueued {
		t.Fatalf("n should cancel confirmation only, calls/statuses = %d/%#v", run.cancelCalls, model.Status)
	}
	if model.Notice != "confirmation cancelled" {
		t.Fatalf("notice = %q", model.Notice)
	}
	model.ConfirmCancelAll = true
	model, _ = updateKey(model, "y")
	if run.cancelCalls != 1 {
		t.Fatal("confirming cancel-all should call cancelRun")
	}
	if model.ConfirmCancelAll {
		t.Fatal("confirmation should close after y")
	}
	if model.Status["api"] != core.StatusCancelled || model.Status["web"] != core.StatusCancelled {
		t.Fatalf("statuses = %#v, want all cancelled", model.Status)
	}
}

func TestActiveTargetSummaryTruncatesLongLists(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api"},
		{ID: "web", RelPath: "web"},
		{ID: "worker", RelPath: "worker"},
		{ID: "docs", RelPath: "docs"},
	}})
	for _, target := range model.Targets {
		model.Status[target.ID] = core.StatusQueued
	}
	if got := model.activeTargetSummary(80); got != "api, web, worker, +1 more" {
		t.Fatalf("summary = %q", got)
	}
	if got := model.activeTargetSummary(10); !strings.HasSuffix(got, "~") {
		t.Fatalf("short summary should truncate, got %q", got)
	}
}

func TestModelCommandPaletteSelectsFailedTargets(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: false},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	model.Status["api"] = core.StatusSucceeded
	model.Status["web"] = core.StatusFailed
	model.Status["worker"] = core.StatusFailed

	model, _ = updateCtrlKey(model, 'p')
	model = typeText(model, "failed")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.Targets[0].Selected || !model.Targets[1].Selected || !model.Targets[2].Selected {
		t.Fatalf("selected targets = %#v", model.Targets)
	}
	if model.Notice != "selected 2 failed target(s)" {
		t.Fatalf("notice = %q", model.Notice)
	}

	model = NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusSucceeded
	model, _ = runPaletteCommand(model, "failed")
	if model.Targets[0].Selected {
		t.Fatal(":failed should clear selection when no target failed")
	}
	if model.Notice != "no failed targets to select" {
		t.Fatalf("notice = %q", model.Notice)
	}
}

func TestModelNavigationAndPreviewScrolling(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	model, _ = updateKey(model, "G")
	if model.Cursor != 2 {
		t.Fatalf("cursor = %d, want last target", model.Cursor)
	}
	model, _ = updateKey(model, "g")
	if model.Cursor != 0 {
		t.Fatalf("cursor = %d, want first target", model.Cursor)
	}

	model.Focus = FocusLogs
	model.LogFollow = true
	model.Logs["api"] = strings.Repeat("line\n", 80)
	model, _ = updateKey(model, "pagedown")
	if model.outputViewport.YOffset() != 5 {
		t.Fatalf("preview offset = %d, want 5", model.outputViewport.YOffset())
	}
	if model.LogFollow {
		t.Fatal("manual preview scroll should disable tail mode")
	}
	model, _ = updateKey(model, "f")
	if !model.LogFollow {
		t.Fatal("f should re-enable tail mode")
	}
}

func TestCtrlCShowsQuitConfirmationFromOverlay(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	run := &fakeActiveRun{cancelResults: [][]string{{"api"}}}
	model.Running = true
	model.ShowHelp = true
	model.activeRun = run
	model.Status["api"] = core.StatusRunning

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("ctrl+c should wait for confirmation before quitting")
	}
	if !model.ConfirmQuit {
		t.Fatal("ctrl+c should show quit confirmation")
	}
	if !model.ConfirmQuitYes {
		t.Fatal("quit confirmation should default to Yes")
	}
	if model.ShowHelp {
		t.Fatal("ctrl+c should replace current overlay with quit confirmation")
	}
	if run.cancelCalls != 0 {
		t.Fatal("ctrl+c should not cancel before confirmation")
	}
	if model.Status["api"] != core.StatusRunning {
		t.Fatalf("status = %s", model.Status["api"])
	}

	updatedModel, cmd := updateKey(model, "n")
	model = updatedModel
	if cmd != nil {
		t.Fatal("n should close confirmation without quitting")
	}
	if model.ConfirmQuit {
		t.Fatal("n should close quit confirmation")
	}
	if run.cancelCalls != 0 {
		t.Fatal("n should not cancel active run")
	}
}

func TestQuitConfirmationUsesTabSelectedButtons(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.ConfirmQuit = true
	model.ConfirmQuitYes = true

	view := stripANSI(model.View().Content)
	for _, want := range []string{"Quit runny?", "Yes", "No"} {
		if !strings.Contains(view, want) {
			t.Fatalf("quit confirmation should contain %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Confirm before leaving", "active targets:", "confirm quit"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("quit confirmation should not contain %q:\n%s", unwanted, view)
		}
	}
	overlay := model.renderOverlay(120, 24)
	if got := maxLineWidth(stripANSI(overlay)); got > 24 {
		t.Fatalf("quit confirmation should fit content width, got %d:\n%s", got, stripANSI(overlay))
	}
	if !strings.Contains(stripANSI(overlay), "│ [ Yes ]  [ No ] │") {
		t.Fatalf("quit confirmation border should close around choices:\n%s", stripANSI(overlay))
	}
	if strings.Contains(stripANSI(overlay), "─ Quit runny? ") {
		t.Fatalf("quit confirmation title should not interrupt border:\n%s", stripANSI(overlay))
	}
	if dangerBorderStyle.GetForeground() != runnyTheme.error {
		t.Fatalf("quit confirmation border should use error color")
	}
	if dangerChoiceStyle.GetBackground() != runnyTheme.error {
		t.Fatalf("quit confirmation selected choice should use error background")
	}

	updated, cmd := updateKey(model, "tab")
	model = updated
	if cmd != nil {
		t.Fatal("tab should not quit")
	}
	if model.ConfirmQuitYes {
		t.Fatal("tab should move selection to No")
	}

	model.ConfirmQuit = true
	model.ConfirmQuitYes = true
	updated, cmd = updateKey(model, "enter")
	model = updated
	if cmd == nil {
		t.Fatal("enter on default Yes should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("enter command = %T, want tea.QuitMsg", cmd())
	}
}

func TestQAlwaysShowsQuitConfirmation(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	updated, cmd := updateKey(model, "q")
	model = updated
	if cmd != nil || !model.ConfirmQuit || !model.ConfirmQuitYes {
		t.Fatalf("idle q should open quit confirmation on Yes, cmd=%v confirm=%v yes=%v", cmd, model.ConfirmQuit, model.ConfirmQuitYes)
	}

	model = NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Running = true
	model.Status["api"] = core.StatusRunning
	model, cmd = updateKey(model, "q")
	if cmd != nil || !model.ConfirmQuit || !model.ConfirmQuitYes {
		t.Fatalf("active q should open quit confirmation on Yes, cmd=%v confirm=%v yes=%v", cmd, model.ConfirmQuit, model.ConfirmQuitYes)
	}
}

func TestQRemainsTextInsideInputModes(t *testing.T) {
	model := NewModel(Options{Command: "echo ", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateKey(model, ":")
	model, cmd := updateKey(model, "q")
	if cmd != nil || model.Command != "echo q" || model.ConfirmQuit {
		t.Fatalf("command q should insert text, cmd=%v command=%q confirm=%v", cmd, model.Command, model.ConfirmQuit)
	}

	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model, _ = updateKey(model, "/")
	model, cmd = updateKey(model, "q")
	if cmd != nil || model.Filter != "q" || model.ConfirmQuit {
		t.Fatalf("filter q should insert text, cmd=%v filter=%q confirm=%v", cmd, model.Filter, model.ConfirmQuit)
	}
}

func TestViewUsesAltScreenAndTUIPanels(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.ShowHelp = true
	model, _ = updateWindowSize(model, 120, 32)

	view := model.View()
	if !view.AltScreen {
		t.Fatal("view should use alt screen")
	}
	plain := stripANSI(view.Content)
	for _, want := range []string{"Keymap", ": run command", "ctrl+p palette", "H history", "q quit", "del/x cancel selected"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("view content should contain %q:\n%s", want, plain)
		}
	}
	for _, want := range []string{"Runs and status", "spinner running", "◌ queued", "✕ failed", "– cancelled"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("help should contain status legend %q:\n%s", want, plain)
		}
	}
}

func TestTUIColorHonorsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if _, ok := tuiColor("#FFFFFF").(lipgloss.NoColor); !ok {
		t.Fatalf("NO_COLOR should return lipgloss.NoColor, got %T", tuiColor("#FFFFFF"))
	}
}

func TestOverlaysAreCenteredAndStyled(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.ShowHelp = true
	model, _ = updateWindowSize(model, 120, 32)
	view := model.View().Content
	plain := stripANSI(view)
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("overlay should include ANSI styling")
	}
	keymapOffset := -1
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "╭─ Keymap") {
			keymapOffset = strings.Index(line, "╭─ Keymap")
			break
		}
	}
	if keymapOffset <= 0 || keymapOffset > 40 {
		t.Fatalf("help overlay should be inset and centered, got offset %d:\n%s", keymapOffset, plain)
	}
	if got := maxLineWidth(view); got > 120 {
		t.Fatalf("help overlay width = %d, want <= 120\n%s", got, plain)
	}

	model.ShowHelp = false
	model.ShowPalette = true
	model.Palette = "hist"
	palette := stripANSI(model.View().Content)
	for _, want := range []string{"Command palette", ": hist", "history", "1 fuzzy match(es)", "enter history"} {
		if !strings.Contains(palette, want) {
			t.Fatalf("palette overlay should contain %q:\n%s", want, palette)
		}
	}
}

func TestPaletteRowsShowMatchCountAndNoMatchGuidance(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.ShowPalette = true
	model.Palette = "run"
	rows := stripANSI(strings.Join(model.paletteRows(), "\n"))
	for _, want := range []string{"6 fuzzy match(es)", "enter run", "run", "rerun-failed"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("palette rows should contain %q:\n%s", want, rows)
		}
	}
	model.Palette = ""
	paletteRows := model.paletteRows()
	rows = stripANSI(strings.Join(paletteRows, "\n"))
	if got, want := len(paletteRows), 3+len(paletteCommands); got != want {
		t.Fatalf("palette rows = %d, want %d with every command:\n%s", got, want, rows)
	}
	for _, command := range paletteCommands {
		if !strings.Contains(rows, command.Description) {
			t.Fatalf("palette rows should contain %q:\n%s", command.Description, rows)
		}
	}
	if strings.Contains(rows, "more command(s)") {
		t.Fatalf("full palette should not show a truncation hint:\n%s", rows)
	}

	model.Palette = "rf"
	renderedRows := strings.Join(model.paletteRows(), "\n")
	if !strings.Contains(stripANSI(renderedRows), "rerun-failed") {
		t.Fatalf("fuzzy palette rows should include rerun-failed:\n%s", stripANSI(renderedRows))
	}
	if !strings.Contains(renderedRows, "\x1b[") {
		t.Fatalf("fuzzy palette rows should highlight matches:\n%s", renderedRows)
	}

	model.Palette = "workers"
	rows = stripANSI(strings.Join(model.paletteRows(), "\n"))
	if strings.Contains(rows, "enter workers N|auto") || !strings.Contains(rows, "enter command") {
		t.Fatalf("argument placeholder command should show generic enter hint:\n%s", rows)
	}

	model.Palette = "zzzz"
	rows = stripANSI(strings.Join(model.paletteRows(), "\n"))
	for _, want := range []string{"0 matches", "ctrl+u clear", "no commands; backspace or ctrl+u to edit"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("no-match palette rows should contain %q:\n%s", want, rows)
		}
	}
}

func TestPaletteOverlayShowsEveryCommandAtStandardSize(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.ShowPalette = true
	model, _ = updateWindowSize(model, 80, 24)

	view := stripANSI(model.View().Content)
	for _, command := range paletteCommands {
		if !strings.Contains(view, command.Description) {
			t.Fatalf("palette overlay should contain %q at 80x24:\n%s", command.Description, view)
		}
	}
	if strings.Contains(view, "more command(s)") {
		t.Fatalf("full palette overlay should not show a truncation hint:\n%s", view)
	}
}

func TestOverlayPreservesMainPanelsBehindPopup(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.ShowHelp = true
	model, _ = updateWindowSize(model, 100, 24)

	view := stripANSI(model.View().Content)
	if !strings.Contains(view, "Keymap") {
		t.Fatalf("help overlay should render:\n%s", view)
	}
	for _, want := range []string{"Tasks", "showing"} {
		if !strings.Contains(view, want) {
			t.Fatalf("overlay should preserve background content %q:\n%s", want, view)
		}
	}
	if lines := strings.Count(view, "\n") + 1; lines > 24 {
		t.Fatalf("overlay should keep screen bounded, lines = %d:\n%s", lines, view)
	}
}

func TestPaletteOverlayPreservesPanelBorderColumns(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.ShowPalette = true
	model, _ = updateWindowSize(model, 120, 33)

	panelHeight, leftWidth, rightWidth := model.panelDimensions(120, 33)
	overlay := model.renderOverlay(120, panelHeight)
	backgroundLines := strings.Split(stripANSI(model.renderPanelArea(120, panelHeight, leftWidth, rightWidth)), "\n")
	composedLines := strings.Split(stripANSI(placeOverlay(
		model.renderPanelArea(120, panelHeight, leftWidth, rightWidth),
		overlay,
		120,
	)), "\n")

	for i := range backgroundLines {
		want := lipgloss.Width(backgroundLines[i])
		if got := lipgloss.Width(composedLines[i]); got != want {
			t.Fatalf("panel border column changed on row %d: got width %d, want %d\n%s", i, got, want, strings.Join(composedLines, "\n"))
		}
	}
}

func TestOverlaysStayBoundedAtMinimumSize(t *testing.T) {
	base := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	base.History = []string{"go test ./...", "pnpm test", "make test", "cargo test", "pytest", "zig build test", "npm run lint"}
	base.RunHistory = []history.RunEntry{
		{Command: "go test ./...", Total: 3, Succeeded: 2, Failed: 1},
		{Command: "pnpm test", Total: 2, Succeeded: 2},
	}

	cases := []struct {
		name  string
		setup func(*Model)
		want  string
	}{
		{name: "help", setup: func(m *Model) { m.ShowHelp = true }, want: "Keymap"},
		{name: "history", setup: func(m *Model) { m.ShowHistory = true }, want: "History"},
		{name: "palette", setup: func(m *Model) { m.ShowPalette = true }, want: "Command palette"},
		{name: "options", setup: func(m *Model) { m.ShowOptions = true }, want: "Options · session"},
		{name: "command", setup: func(m *Model) { m.openCommandOverlay() }, want: "Run command"},
		{name: "confirm", setup: func(m *Model) { m.ConfirmRun = true }, want: "Rerun failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := base
			tc.setup(&model)
			model, _ = updateWindowSize(model, 60, 20)
			view := stripANSI(model.View().Content)
			if !strings.Contains(view, tc.want) {
				t.Fatalf("overlay should contain %q:\n%s", tc.want, view)
			}
			if lines := strings.Count(view, "\n") + 1; lines > 20 {
				t.Fatalf("%s overlay should fit minimum height, lines = %d:\n%s", tc.name, lines, view)
			}
			if got := maxLineWidth(view); got > 60 {
				t.Fatalf("%s overlay max width = %d:\n%s", tc.name, got, view)
			}
		})
	}
}

func TestViewShowsMinimumSizeGate(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{
		{59, 24},
		{100, 19},
	} {
		model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
		model, _ = updateWindowSize(model, size.width, size.height)
		view := stripANSI(model.View().Content)
		if !strings.Contains(view, "terminal too small") {
			t.Fatalf("%dx%d should show minimum-size gate:\n%s", size.width, size.height, view)
		}
		if got := maxLineWidth(view); got > size.width {
			t.Fatalf("%dx%d line width = %d:\n%s", size.width, size.height, got, view)
		}
		if lines := strings.Count(view, "\n") + 1; lines > size.height {
			t.Fatalf("%dx%d line count = %d:\n%s", size.width, size.height, lines, view)
		}
	}
}

func TestViewBeautifulDashboardGolden(t *testing.T) {
	model := NewModel(Options{Command: "pnpm test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", Selected: true, ParentID: "api", Depth: 2},
		{ID: "web", RelPath: "web", Selected: false},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	model.Status["api"] = core.StatusRunning
	model.Status["api/cmd"] = core.StatusQueued
	model.Status["web"] = core.StatusSkipped
	model.Status["worker"] = core.StatusFailed
	model, _ = updateWindowSize(model, 100, 26)

	view := model.View()
	if !strings.Contains(view.Content, "\x1b[") {
		t.Fatal("dashboard should include ANSI styling")
	}
	if width := maxLineWidth(view.Content); width > 100 {
		t.Fatalf("max line width = %d, want <= 100\n%s", width, stripANSI(view.Content))
	}

	want, err := os.ReadFile("testdata/TestViewBeautifulDashboardGolden.golden")
	if err != nil {
		t.Fatal(err)
	}
	got := stripANSI(view.Content)
	if got != strings.TrimRight(string(want), "\n") {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestViewResponsiveWidths(t *testing.T) {
	for _, width := range []int{60, 80, 100, 120} {
		model := NewModel(Options{Command: "go test ./...", Targets: []core.Target{
			{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/internal/very-long-child-name"}},
			{ID: "api/internal/very-long-child-name", RelPath: "api/internal/very-long-child-name", ParentID: "api", Depth: 2, Selected: true},
			{ID: "web", RelPath: "web", Selected: true},
		}})
		model.Logs["api"] = strings.Repeat("long output line with useful context\n", 6)
		model.Status["api"] = core.StatusRunning
		model, _ = updateWindowSize(model, width, 24)
		if got := maxLineWidth(model.View().Content); got > width {
			t.Fatalf("width %d rendered line width = %d\n%s", width, got, stripANSI(model.View().Content))
		}
	}
}

func TestRunContextPersistsCommandScopeAndExecutionModeAtBreakpoints(t *testing.T) {
	for _, test := range []struct {
		width int
		want  []string
	}{
		{width: 120, want: []string{"command: go test ./...", "targets: 2/3 selected", "mode: parallel", "workers: 2"}},
		{width: 100, want: []string{"command: go test ./...", "targets: 2/3 selected", "mode: parallel", "workers: 2"}},
		{width: 99, want: []string{"cmd: go test ./...", "2/3 selected", "parallel/2"}},
		{width: 60, want: []string{"cmd: go test ./...", "2/3 selected", "parallel/2"}},
	} {
		t.Run(fmt.Sprintf("width_%d", test.width), func(t *testing.T) {
			model := NewModel(Options{Command: "go test ./...", Workers: 2, Targets: []core.Target{
				{ID: "api", RelPath: "api", Selected: true},
				{ID: "docs", RelPath: "docs", Selected: false},
				{ID: "web", RelPath: "web", Selected: true},
			}})
			model, _ = updateWindowSize(model, test.width, 24)
			contextLine := stripANSI(strings.Split(model.View().Content, "\n")[0])
			for _, want := range test.want {
				if !strings.Contains(contextLine, want) {
					t.Fatalf("%d-column context should contain %q:\n%s", test.width, want, contextLine)
				}
			}
		})
	}
}

func TestRunContextKeepsScopeVisibleWhenCommandIsLong(t *testing.T) {
	model := NewModel(Options{Command: strings.Repeat("very-long-command ", 12), Mode: core.ModeSerial, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: false},
	}})
	model, _ = updateWindowSize(model, 60, 24)
	contextLine := stripANSI(strings.Split(model.View().Content, "\n")[0])
	for _, want := range []string{"cmd:", "1/2 selected", "serial/1"} {
		if !strings.Contains(contextLine, want) {
			t.Fatalf("long command should preserve %q in context:\n%s", want, contextLine)
		}
	}
	if got := lipgloss.Width(contextLine); got > 60 {
		t.Fatalf("context width = %d, want <= 60:\n%s", got, contextLine)
	}
}

func TestPersistentRunContextKeepsViewHeightBounded(t *testing.T) {
	for _, size := range []struct {
		width  int
		height int
	}{{120, 40}, {100, 30}, {99, 30}, {60, 24}} {
		model := NewModel(Options{Command: "go test ./...", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
		model, _ = updateWindowSize(model, size.width, size.height)
		view := stripANSI(model.View().Content)
		if got := strings.Count(view, "\n") + 1; got > size.height {
			t.Fatalf("%dx%d rendered %d rows:\n%s", size.width, size.height, got, view)
		}
	}
}

func TestOperatorLayoutUsesSinglePaneAtSixtyColumns(t *testing.T) {
	model := NewModel(Options{Command: "go test ./...", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Logs["api"] = "live output\n"
	model.Status["api"] = core.StatusRunning
	model, _ = updateWindowSize(model, 60, 24)

	tasks := stripANSI(model.View().Content)
	if strings.Contains(tasks, "terminal too small") || !strings.Contains(tasks, "Tasks") || strings.Contains(tasks, "Output —") {
		t.Fatalf("60-column tasks view should use one usable pane:\n%s", tasks)
	}
	if got := maxLineWidth(tasks); got > 60 {
		t.Fatalf("60-column tasks width = %d:\n%s", got, tasks)
	}

	model, _ = updateSpecialKey(model, tea.KeyTab)
	output := stripANSI(model.View().Content)
	if !strings.Contains(output, "Output — api [RUN]") || strings.Contains(output, "╔═ Tasks") {
		t.Fatalf("Tab should switch 60-column view to Output:\n%s", output)
	}

	model, _ = updateWindowSize(model, 59, 24)
	if view := stripANSI(model.View().Content); !strings.Contains(view, "need at least 60x20") {
		t.Fatalf("59 columns should show minimum-size gate:\n%s", view)
	}
}

func TestOperatorLayoutSplitsTasksAndOutputFortyTwoFiftyEight(t *testing.T) {
	panelHeight, tasksWidth, outputWidth := panelDimensionsForInput(120, 32, 1)
	if panelHeight <= 0 {
		t.Fatalf("panel height = %d, want positive", panelHeight)
	}
	if tasksWidth != 50 || outputWidth != 66 {
		t.Fatalf("panel widths = %d/%d, want 50/66 for 42/58 split", tasksWidth, outputWidth)
	}
}

func TestOperatorLayoutBreakpointUsesSinglePaneBelowOneHundred(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateWindowSize(model, 99, 24)
	if view := stripANSI(model.View().Content); strings.Contains(view, "Output —") {
		t.Fatalf("99 columns should use single-pane tasks view:\n%s", view)
	}
	model, _ = updateWindowSize(model, 100, 24)
	if view := stripANSI(model.View().Content); !strings.Contains(view, "Output —") {
		t.Fatalf("100 columns should restore split view:\n%s", view)
	}
}

func TestOutputPanelShowsOnlyCommandOutput(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusFailed
	model.Logs["api"] = "ok line\nexit status 1\n"
	model.TargetStarted["api"] = time.Date(2026, 7, 3, 14, 5, 6, 0, time.Local)
	model, _ = updateWindowSize(model, 90, 24)
	view := model.renderLogPanel(72, 16)
	rendered := strings.Join(view, "\n")
	plain := stripANSI(rendered)
	for _, want := range []string{"Output — api [FAIL] · follow:on · 2 lines", "ok line", "exit status 1"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("output panel should contain %q:\n%s", want, plain)
		}
	}
	for _, unwanted := range []string{"Command to run", "14:05:06", "go test", "1 │", "2 │", "Output ("} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("output panel should not contain %q:\n%s", unwanted, plain)
		}
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("output panel should style logs:\n%s", rendered)
	}
}

func TestOutputPanelIsEmptyBeforeCommandWritesOutput(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	view := strings.Join(model.renderLogPanel(72, 18), "\n")
	plain := stripANSI(view)
	for _, unwanted := range []string{"Command to run", "--:--:--", "(not set)", "Output (empty)", "No output yet."} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("empty output panel should not contain %q:\n%s", unwanted, plain)
		}
	}
}

func TestOutputPanelStaysEmptyForRunningCommandUntilOutputArrives(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	model.Status["api"] = core.StatusQueued
	view := stripANSI(strings.Join(model.renderLogPanel(72, 18), "\n"))
	for _, unwanted := range []string{"Waiting for worker slot.", "Use :workers N", "del/x to cancel", "go test"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("queued output panel should not contain %q:\n%s", unwanted, view)
		}
	}

	model.Status["api"] = core.StatusRunning
	view = stripANSI(strings.Join(model.renderLogPanel(72, 18), "\n"))
	for _, unwanted := range []string{"Running. Output appears here", "Press f to toggle tail", "del/x to cancel this target", "go test"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("running output panel should not contain %q:\n%s", unwanted, view)
		}
	}
}

func TestRunOutputEventUpdatesRunningTarget(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", Selected: true}
	model := NewModel(Options{Targets: []core.Target{target}})
	model.Running = true
	model.Status["api"] = core.StatusRunning
	model.activeRun = &fakeActiveRun{}

	updated, next := model.Update(runEventMsg{event: targetEvent(
		runpkg.EventTargetOutputChanged, target, core.StatusRunning, "live stdout\nlive stderr\n", "",
	)})
	model = updated.(Model)

	if model.Logs["api"] != "live stdout\nlive stderr\n" {
		t.Fatalf("api logs = %q", model.Logs["api"])
	}
	if next == nil {
		t.Fatal("output event should keep waiting for target stream")
	}
}

func TestRunOutputEventProjectsBoundedLiveOutput(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", Selected: true}
	model := NewModel(Options{Targets: []core.Target{target}})
	output := runpkg.TruncatedOutputMarker + strings.Repeat("x", runpkg.MaxOutputBytes)

	updated, _ := model.Update(runEventMsg{event: runpkg.Event{
		Kind: runpkg.EventTargetOutputChanged,
		Target: &runpkg.TargetSnapshot{
			Target:          target,
			Status:          core.StatusRunning,
			OutputTail:      output,
			OutputTruncated: true,
		},
	}})
	got := updated.(Model).Logs["api"]

	if !strings.HasPrefix(got, runpkg.TruncatedOutputMarker) {
		t.Fatalf("live output prefix = %q", got[:min(len(got), len(runpkg.TruncatedOutputMarker))])
	}
	if len(got) != len(runpkg.TruncatedOutputMarker)+runpkg.MaxOutputBytes {
		t.Fatalf("live output length = %d", len(got))
	}
}

func TestRunOutputMessageFollowsTailAndPreservesManualScroll(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusRunning
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i+1)
	}

	firstOutput := strings.Join(lines, "\n") + "\n"
	updated, _ := model.Update(runEventMsg{event: targetEvent(
		runpkg.EventTargetOutputChanged, model.Targets[0], core.StatusRunning, firstOutput, "",
	)})
	model = updated.(Model)
	view := strings.Join(model.renderOutputLines("api", 80, 4), "\n")
	if !strings.Contains(view, "line-20") || strings.Contains(view, "line-01") {
		t.Fatalf("tail view did not follow live output:\n%s", view)
	}

	model.LogFollow = false
	model.syncOutputViewport()
	model.outputViewport.SetYOffset(2)
	updated, _ = model.Update(runEventMsg{event: targetEvent(
		runpkg.EventTargetOutputChanged, model.Targets[0], core.StatusRunning, firstOutput+"line-21\n", "",
	)})
	model = updated.(Model)
	if model.outputViewport.YOffset() != 2 {
		t.Fatalf("manual preview offset = %d, want 2", model.outputViewport.YOffset())
	}
	view = strings.Join(model.renderOutputLines("api", 80, 4), "\n")
	if !strings.Contains(view, "line-03") || strings.Contains(view, "line-21") {
		t.Fatalf("manual scroll moved after live output:\n%s", view)
	}
}

func TestOutputViewportPreservesPanelTruncationMarker(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Logs["api"] = "123456789"

	rows := model.renderOutputLines("api", 8, 1)
	if len(rows) != 1 {
		t.Fatalf("rendered rows = %d, want 1", len(rows))
	}
	if got := stripANSI(rows[0]); got != "1234567~" {
		t.Fatalf("rendered row = %q, want truncation marker", got)
	}
}

func TestPreviewOutputRangeShowsManualScroll(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Logs["api"] = strings.Join([]string{
		"line-01", "line-02", "line-03", "line-04", "line-05", "line-06", "line-07", "line-08",
		"line-09", "line-10", "line-11", "line-12", "line-13", "line-14", "line-15",
		"line-16", "line-17", "line-18", "line-19", "line-20",
	}, "\n")
	model.LogFollow = false
	model.syncOutputViewport()
	model.outputViewport.SetYOffset(2)

	view := stripANSI(strings.Join(model.renderLogPanel(52, 18), "\n"))
	for _, want := range []string{"line-03", "line-18"} {
		if !strings.Contains(view, want) {
			t.Fatalf("output panel should contain %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Output (", "1 │", "3 │", "18 │", "line-01", "line-19"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("output panel should not contain %q:\n%s", unwanted, view)
		}
	}
}

func TestPreviewTailShowsTailForCompletedLongLogs(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusFailed
	model.Logs["api"] = strings.Join([]string{
		"line-01", "line-02", "line-03", "line-04", "line-05", "line-06", "line-07", "line-08",
		"line-09", "line-10", "line-11", "line-12", "line-13", "line-14", "line-15",
		"line-16", "line-17", "line-18", "line-19", "line-20",
	}, "\n")
	model.LogFollow = true

	view := stripANSI(strings.Join(model.renderLogPanel(52, 18), "\n"))
	for _, want := range []string{"line-05", "line-20"} {
		if !strings.Contains(view, want) {
			t.Fatalf("completed output tail should show tail %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Output (", "5 │ line-05", "line-01", "line-04"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("completed output tail should not contain %q:\n%s", unwanted, view)
		}
	}
}

func TestRunningDashboardAndTaskActivityIndicators(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	model.Running = true
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusQueued
	model.Status["worker"] = core.StatusFailed
	dashboard := stripANSI(model.renderDashboard(120))
	for _, want := range []string{"● 1 running", "◌ 1 queued", "✓ 0 ok", "× 1 failed", "follow:on"} {
		if !strings.Contains(dashboard, want) {
			t.Fatalf("dashboard should show %q:\n%s", want, dashboard)
		}
	}
	rows := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	for _, want := range []string{"[●]", "running", "queued", "failed"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("task rows should contain activity marker %q:\n%s", want, rows)
		}
	}
}

func TestStatusRailStructurePersistsAcrossLifecycle(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web"},
	}})
	model, _ = updateWindowSize(model, 80, 24)

	tests := []struct {
		name    string
		prepare func(*Model)
		want    string
	}{
		{
			name: "initial",
			want: "● 0 running · ◌ 0 queued · ✓ 0 ok · × 0 failed",
		},
		{
			name: "selection notice",
			prepare: func(m *Model) {
				m.Targets[1].Selected = true
				m.Notice = "selected 2 target(s)"
			},
			want: "● 0 running · ◌ 0 queued · ✓ 0 ok · × 0 failed",
		},
		{
			name: "running notice",
			prepare: func(m *Model) {
				m.Status["api"] = core.StatusRunning
				m.Status["web"] = core.StatusQueued
				m.Notice = "started 2 target(s)"
			},
			want: "● 1 running · ◌ 1 queued · ✓ 0 ok · × 0 failed",
		},
		{
			name: "complete warning",
			prepare: func(m *Model) {
				m.Status["api"] = core.StatusSucceeded
				m.Status["web"] = core.StatusFailed
				m.Notice = "run complete: 1 ok, 1 failed, 0 cancelled"
			},
			want: "● 0 running · ◌ 0 queued · ✓ 1 ok · × 1 failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := model
			if test.prepare != nil {
				test.prepare(&current)
			}
			dashboard := stripANSI(current.renderDashboard(80))
			if !strings.Contains(dashboard, test.want) {
				t.Fatalf("status rail structure changed:\n%s", dashboard)
			}
			if got := maxLineWidth(dashboard); got > 80 {
				t.Fatalf("status rail width = %d, want at most 80:\n%s", got, dashboard)
			}
		})
	}
}

func TestDashboardShowsStatusCountsWithoutProgressChrome(t *testing.T) {
	targets := make([]core.Target, 0, 24)
	for i := 0; i < 24; i++ {
		id := "target-" + strconv.Itoa(i)
		targets = append(targets, core.Target{ID: id, RelPath: id, Selected: i < 9})
	}
	model := NewModel(Options{Command: "go test", Targets: targets})
	for _, target := range model.Targets[:9] {
		model.Status[target.ID] = core.StatusSucceeded
	}

	dashboard := stripANSI(model.renderDashboard(120))
	if !strings.Contains(dashboard, "✓ 9 ok") || !strings.Contains(dashboard, "follow:on") {
		t.Fatalf("dashboard should show status count and follow mode:\n%s", dashboard)
	}
	if strings.Contains(dashboard, "%") || strings.Contains(dashboard, "progress") {
		t.Fatalf("dashboard should omit progress chrome:\n%s", dashboard)
	}
}

func TestModelProjectsRunLifecycle(t *testing.T) {
	targets := []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}
	run := &fakeActiveRun{}
	var spec runpkg.Spec
	model := NewModel(Options{Command: "echo ok", Targets: targets, startRun: fakeStart(run, &spec)})

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	if cmd == nil {
		t.Fatal("enter should start a run")
	}
	if !model.Running {
		t.Fatal("model should be running")
	}
	if model.Status["api"] != core.StatusQueued || model.Status["web"] != core.StatusQueued {
		t.Fatalf("statuses = %#v", model.Status)
	}
	if spec.Command != "echo ok" || len(spec.Targets) != 2 {
		t.Fatalf("run spec = %#v", spec)
	}

	for _, event := range []runpkg.Event{
		targetEvent(runpkg.EventTargetStarted, targets[0], core.StatusRunning, "", ""),
		targetEvent(runpkg.EventTargetFinished, targets[0], core.StatusSucceeded, "api ok\n", ""),
		targetEvent(runpkg.EventTargetStarted, targets[1], core.StatusRunning, "", ""),
		targetEvent(runpkg.EventTargetFinished, targets[1], core.StatusFailed, "web bad\n", "exit status 1"),
		completedEvent("run-1", "echo ok",
			runpkg.TargetSnapshot{Target: targets[0], Status: core.StatusSucceeded, OutputTail: "api ok\n"},
			runpkg.TargetSnapshot{Target: targets[1], Status: core.StatusFailed, OutputTail: "web bad\n", Error: "exit status 1"},
		),
	} {
		updated, _ := model.Update(runEventMsg{event: event})
		model = updated.(Model)
	}
	if model.Running {
		t.Fatal("model should stop running after results")
	}
	if model.Status["api"] != core.StatusSucceeded || model.Status["web"] != core.StatusFailed {
		t.Fatalf("statuses = %#v", model.Status)
	}
	if model.Notice != "run complete: 1 ok, 1 failed, 0 cancelled · press R to rerun failed" {
		t.Fatalf("notice = %q", model.Notice)
	}
	if !strings.Contains(model.Logs["web"], "web bad") || !strings.Contains(model.Logs["web"], "exit status 1") {
		t.Fatalf("web logs = %q", model.Logs["web"])
	}
}

func TestRunEventCommandStreamsBeforeCompletion(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", Selected: true}
	run := &fakeActiveRun{events: []runpkg.Event{
		targetEvent(runpkg.EventTargetOutputChanged, target, core.StatusRunning, "live output\n", ""),
		targetEvent(runpkg.EventTargetFinished, target, core.StatusSucceeded, "live output\n", ""),
		completedEvent("run-1", "echo ok", runpkg.TargetSnapshot{Target: target, Status: core.StatusSucceeded, OutputTail: "live output\n"}),
	}}
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{target}, startRun: fakeStart(run, nil)})
	model.Running = true
	model.activeRun = run

	msg := waitForRunEvent(context.Background(), run)()
	updated, next := model.Update(msg)
	model = updated.(Model)
	if model.Logs[target.ID] != "live output\n" {
		t.Fatalf("live logs = %q", model.Logs[target.ID])
	}
	if next == nil {
		t.Fatal("live output should schedule next stream read")
	}

	for next != nil {
		updated, next = model.Update(next())
		model = updated.(Model)
	}
	if model.Logs[target.ID] != "live output\n" {
		t.Fatalf("final logs duplicated streamed output: %q", model.Logs[target.ID])
	}
}

func TestRunCompletionPreservesStreamedOutputAndDeduplicatesError(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", Selected: true}
	model := NewModel(Options{Targets: []core.Target{target}})
	model.Running = true
	model.Status[target.ID] = core.StatusRunning
	model.Logs[target.ID] = "live output\nexit status 1\n"

	updated, _ := model.Update(runEventMsg{event: targetEvent(
		runpkg.EventTargetFinished,
		target,
		core.StatusFailed,
		"live output\nexit status 1\n",
		"exit status 1",
	)})
	model = updated.(Model)

	if model.Logs[target.ID] != "live output\nexit status 1\n" {
		t.Fatalf("final logs = %q", model.Logs[target.ID])
	}
}

func TestCommandEnterFocusesTasksWhenRunStarts(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
	}})
	model.openCommandOverlay()

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)

	if cmd == nil {
		t.Fatal("enter should start a run")
	}
	if updated.Focus != FocusTargets {
		t.Fatalf("focus = %v, want tasks after starting run", updated.Focus)
	}
}

func TestCommandEnterFocusesTasksWhenNoTargetIsSelected(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: false},
	}})
	model.openCommandOverlay()

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)

	if cmd != nil {
		t.Fatal("enter should not start a run without selected targets")
	}
	if updated.Focus != FocusTargets {
		t.Fatalf("focus = %v, want tasks when no target is selected", updated.Focus)
	}
	if updated.RunError != "no selected targets; press a to toggle visible" {
		t.Fatalf("run error = %q", updated.RunError)
	}
}

func TestModelStartErrorLeavesRunUnaccepted(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
	}, startRun: func(context.Context, runpkg.Spec) (activeRun, error) {
		return nil, errors.New("runner setup failed")
	}})

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)
	model = updated

	if cmd != nil {
		t.Fatal("start error should not schedule command")
	}
	if model.Status["api"] != core.StatusIdle {
		t.Fatalf("api status = %s, want idle", model.Status["api"])
	}
	if model.Running {
		t.Fatal("rejected run should not be running")
	}
	if !strings.Contains(model.RunError, "runner setup failed") {
		t.Fatalf("run error = %q", model.RunError)
	}
	if len(model.RunHistory) != 0 {
		t.Fatalf("run history = %#v, want no unaccepted run", model.RunHistory)
	}
}

func TestModelPassesFailFastAndProjectsFinalOutcomes(t *testing.T) {
	targets := []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}
	run := &fakeActiveRun{}
	var spec runpkg.Spec
	model := NewModel(Options{
		Command:  "echo ok",
		Workers:  1,
		FailFast: true,
		Targets:  targets,
		startRun: fakeStart(run, &spec),
	})

	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	if !spec.FailFast || spec.Workers != 1 {
		t.Fatalf("run spec = %#v", spec)
	}
	event := completedEvent("run-1", "echo ok",
		runpkg.TargetSnapshot{Target: targets[0], Status: core.StatusFailed, Error: "exit status 1"},
		runpkg.TargetSnapshot{Target: targets[1], Status: core.StatusCancelled},
		runpkg.TargetSnapshot{Target: targets[2], Status: core.StatusCancelled},
	)
	updatedModel, _ := model.Update(runEventMsg{event: event})
	model = updatedModel.(Model)
	if model.Status["api"] != core.StatusFailed ||
		model.Status["web"] != core.StatusCancelled ||
		model.Status["worker"] != core.StatusCancelled {
		t.Fatalf("fail-fast statuses = %#v", model.Status)
	}
	if model.Running || model.hasActiveRuns() {
		t.Fatalf("run should be terminal, running/active = %t/%t", model.Running, model.hasActiveRuns())
	}
	if len(model.RunHistory) != 1 {
		t.Fatalf("run history = %#v, want one terminal run", model.RunHistory)
	}
	summary := model.RunHistory[0]
	if summary.Total != 3 || summary.Failed != 1 || summary.Cancelled != 2 || summary.Succeeded != 0 {
		t.Fatalf("run totals = %#v, want one failure and two cancellations", summary)
	}
}

func TestModelUsesRunIDForArchivedHistoryProjection(t *testing.T) {
	targets := []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}
	run := &fakeActiveRun{}
	model := NewModel(Options{
		Command:  "echo ok",
		Workers:  2,
		SaveLogs: true,
		LogRoot:  t.TempDir(),
		Targets:  targets,
		startRun: fakeStart(run, nil),
	})

	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	event := completedEvent("20260830T010203.000000000Z", "echo ok",
		runpkg.TargetSnapshot{Target: targets[0], Status: core.StatusSucceeded},
		runpkg.TargetSnapshot{Target: targets[1], Status: core.StatusSucceeded},
	)
	event.Run.LogID = event.Run.ID
	updatedModel, _ := model.Update(runEventMsg{event: event})
	model = updatedModel.(Model)
	if len(model.RunHistory) != 1 || model.RunHistory[0].LogID != "20260830T010203.000000000Z" {
		t.Fatalf("run history = %#v", model.RunHistory)
	}
}

func TestModelBackspaceRemovesRune(t *testing.T) {
	inputs := []struct {
		name  string
		value string
		want  string
	}{
		{name: "accented", value: "café", want: "caf"},
		{name: "emoji", value: "go 🚀", want: "go "},
	}
	fields := []struct {
		name    string
		prepare func(*Model, string)
		value   func(Model) string
	}{
		{
			name: "command",
			prepare: func(model *Model, value string) {
				model.Focus = FocusCommand
				model.Command = value
			},
			value: func(model Model) string { return model.Command },
		},
		{
			name: "filter",
			prepare: func(model *Model, value string) {
				model.Focus = FocusFilter
				model.Filter = value
			},
			value: func(model Model) string { return model.Filter },
		},
		{
			name: "palette",
			prepare: func(model *Model, value string) {
				model.ShowPalette = true
				model.Palette = value
			},
			value: func(model Model) string { return model.Palette },
		},
		{
			name: "history",
			prepare: func(model *Model, value string) {
				model.ShowHistory = true
				model.HistorySearching = true
				model.HistoryFilter = value
			},
			value: func(model Model) string { return model.HistoryFilter },
		},
	}

	for _, field := range fields {
		for _, input := range inputs {
			t.Run(field.name+"/"+input.name, func(t *testing.T) {
				model := NewModel(Options{})
				field.prepare(&model, input.value)

				model, _ = updateSpecialKey(model, tea.KeyBackspace)
				got := field.value(model)

				if !utf8.ValidString(got) {
					t.Fatalf("backspace produced invalid UTF-8: %q", got)
				}
				if got != input.want {
					t.Fatalf("backspace result = %q, want %q", got, input.want)
				}
			})
		}
	}
}

func TestCompletionNoticeOnlyMentionsRerunWhenFailed(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Status["api"] = core.StatusSucceeded
	model.Status["web"] = core.StatusSucceeded
	if notice := model.completionNotice(); notice != "run complete: 2 ok, 0 failed, 0 cancelled" {
		t.Fatalf("notice = %q", notice)
	}

	model.Status["web"] = core.StatusFailed
	if notice := model.completionNotice(); !strings.Contains(notice, "press R to rerun failed") {
		t.Fatalf("failed notice should mention rerun shortcut: %q", notice)
	}
}

func TestModelRunErrorsGuideNextAction(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusTargets
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.RunError != "command is empty; press : to edit" {
		t.Fatalf("run error = %q", model.RunError)
	}
	if model.Focus != FocusCommand || !model.ShowCommand {
		t.Fatalf("focus/overlay = %v/%t, want command overlay after empty run", model.Focus, model.ShowCommand)
	}
	if rendered := model.renderCommandOverlay(100, 20); !strings.Contains(stripANSI(rendered), "Run command") || model.commandCursor != 0 || strings.Contains(stripANSI(rendered), "<type command") {
		t.Fatalf("empty run should open initialized command overlay:\n%s", stripANSI(rendered))
	}

	model = NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: false}}})
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.RunError != "no selected targets; press a to toggle visible" {
		t.Fatalf("run error = %q", model.RunError)
	}

	model = NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: false}}})
	model.Filter = "api"
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.RunError != "no selected targets; press a to toggle matching" {
		t.Fatalf("run error = %q", model.RunError)
	}

	model = NewModel(Options{Command: "echo ok"})
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.RunError != "no target directories found" {
		t.Fatalf("run error = %q", model.RunError)
	}
}

func TestStatusRailOnlyRendersErrors(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateWindowSize(model, 80, 24)
	model.RunError = "command is empty; press : to edit"
	view := model.View().Content
	plain := stripANSI(view)
	if !strings.Contains(plain, "ERROR command is empty; press : to edit") {
		t.Fatalf("error status missing:\n%s", plain)
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("error bar should be styled")
	}
	if got := maxLineWidth(view); got > 80 {
		t.Fatalf("error view width = %d:\n%s", got, plain)
	}

	model.RunError = ""
	for _, notice := range []string{
		"workers set to 2",
		"selected 2 target(s)",
		"started 2 target(s)",
		"run complete: 0 ok, 1 failed, 0 cancelled · press R to rerun failed",
		"run complete: 0 ok, 0 failed, 1 cancelled",
	} {
		model.Notice = notice
		plain = stripANSI(model.View().Content)
		if strings.Contains(plain, notice) || strings.Contains(plain, "INFO ") || strings.Contains(plain, "WARN ") {
			t.Fatalf("action notice should stay hidden for %q:\n%s", notice, plain)
		}
	}
}

func TestModelCancelsOnlySelectedRunningTarget(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: false},
	}})
	run := &fakeActiveRun{cancelResults: [][]string{{"api"}}}
	model.Running = true
	model.activeRun = run
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusRunning

	model, _ = updateKey(model, "delete")
	if run.cancelCalls != 1 {
		t.Fatal("selected api target should be cancelled")
	}
	if model.Status["api"] != core.StatusCancelled || model.Status["web"] != core.StatusRunning {
		t.Fatalf("statuses = %#v", model.Status)
	}

	updated, _ := model.Update(runEventMsg{event: completedEvent("run-1", "sleep 10",
		runpkg.TargetSnapshot{Target: model.Targets[0], Status: core.StatusCancelled},
		runpkg.TargetSnapshot{Target: model.Targets[1], Status: core.StatusSucceeded},
	)})
	model = updated.(Model)
	if model.Running {
		t.Fatal("model should stop after remaining target completes")
	}
	if model.Status["web"] != core.StatusSucceeded {
		t.Fatalf("web status = %s", model.Status["web"])
	}
}

func TestModelPassesModeAndWorkerLimitToRun(t *testing.T) {
	run := &fakeActiveRun{}
	var spec runpkg.Spec
	model := NewModel(Options{Command: "echo ok", Mode: core.ModeSerial, Workers: 7, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}, startRun: fakeStart(run, &spec)})

	_, _ = updateSpecialKey(model, tea.KeyEnter)
	if spec.Mode != core.ModeSerial || spec.Workers != 7 {
		t.Fatalf("run spec = %#v", spec)
	}
}

func TestModelCancelsSelectedQueuedTargets(t *testing.T) {
	run := &fakeActiveRun{cancelResults: [][]string{{"web", "worker"}}}
	model := NewModel(Options{Command: "echo ok", Workers: 1, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}, startRun: fakeStart(run, nil)})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	model.Status["api"] = core.StatusRunning
	model.Targets[0].Selected = false

	model, _ = updateKey(model, "delete")
	if model.Status["web"] != core.StatusQueued || model.Status["worker"] != core.StatusQueued {
		t.Fatalf("multi-target cancellation should wait for confirmation: %#v", model.Status)
	}
	model, _ = updateKey(model, "y")
	if model.Status["api"] != core.StatusRunning {
		t.Fatalf("active unselected target should keep running: %#v", model.Status)
	}
	if model.Status["web"] != core.StatusCancelled || model.Status["worker"] != core.StatusCancelled {
		t.Fatalf("queued selected targets should be cancelled: %#v", model.Status)
	}
	updatedModel, next := model.Update(runEventMsg{event: completedEvent("run-1", "echo ok",
		runpkg.TargetSnapshot{Target: model.Targets[0], Status: core.StatusSucceeded},
		runpkg.TargetSnapshot{Target: model.Targets[1], Status: core.StatusCancelled},
		runpkg.TargetSnapshot{Target: model.Targets[2], Status: core.StatusCancelled},
	)})
	model = updatedModel.(Model)
	if next != nil {
		t.Fatal("no follow-up command expected after queue was cancelled")
	}
	if model.Running {
		t.Fatal("model should stop after active target completes")
	}
	if len(model.RunHistory) != 1 || model.RunHistory[0].Cancelled != 2 {
		t.Fatalf("run history = %#v", model.RunHistory)
	}
}

func TestCancelAllClearsQueuedWork(t *testing.T) {
	run := &fakeActiveRun{cancelResults: [][]string{{"api", "web", "worker"}}}
	model := NewModel(Options{Command: "echo ok", Workers: 1, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}, startRun: fakeStart(run, nil)})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated

	model.cancelAll()
	if run.cancelCalls != 1 {
		t.Fatal("cancelAll should call root cancel function")
	}
	for _, id := range []string{"api", "web", "worker"} {
		if model.Status[id] != core.StatusCancelled {
			t.Fatalf("%s status = %s, want cancelled", id, model.Status[id])
		}
	}

	updatedModel, next := model.Update(runEventMsg{event: completedEvent("run-1", "echo ok",
		runpkg.TargetSnapshot{Target: model.Targets[0], Status: core.StatusCancelled},
		runpkg.TargetSnapshot{Target: model.Targets[1], Status: core.StatusCancelled},
		runpkg.TargetSnapshot{Target: model.Targets[2], Status: core.StatusCancelled},
	)})
	model = updatedModel.(Model)
	if next != nil {
		t.Fatal("cancelAll should not schedule queued work after active target completes")
	}
	if model.Running {
		t.Fatal("model should stop after active cancellation completes")
	}
	if len(model.RunHistory) != 1 || model.RunHistory[0].Cancelled != 3 {
		t.Fatalf("run history = %#v", model.RunHistory)
	}
}

func TestCtrlCConfirmsBeforeCancellingActiveWorkAndQuitting(t *testing.T) {
	run := &fakeActiveRun{cancelResults: [][]string{{"api", "web"}}}
	model := NewModel(Options{Command: "echo ok", Workers: 1, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}, startRun: fakeStart(run, nil)})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated

	updated, cmd := updateKey(model, "ctrl+c")
	model = updated

	if cmd != nil {
		t.Fatal("ctrl+c should wait for confirmation before quitting")
	}
	if !model.ConfirmQuit {
		t.Fatal("ctrl+c should show quit confirmation")
	}
	if !model.ConfirmQuitYes {
		t.Fatal("quit confirmation should default to Yes")
	}
	if run.cancelCalls != 0 {
		t.Fatal("ctrl+c should not cancel the root run context before confirmation")
	}
	if model.Status["api"] == core.StatusCancelled || model.Status["web"] == core.StatusCancelled {
		t.Fatalf("statuses = %#v, want active work unchanged before confirmation", model.Status)
	}

	updated, cmd = updateKey(model, "enter")
	model = updated
	if run.cancelCalls != 1 {
		t.Fatal("enter on Yes should cancel the root run context")
	}
	if model.Status["api"] != core.StatusCancelled || model.Status["web"] != core.StatusCancelled {
		t.Fatalf("statuses = %#v, want all active work cancelled", model.Status)
	}
	if cmd == nil {
		t.Fatal("ctrl+c should return tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
	}
}

func TestModelHistoryAndRerunFailed(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.Status["api"] = core.StatusSucceeded
	model.Status["web"] = core.StatusFailed
	model.History = []string{"go test", "pnpm test"}

	model, _ = updateKey(model, "H")
	if !model.ShowHistory {
		t.Fatal("history should open")
	}
	model, _ = updateKey(model, "]")
	model, _ = updateSpecialKey(model, tea.KeyDown)
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.Command != "pnpm test" {
		t.Fatalf("command = %q, want history command", model.Command)
	}

	model, _ = updateSpecialKey(model, tea.KeyEsc)
	model, _ = updateKey(model, "R")
	if !model.ConfirmRun {
		t.Fatal("R should open rerun confirmation")
	}
	confirmView := stripANSI(model.View().Content)
	for _, want := range []string{"Rerun failed", "1 failed target(s)", "command: pnpm test", "targets: web", "y/enter confirm", "n/esc cancel"} {
		if !strings.Contains(confirmView, want) {
			t.Fatalf("rerun confirmation should contain %q:\n%s", want, confirmView)
		}
	}
	model, _ = updateKey(model, "n")
	if model.ConfirmRun {
		t.Fatal("n should close rerun confirmation")
	}
	if model.Notice != "confirmation cancelled" {
		t.Fatalf("notice = %q", model.Notice)
	}
	model, _ = updateKey(model, "R")
	run := &fakeActiveRun{}
	var spec runpkg.Spec
	model.startLifecycle = fakeStart(run, &spec)
	updated, cmd := updateKey(model, "y")
	model = updated
	if cmd == nil {
		t.Fatal("confirm should start rerun")
	}
	if len(spec.Targets) != 1 || spec.Targets[0].ID != "web" {
		t.Fatalf("rerun targets = %#v", spec.Targets)
	}
	updatedModel, _ := model.Update(runEventMsg{event: completedEvent("run-1", spec.Command,
		runpkg.TargetSnapshot{Target: spec.Targets[0], Status: core.StatusSucceeded, OutputTail: "fixed\n"},
	)})
	model = updatedModel.(Model)
	if model.Status["web"] != core.StatusSucceeded {
		t.Fatalf("web status = %s", model.Status["web"])
	}
}

func TestModelHistoryOverlayShowsProjectRuns(t *testing.T) {
	tmp := t.TempDir()
	runHistory := filepath.Join(tmp, "project-history.jsonl")
	if err := history.AppendRun(runHistory, history.RunEntry{Command: "go test", Total: 2, Succeeded: 1, Failed: 1}); err != nil {
		t.Fatal(err)
	}
	model := NewModel(Options{
		Command:        "echo ok",
		RunHistoryPath: runHistory,
		Targets:        []core.Target{{ID: "api", RelPath: "api", Selected: true}},
	})
	model, _ = updateKey(model, "H")
	view := stripANSI(model.View().Content)
	for _, want := range []string{"Project runs 1", "WHEN", "go test"} {
		if !strings.Contains(view, want) {
			t.Fatalf("history overlay should contain %q:\n%s", want, view)
		}
	}
}

func TestHistoryOverlayCanReuseProjectRunCommand(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./..."}
	model.RunHistory = []history.RunEntry{{Command: "pnpm test", Total: 3, Succeeded: 2, Failed: 1}}

	model, _ = updateKey(model, "H")
	if model.HistoryPos != 0 {
		t.Fatalf("history pos = %d, want first project run", model.HistoryPos)
	}
	view := model.renderHistoryOverlay(120, 24)
	if !strings.Contains(stripANSI(view), "›") || !strings.Contains(view, "\x1b[") {
		t.Fatalf("selected project run should be highlighted:\n%s", view)
	}
	model, _ = updateKey(model, "r")
	if model.Command != "pnpm test" {
		t.Fatalf("command = %q, want selected project run command", model.Command)
	}
	if model.Focus != FocusCommand {
		t.Fatalf("focus = %v, want command", model.Focus)
	}
}

func TestHistoryOverlayIgnoresQAndClosesOnEscape(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./..."}
	model, _ = updateKey(model, "H")
	model, _ = updateKey(model, "q")
	if !model.ShowHistory {
		t.Fatal("q should not close history overlay")
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowHistory {
		t.Fatal("escape should close history overlay")
	}
}

func TestHistoryOverlaySupportsFuzzySearch(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./...", "pnpm lint", "terraform plan"}
	model.RunHistory = []history.RunEntry{
		{Command: "pnpm test", Total: 3, Succeeded: 2, Failed: 1},
		{Command: "docker build", Total: 1, Succeeded: 1},
	}

	model, _ = updateKey(model, "H")
	model, _ = updateKey(model, "/")
	if !model.HistorySearching {
		t.Fatal("/ should enable history search")
	}
	model = typeText(model, "pt")
	view := model.renderHistoryOverlay(120, 24)
	plain := stripANSI(view)
	for _, want := range []string{"/ pt", "Commands 1/3", "Project runs 1/2", "pnpm test"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("history search should contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "terraform plan") || strings.Contains(plain, "docker build") || strings.Contains(plain, "pnpm lint") {
		t.Fatalf("history search should hide non-matches:\n%s", plain)
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("history search matches should be styled:\n%s", view)
	}

	model, _ = updateKey(model, "ctrl+u")
	if model.HistoryFilter != "" || model.HistoryPos != 0 {
		t.Fatalf("ctrl+u should clear history search, filter/pos = %q/%d", model.HistoryFilter, model.HistoryPos)
	}
	model.HistoryFilter = "'pt"
	if commands := model.filteredHistoryCommands(); len(commands) != 0 {
		t.Fatalf("exact history search should not fuzzy match commands: %#v", commands)
	}
	if runs := model.filteredHistoryRuns(); len(runs) != 0 {
		t.Fatalf("exact history search should not fuzzy match runs: %#v", runs)
	}
}

func TestHistoryEnterNoMatchKeepsOverlayOpen(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./..."}
	model.RunHistory = []history.RunEntry{{Command: "pnpm test", Total: 2, Succeeded: 2}}

	model, _ = updateKey(model, "H")
	model, _ = updateKey(model, "/")
	model = typeText(model, "zzzz")
	model, _ = updateSpecialKey(model, tea.KeyEnter)

	if !model.ShowHistory || !model.HistorySearching {
		t.Fatal("no-match enter should keep history search open")
	}
	if model.RunError != "no project run matches" {
		t.Fatalf("run error = %q", model.RunError)
	}
	view := stripANSI(model.View().Content)
	for _, want := range []string{"History", "Commands 0", "No project runs match.", "ERROR no project run matches"} {
		if !strings.Contains(view, want) {
			t.Fatalf("history no-match view should contain %q:\n%s", want, view)
		}
	}

	model, _ = updateSpecialKey(model, tea.KeyBackspace)
	if model.RunError != "" {
		t.Fatalf("editing history search should clear no-match error, got %q", model.RunError)
	}
}

func TestHistoryTabsAreScannableAndHighlightSelection(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./...", "pnpm test"}
	model.HistoryCommandPos = 1
	model.RunHistory = []history.RunEntry{
		{Command: "go test ./...", Total: 3, Succeeded: 2, Failed: 1},
		{Command: "pnpm test", Total: 2, Succeeded: 2},
		{Command: "sleep 10", Total: 1, Cancelled: 1},
	}
	model.ShowHistory = true
	runs := model.renderHistoryOverlay(120, 24)
	plain := stripANSI(runs)
	for _, want := range []string{"Commands 2", "Project runs 3", "RESULT", "failed", "ok", "cancelled"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("run tab should contain %q:\n%s", want, plain)
		}
	}
	model.HistoryTab = historyTabCommands
	commands := model.renderHistoryOverlay(120, 24)
	for _, want := range []string{"#   COMMAND", "› 2", "pnpm test"} {
		if !strings.Contains(stripANSI(commands), want) {
			t.Fatalf("command tab should contain %q:\n%s", want, stripANSI(commands))
		}
	}
	if !strings.Contains(runs, "\x1b[") || !strings.Contains(commands, "\x1b[") {
		t.Fatal("selected rows and outcomes should be styled")
	}
}

func TestHistoryUsesProjectRunsTabAndRestoresTabCursors(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./...", "pnpm test"}
	model.RunHistory = []history.RunEntry{
		{Command: "go test ./...", Total: 1, Succeeded: 1},
		{Command: "pnpm test", Total: 1, Failed: 1},
	}

	model, _ = updateKey(model, "H")
	if model.HistoryTab != historyTabRuns || model.HistoryDepth != historyDepthRuns {
		t.Fatalf("history state = tab %v depth %v", model.HistoryTab, model.HistoryDepth)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.HistoryPos != 1 {
		t.Fatalf("run cursor = %d, want 1", model.HistoryPos)
	}
	model, _ = updateKey(model, "]")
	if model.HistoryTab != historyTabCommands || model.HistoryCommandPos != 0 {
		t.Fatalf("commands state = tab %v cursor %d", model.HistoryTab, model.HistoryCommandPos)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	model, _ = updateKey(model, "[")
	if model.HistoryTab != historyTabRuns || model.HistoryPos != 1 || model.HistoryCommandPos != 1 {
		t.Fatalf("restored state = tab %v run %d command %d", model.HistoryTab, model.HistoryPos, model.HistoryCommandPos)
	}
}

func TestHistoryRunDiagnosticDefaultsToFailedAndCancelledTargets(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.RunHistory = []history.RunEntry{{
		Command: "go test ./...",
		Total:   3,
		Targets: []history.TargetEntry{
			{ID: "api", RelPath: "api", Status: core.StatusSucceeded},
			{ID: "web", RelPath: "services/web", Status: core.StatusFailed, Error: "exit status 1"},
			{ID: "docs", RelPath: "docs", Status: core.StatusCancelled},
		},
	}}

	model, _ = updateKey(model, "H")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.HistoryDepth != historyDepthTargets {
		t.Fatalf("history depth = %v, want targets", model.HistoryDepth)
	}
	visible := model.visibleHistoryTargets()
	if len(visible) != 2 || visible[0].ID != "web" || visible[1].ID != "docs" {
		t.Fatalf("visible targets = %#v", visible)
	}
	model, _ = updateKey(model, "a")
	if visible := model.visibleHistoryTargets(); len(visible) != 3 {
		t.Fatalf("all targets = %#v", visible)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.HistoryDepth != historyDepthRuns || !model.ShowHistory {
		t.Fatalf("back state = depth %v show %v", model.HistoryDepth, model.ShowHistory)
	}
}

func TestHistoryResponsiveMasterDetailAndDrillDown(t *testing.T) {
	base := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	base.RunHistory = []history.RunEntry{{
		Command: "go test ./...",
		Total:   2,
		Failed:  1,
		Targets: []history.TargetEntry{{ID: "web", RelPath: "services/web", Status: core.StatusFailed, Error: "exit status 1"}},
	}}
	base.ShowHistory = true

	wide, _ := updateWindowSize(base, 140, 40)
	wideView := stripANSI(wide.View().Content)
	if got := strings.Count(wide.View().Content, "\n") + 1; got != 40 {
		t.Fatalf("wide history height = %d, want 40:\n%s", got, wideView)
	}
	for _, want := range []string{"Project runs 1", "Commands 0", "TARGETS", "Diagnostic", "services/web"} {
		if !strings.Contains(wideView, want) {
			t.Fatalf("wide history missing %q:\n%s", want, wideView)
		}
	}

	standard, _ := updateWindowSize(base, 100, 30)
	standardView := stripANSI(standard.View().Content)
	if strings.Contains(standardView, "services/web") {
		t.Fatalf("standard list should hide target detail before drill-down:\n%s", standardView)
	}
	standard, _ = updateSpecialKey(standard, tea.KeyEnter)
	if view := stripANSI(standard.View().Content); !strings.Contains(view, "Failed and cancelled targets") || !strings.Contains(view, "services/web") {
		t.Fatalf("standard diagnostic missing target detail:\n%s", view)
	}

	minimum, _ := updateWindowSize(base, 80, 20)
	minimumView := stripANSI(minimum.View().Content)
	if got := maxLineWidth(minimumView); got > 80 {
		t.Fatalf("minimum history width = %d:\n%s", got, minimumView)
	}
	if got := strings.Count(minimumView, "\n") + 1; got > 20 {
		t.Fatalf("minimum history height = %d:\n%s", got, minimumView)
	}
	minimum, _ = updateSpecialKey(minimum, tea.KeyEnter)
	minimumDiagnostic := stripANSI(minimum.View().Content)
	for _, want := range []string{"Failed and cancelled targets", "services/web", "exit status 1"} {
		if !strings.Contains(minimumDiagnostic, want) {
			t.Fatalf("minimum diagnostic missing %q:\n%s", want, minimumDiagnostic)
		}
	}
	minimum, _ = updateSpecialKey(minimum, tea.KeyPgUp)
	minimumMetadata := stripANSI(minimum.View().Content)
	for _, want := range []string{"command  go test ./...", "1 failed", "logs     unavailable"} {
		if !strings.Contains(minimumMetadata, want) {
			t.Fatalf("minimum metadata page missing %q:\n%s", want, minimumMetadata)
		}
	}
}

func TestHistoryDiagnosticOverlayGolden(t *testing.T) {
	started := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.Local)
	ended := started.Add(1250 * time.Millisecond)
	model := NewModel(Options{Command: "echo current", Targets: []core.Target{{ID: "web", RelPath: "services/web", Selected: true}}})
	model.ShowHistory = true
	model.History = []string{"go test ./...", "pnpm test"}
	model.RunHistory = []history.RunEntry{{
		Started:   started,
		Ended:     ended,
		Command:   "pnpm test --filter web",
		LogID:     "run-1",
		Total:     2,
		Succeeded: 1,
		Failed:    1,
		Targets: []history.TargetEntry{
			{ID: "api", RelPath: "services/api", Status: core.StatusSucceeded, ExitCode: 0, Started: started, Ended: ended},
			{ID: "web", RelPath: "services/web", Status: core.StatusFailed, ExitCode: 1, Error: "exit status 1", Started: started, Ended: ended},
		},
	}}

	wide := normalizeHistoryGolden(model.renderHistoryOverlay(120, 16))
	model.HistoryDepth = historyDepthTargets
	compact := normalizeHistoryGolden(model.renderHistoryOverlay(80, 11))
	got := "wide:\n" + wide + "\n\ncompact:\n" + compact
	want, err := os.ReadFile("testdata/TestHistoryDiagnosticOverlayGolden.golden")
	if err != nil {
		t.Fatal(err)
	}
	wantText := strings.TrimSpace(string(want))
	gotText := strings.TrimSpace(got)
	if gotText != wantText {
		index := 0
		for index < len(gotText) && index < len(wantText) && gotText[index] == wantText[index] {
			index++
		}
		start := max(0, index-24)
		endGot := min(len(gotText), index+48)
		endWant := min(len(wantText), index+48)
		t.Fatalf("golden mismatch at byte %d (got %d bytes, want %d): got %q want %q\n--- got ---\n%s\n--- want ---\n%s", index, len(gotText), len(wantText), gotText[start:endGot], wantText[start:endWant], got, want)
	}
}

func TestHistoryOverlayKeepsDefaultBackgroundOutsideHighlights(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	rendered := renderHistoryRow("command  echo ok", 72)
	if byteIndex := strings.Index(rendered, "\x1b[48;"); byteIndex >= 0 {
		t.Fatalf("ordinary history row paints dark background at byte %d: %q", byteIndex, rendered)
	}
}

func TestHistoryDiagnosticWithoutColor(t *testing.T) {
	if os.Getenv("RUNNY_NO_COLOR_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestHistoryDiagnosticWithoutColor$")
		cmd.Env = append(os.Environ(), "RUNNY_NO_COLOR_TEST=1", "NO_COLOR=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("no-color subprocess: %v\n%s", err, output)
		}
		return
	}

	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "web", RelPath: "web", Selected: true}}})
	model.ShowHistory = true
	model.RunHistory = []history.RunEntry{{
		Command: "pnpm test", Total: 1, Failed: 1,
		Targets: []history.TargetEntry{{ID: "web", RelPath: "web", Status: core.StatusFailed, ExitCode: 1}},
	}}
	rendered := model.renderHistoryOverlay(120, 16)
	if strings.Contains(rendered, "\x1b[38;") || strings.Contains(rendered, "\x1b[48;") {
		t.Fatalf("no-color history contains color SGR: %q", rendered)
	}
	for _, want := range []string{"›", "1 failed", "web"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("no-color history missing %q:\n%s", want, rendered)
		}
	}
}

func normalizeHistoryGolden(rendered string) string {
	lines := []string{}
	for _, line := range strings.Split(stripANSI(rendered), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "╭") || strings.HasPrefix(line, "╰") {
			continue
		}
		line = strings.Trim(line, "│ ")
		if line == "" {
			continue
		}
		lines = append(lines, strings.Join(strings.Fields(line), " "))
	}
	return strings.Join(lines, "\n")
}

func TestHistoryRerunFailedConfirmsHistoricalCommandAndTargets(t *testing.T) {
	model := NewModel(Options{Command: "echo current", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "services/web", Selected: true},
	}})
	model.RunHistory = []history.RunEntry{{
		Command: "pnpm test",
		Total:   2,
		Failed:  1,
		Targets: []history.TargetEntry{
			{ID: "api", RelPath: "api", Status: core.StatusSucceeded},
			{ID: "web", RelPath: "services/web", Status: core.StatusFailed},
		},
	}}

	model, _ = updateKey(model, "H")
	model, _ = updateKey(model, "R")
	if model.ShowHistory || !model.ConfirmRun {
		t.Fatalf("history/confirm = %v/%v", model.ShowHistory, model.ConfirmRun)
	}
	if model.Command != "echo current" {
		t.Fatalf("command changed before confirmation = %q", model.Command)
	}
	confirm := stripANSI(model.View().Content)
	for _, want := range []string{"Rerun failed", "1 failed target(s)", "command: pnpm test", "targets: services/web"} {
		if !strings.Contains(confirm, want) {
			t.Fatalf("historical confirmation missing %q:\n%s", want, confirm)
		}
	}

	run := &fakeActiveRun{}
	var request runpkg.Spec
	model.startLifecycle = fakeStart(run, &request)
	model, cmd := updateKey(model, "y")
	if cmd == nil {
		t.Fatal("historical confirmation should start run")
	}
	if model.Command != "pnpm test" {
		t.Fatalf("confirmed command = %q", model.Command)
	}
	if request.Command != "pnpm test" || len(request.Targets) != 1 || request.Targets[0].ID != "web" {
		t.Fatalf("historical request = %#v", request)
	}
}

func TestHistoryLoadsPersistedTargetLogAsynchronously(t *testing.T) {
	root := t.TempDir()
	runID := "20260815T080000.000000000Z"
	logPath := filepath.Join(root, runID, "services", "web.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("line one\nline two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	model := NewModel(Options{Command: "echo ok", LogRoot: root, Targets: []core.Target{{ID: "web", RelPath: "services/web", Selected: true}}})
	model.RunHistory = []history.RunEntry{{
		Command: "pnpm test",
		Total:   1,
		Failed:  1,
		LogID:   runID,
		Targets: []history.TargetEntry{{ID: "services/web", RelPath: "services/web", Status: core.StatusFailed}},
	}}

	model, _ = updateKey(model, "H")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	model, cmd := updateSpecialKey(model, tea.KeyEnter)
	if model.HistoryDepth != historyDepthLogs || !model.HistoryLogLoading || cmd == nil {
		t.Fatalf("log loading state = depth %v loading %v cmd %v", model.HistoryDepth, model.HistoryLogLoading, cmd != nil)
	}
	updated, _ := model.Update(cmd())
	model = updated.(Model)
	if model.HistoryLogLoading || model.HistoryLogError != "" || model.HistoryLog != "line one\nline two\n" {
		t.Fatalf("loaded log = loading %v error %q content %q", model.HistoryLogLoading, model.HistoryLogError, model.HistoryLog)
	}
	if view := stripANSI(model.renderHistoryOverlay(100, 20)); !strings.Contains(view, "Logs · services/web") || !strings.Contains(view, "line two") {
		t.Fatalf("log viewport missing content:\n%s", view)
	}
	model, _ = updateKey(model, "a")
	if model.HistoryShowAll || model.HistoryDepth != historyDepthLogs || model.HistoryLog != "line one\nline two\n" {
		t.Fatalf("a should not mutate log selection: showAll=%v depth=%v content=%q", model.HistoryShowAll, model.HistoryDepth, model.HistoryLog)
	}
}

func TestHistoryLogWithoutPersistenceExplainsUnavailableOutput(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "web", RelPath: "web", Selected: true}}})
	model.RunHistory = []history.RunEntry{{
		Command: "pnpm test",
		Total:   1,
		Failed:  1,
		Targets: []history.TargetEntry{{ID: "web", RelPath: "web", Status: core.StatusFailed}},
	}}

	model, _ = updateKey(model, "H")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	model, cmd := updateSpecialKey(model, tea.KeyEnter)
	if cmd != nil || model.HistoryDepth != historyDepthLogs || model.HistoryLogError == "" {
		t.Fatalf("unavailable state = depth %v error %q cmd %v", model.HistoryDepth, model.HistoryLogError, cmd != nil)
	}
	if view := stripANSI(model.renderHistoryOverlay(100, 20)); !strings.Contains(view, "--save-logs") {
		t.Fatalf("unavailable viewport should explain persistence:\n%s", view)
	}
}

func TestHistoryLogEmptyReadErrorAndBackStack(t *testing.T) {
	root := t.TempDir()
	runID := "run"
	logPath := filepath.Join(root, runID, "web.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	model := NewModel(Options{Command: "echo ok", LogRoot: root, Targets: []core.Target{{ID: "web", RelPath: "web", Selected: true}}})
	model.RunHistory = []history.RunEntry{{
		Command: "pnpm test", Total: 1, Failed: 1, LogID: runID,
		Targets: []history.TargetEntry{{ID: "web", RelPath: "web", Status: core.StatusFailed}},
	}}
	model, _ = updateKey(model, "H")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	model, cmd := updateSpecialKey(model, tea.KeyEnter)
	updated, _ := model.Update(cmd())
	model = updated.(Model)
	if view := stripANSI(model.renderHistoryOverlay(100, 20)); !strings.Contains(view, "(empty log)") {
		t.Fatalf("empty log state missing:\n%s", view)
	}

	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	model.HistoryDepth = historyDepthTargets
	model, cmd = updateSpecialKey(model, tea.KeyEnter)
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if model.HistoryLogError == "" || !strings.Contains(model.HistoryLogError, "logs unavailable") {
		t.Fatalf("read error = %q", model.HistoryLogError)
	}

	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.HistoryDepth != historyDepthTargets || !model.ShowHistory {
		t.Fatalf("log back state = %v/%v", model.HistoryDepth, model.ShowHistory)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.HistoryDepth != historyDepthRuns || !model.ShowHistory {
		t.Fatalf("target back state = %v/%v", model.HistoryDepth, model.ShowHistory)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowHistory {
		t.Fatal("run back should close history")
	}
}

func TestHistoryLegacyRunExplainsMissingTargetDetails(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "web", RelPath: "web", Selected: true}}})
	model.RunHistory = []history.RunEntry{{Command: "legacy command", Total: 1, Failed: 1}}
	model, _ = updateWindowSize(model, 100, 30)
	model, _ = updateKey(model, "H")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if view := stripANSI(model.View().Content); !strings.Contains(view, "target details unavailable for legacy run") {
		t.Fatalf("legacy explanation missing:\n%s", view)
	}
}

func TestHistoryLogOffsetClampsAtEndAndAfterResize(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "web", RelPath: "web", Selected: true}}})
	model.ShowHistory = true
	model.HistoryDepth = historyDepthLogs
	model.HistoryLog = strings.Repeat("line\n", 20)
	model, _ = updateWindowSize(model, 80, 20)
	for range 100 {
		model.moveHistorySelection(1)
	}
	if model.historyLogViewport.YOffset() != model.maxHistoryLogOffset() {
		t.Fatalf("log offset = %d, max = %d", model.historyLogViewport.YOffset(), model.maxHistoryLogOffset())
	}
	before := model.historyLogViewport.YOffset()
	model.moveHistorySelection(-1)
	if model.historyLogViewport.YOffset() != before-1 {
		t.Fatalf("up from end offset = %d, want %d", model.historyLogViewport.YOffset(), before-1)
	}
	model, _ = updateWindowSize(model, 140, 40)
	if model.historyLogViewport.YOffset() != 0 {
		t.Fatalf("resized log offset = %d, want 0", model.historyLogViewport.YOffset())
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

func updateNamedKey(model Model, key string) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Text: key}))
	return updated.(Model), cmd
}

func typeText(model Model, text string) Model {
	for _, char := range text {
		model, _ = updateKey(model, string(char))
	}
	return model
}

func runPaletteCommand(model Model, command string) (Model, tea.Cmd) {
	model, _ = updateCtrlKey(model, 'p')
	model = typeText(model, command)
	return updateSpecialKey(model, tea.KeyEnter)
}

func updateCtrlKey(model Model, key rune) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: key, Mod: tea.ModCtrl}))
	return updated.(Model), cmd
}

func updateSpecialKey(model Model, key rune) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: key}))
	return updated.(Model), cmd
}

func updateWindowSize(model Model, width int, height int) (Model, tea.Cmd) {
	updated, cmd := model.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return updated.(Model), cmd
}

func applyCmd(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			model = applyCmd(t, model, child)
		}
		return model
	}
	updated, _ := model.Update(msg)
	return updated.(Model)
}

func applyOneCmd(t *testing.T, model Model, cmd tea.Cmd) (Model, tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return model, nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var next tea.Cmd
		for _, child := range batch {
			model, next = applyOneCmd(t, model, child)
		}
		return model, next
	}
	updated, next := model.Update(msg)
	return updated.(Model), next
}
