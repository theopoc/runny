package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
)

func TestPaneResizeDragUpdatesSplitLiveAndPreservesState(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Folded: true},
		{ID: "web", RelPath: "web"},
	}})
	model.Width = 120
	model.Height = 26
	model.Focus = FocusLogs
	model.Cursor = 1
	model.Notice = "keep me"
	model.Running = true
	model.LogFollow = false
	model.Logs["web"] = strings.Repeat("line\n", 40)
	model.syncOutputViewport()
	model.outputViewport.SetYOffset(3)

	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	_, leftWidth, _ := model.panelDimensions(model.Width, model.Height)
	updated, _ := model.Update(tea.MouseClickMsg{
		X: leftWidth + 1, Y: panelTop + 4, Button: tea.MouseLeft,
	})
	model = updated.(Model)
	if !model.paneResizeActive {
		t.Fatal("divider click should start pane resize")
	}

	updated, _ = model.Update(tea.MouseMotionMsg{
		X: leftWidth + 11, Y: panelTop + 4, Button: tea.MouseLeft,
	})
	model = updated.(Model)
	_, gotLeft, gotRight := model.panelDimensions(model.Width, model.Height)
	if gotLeft != 60 || gotRight != 56 {
		t.Fatalf("live split = %d/%d, want 60/56", gotLeft, gotRight)
	}
	if model.Focus != FocusLogs || model.Cursor != 1 || model.Notice != "keep me" || !model.Running || model.LogFollow || model.outputViewport.YOffset() != 3 {
		t.Fatalf("resize changed state: focus=%v cursor=%d notice=%q running=%t follow=%t offset=%d", model.Focus, model.Cursor, model.Notice, model.Running, model.LogFollow, model.outputViewport.YOffset())
	}
	if !model.Targets[0].Selected || !model.Targets[0].Folded || model.Targets[1].Selected {
		t.Fatalf("resize changed task state: %#v", model.Targets)
	}

	updated, _ = model.Update(tea.MouseReleaseMsg{
		X: leftWidth + 11, Y: panelTop + 4, Button: tea.MouseLeft,
	})
	model = updated.(Model)
	if model.paneResizeActive {
		t.Fatal("left release should finish pane resize")
	}
}

func TestPaneResizeClickWithoutMotionKeepsSplit(t *testing.T) {
	model := NewModel(Options{})
	model.Width = 120
	model.Height = 26
	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	_, leftBefore, rightBefore := model.panelDimensions(model.Width, model.Height)

	updated, _ := model.Update(tea.MouseClickMsg{X: leftBefore + 1, Y: panelTop, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: leftBefore + 1, Y: panelTop, Button: tea.MouseLeft})
	model = updated.(Model)
	_, leftAfter, rightAfter := model.panelDimensions(model.Width, model.Height)
	if leftAfter != leftBefore || rightAfter != rightBefore {
		t.Fatalf("click-only split = %d/%d, want %d/%d", leftAfter, rightAfter, leftBefore, rightBefore)
	}
	model, _ = updateWindowSize(model, 110, 26)
	_, leftAfter, rightAfter = model.panelDimensions(model.Width, model.Height)
	_, wantLeft, wantRight := panelDimensionsForInput(110, 26, 1)
	if leftAfter != wantLeft || rightAfter != wantRight {
		t.Fatalf("click-only changed stored ratio: got %d/%d, want default %d/%d", leftAfter, rightAfter, wantLeft, wantRight)
	}
}

func TestPaneResizeKeepsFilterFocusAndText(t *testing.T) {
	model := NewModel(Options{})
	model.Width = 120
	model.Height = 26
	model.Focus = FocusFilter
	model.Filter = "api"
	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	_, leftWidth, _ := model.panelDimensions(model.Width, model.Height)

	updated, _ := model.Update(tea.MouseClickMsg{X: leftWidth, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: leftWidth + 5, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.Focus != FocusFilter || model.Filter != "api" {
		t.Fatalf("resize changed filter state: focus=%v filter=%q", model.Focus, model.Filter)
	}
}

func TestPaneResizeClampsToPaneMinimums(t *testing.T) {
	model := NewModel(Options{})
	model.Width = 120
	model.Height = 26
	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	_, leftWidth, _ := model.panelDimensions(model.Width, model.Height)

	updated, _ := model.Update(tea.MouseClickMsg{X: leftWidth, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: 0, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	_, gotLeft, gotRight := model.panelDimensions(model.Width, model.Height)
	if gotLeft != 36 || gotRight != 80 {
		t.Fatalf("minimum Tasks split = %d/%d, want 36/80", gotLeft, gotRight)
	}

	updated, _ = model.Update(tea.MouseMotionMsg{X: 119, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	_, gotLeft, gotRight = model.panelDimensions(model.Width, model.Height)
	if gotLeft != 84 || gotRight != 32 {
		t.Fatalf("minimum Output split = %d/%d, want 84/32", gotLeft, gotRight)
	}
}

func TestPaneResizeRatioSurvivesResponsiveClampAndModes(t *testing.T) {
	model := NewModel(Options{})
	model.Width = 120
	model.Height = 26
	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	_, leftWidth, _ := model.panelDimensions(model.Width, model.Height)

	updated, _ := model.Update(tea.MouseClickMsg{X: leftWidth, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: 84, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: 84, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)

	model, _ = updateWindowSize(model, 100, 26)
	_, gotLeft, gotRight := model.panelDimensions(model.Width, model.Height)
	if gotLeft != 64 || gotRight != 32 {
		t.Fatalf("narrow clamp = %d/%d, want 64/32", gotLeft, gotRight)
	}
	model.Zoom = true
	model, _ = updateWindowSize(model, 99, 26)
	model.Zoom = false
	model, _ = updateWindowSize(model, 120, 26)
	_, gotLeft, gotRight = model.panelDimensions(model.Width, model.Height)
	if gotLeft != 84 || gotRight != 32 {
		t.Fatalf("restored desired ratio = %d/%d, want 84/32", gotLeft, gotRight)
	}
}

func TestPaneResizeUnavailableOutsideSplitDashboard(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*Model)
		click tea.MouseClickMsg
	}{
		{name: "compact", setup: func(m *Model) { m.Width = 99 }},
		{name: "zoom", setup: func(m *Model) { m.Zoom = true }},
		{name: "history", setup: func(m *Model) { m.ShowHistory = true }},
		{name: "overlay", setup: func(m *Model) { m.ShowHelp = true }},
		{name: "modified left", click: tea.MouseClickMsg{Button: tea.MouseLeft, Mod: tea.ModShift}},
		{name: "right button", click: tea.MouseClickMsg{Button: tea.MouseRight}},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel(Options{})
			model.Width = 120
			model.Height = 26
			if test.setup != nil {
				test.setup(&model)
			}
			panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
			_, leftWidth, _ := model.panelDimensions(model.Width, model.Height)
			click := test.click
			click.X = leftWidth
			click.Y = panelTop + 2
			if click.Button == 0 {
				click.Button = tea.MouseLeft
			}
			updated, _ := model.Update(click)
			if updated.(Model).paneResizeActive {
				t.Fatal("unavailable divider should not start resize")
			}
		})
	}
}

func TestPaneResizePreservesOutputSelectionAcrossReflow(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Width = 120
	model.Height = 26
	model.Focus = FocusLogs
	model.LogFollow = false
	model.Logs["api"] = strings.Repeat("0123456789abcdefghijklmnopqrstuvwxyz\n", 20)
	model.syncOutputViewport()
	model.outputViewport.SetYOffset(3)
	rect, ok := model.outputContentRect()
	if !ok {
		t.Fatal("Output content rect unavailable")
	}

	updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 5, Y: rect.y + 1, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 5, Y: rect.y + 1, Button: tea.MouseLeft})
	model = updated.(Model)
	wantText := model.outputSelection.text()
	wantTarget := model.outputSelection.targetID
	wantRestoreOffset := model.outputSelection.restoreOffset

	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	_, leftWidth, _ := model.panelDimensions(model.Width, model.Height)
	updated, _ = model.Update(tea.MouseClickMsg{X: leftWidth, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: leftWidth + 10, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)

	if got := model.outputSelection.text(); got != wantText {
		t.Fatalf("resize changed selected text: got %q, want %q", got, wantText)
	}
	if !model.outputSelection.active() || model.outputSelection.targetID != wantTarget || model.outputSelection.restoreOffset != wantRestoreOffset || model.LogFollow {
		t.Fatalf("resize changed selection state: %#v follow=%t", model.outputSelection, model.LogFollow)
	}
}

func TestPaneDividerRendersHandleAndHelp(t *testing.T) {
	model := NewModel(Options{})
	model.Width = 120
	model.Height = 26
	panelHeight, leftWidth, rightWidth := model.panelDimensions(model.Width, model.Height)
	inactive := model.renderPanelArea(model.Width, panelHeight, leftWidth, rightWidth)
	if !strings.Contains(stripANSI(inactive), "↔") {
		t.Fatalf("split should render divider handle:\n%s", stripANSI(inactive))
	}

	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
	updated, _ := model.Update(tea.MouseClickMsg{X: leftWidth, Y: panelTop + 2, Button: tea.MouseLeft})
	model = updated.(Model)
	active := model.renderPanelArea(model.Width, panelHeight, leftWidth, rightWidth)
	inactiveDivider := renderPanelSeparator(panelHeight/2, panelHeight, true, false)
	activeDivider := renderPanelSeparator(panelHeight/2, panelHeight, true, true)
	if activeDivider == inactiveDivider || !strings.Contains(active, activeDivider) {
		t.Fatalf("active divider should use distinct styling: inactive=%q active=%q", inactiveDivider, activeDivider)
	}
	if strings.Contains(activeDivider, "\x1b[48;") || strings.Contains(activeDivider, "\x1b[48:") {
		t.Fatalf("divider should not set a background color: %q", activeDivider)
	}
	model.ShowHelp = true
	hidden := model.renderPanelArea(model.Width, panelHeight, leftWidth, rightWidth)
	if strings.Contains(stripANSI(hidden), "↔") {
		t.Fatalf("overlay should hide unavailable divider:\n%s", stripANSI(hidden))
	}

	help := stripANSI(strings.Join(model.helpRows(), "\n"))
	if !strings.Contains(help, "drag divider") || !strings.Contains(help, "resize panes") {
		t.Fatalf("help should document pane resize:\n%s", help)
	}
}
