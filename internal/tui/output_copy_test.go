package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/theopoc/runny/internal/core"
	runpkg "github.com/theopoc/runny/internal/run"
)

func TestYCopiesOnlyFocusedCurrentOutput(t *testing.T) {
	var copied string
	model := NewModel(Options{
		Targets: []core.Target{{ID: "api", RelPath: "api"}, {ID: "web", RelPath: "web"}},
		clipboardCopy: func(_ context.Context, text string) tea.Msg {
			copied = text
			return clipboardResultMsg{status: clipboardConfirmed}
		},
	})
	model.Logs["api"] = runpkg.TruncatedOutputMarker + "api\noutput\n"
	model.liveLogTruncated["api"] = true
	model.Logs["web"] = "web output"
	model.Focus = FocusTargets

	unchanged, cmd := updateKey(model, "y")
	if cmd != nil || copied != "" || unchanged.copyToast.visible() {
		t.Fatal("y outside Output should remain unhandled")
	}
	model.Focus = FocusLogs
	model.ShowHistory = true
	unchanged, cmd = updateKey(model, "y")
	if cmd != nil || copied != "" || unchanged.copyToast.visible() {
		t.Fatal("y in History should not copy Output")
	}
	model.ShowHistory = false

	model, cmd = updateKey(model, "y")
	if cmd == nil {
		t.Fatal("y in Output should return clipboard command")
	}
	msg := cmd()
	if copied != "api\noutput" {
		t.Fatalf("copied = %q, want current Output without marker", copied)
	}
	updated, timer := model.Update(msg)
	model = updated.(Model)
	if timer == nil || model.copyToast.status != clipboardConfirmed {
		t.Fatalf("toast = %#v, timer nil = %t", model.copyToast, timer == nil)
	}
}

func TestYOnEmptyOutputLeavesClipboardUntouched(t *testing.T) {
	called := false
	model := NewModel(Options{
		Targets: []core.Target{{ID: "api", RelPath: "api"}},
		clipboardCopy: func(context.Context, string) tea.Msg {
			called = true
			return clipboardResultMsg{status: clipboardConfirmed}
		},
	})
	model.Focus = FocusLogs
	model, cmd := updateKey(model, "y")
	if cmd == nil || called || model.copyToast.status != clipboardEmpty {
		t.Fatalf("empty copy = called:%t toast:%#v cmd nil:%t", called, model.copyToast, cmd == nil)
	}
}

func TestMouseDragCopiesSnapshotWhileLiveOutputContinues(t *testing.T) {
	var copied string
	model := NewModel(Options{
		Targets: []core.Target{{ID: "api", RelPath: "api"}},
		clipboardCopy: func(_ context.Context, text string) tea.Msg {
			copied = text
			return clipboardResultMsg{status: clipboardConfirmed}
		},
	})
	model.Width = 120
	model.Height = 26
	model.Focus = FocusLogs
	model.Logs["api"] = "alpha\nbeta\ngamma\n"
	model.LogFollow = true
	rect, ok := model.outputContentRect()
	if !ok {
		t.Fatal("Output content rect unavailable")
	}

	updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 1, Y: rect.y + 1, Button: tea.MouseLeft})
	model = updated.(Model)
	model.applyTargetSnapshot(runpkg.TargetSnapshot{Target: core.Target{ID: "api"}, Status: core.StatusRunning, OutputTail: "alpha\nbeta\ngamma\nlive\n"})
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 1, Y: rect.y + 1, Button: tea.MouseLeft})
	model = updated.(Model)

	if !model.outputSelection.active() || model.outputSelection.pendingLive == 0 {
		t.Fatalf("selection state = %#v", model.outputSelection)
	}
	model, cmd := updateKey(model, "y")
	if cmd == nil {
		t.Fatal("selected y should return clipboard command")
	}
	_ = cmd()
	if copied != "alpha\nbe" {
		t.Fatalf("copied snapshot = %q, want %q", copied, "alpha\nbe")
	}
	if model.outputSelection.active() || !model.LogFollow {
		t.Fatalf("copy should clear selection and restore follow: %#v/%t", model.outputSelection, model.LogFollow)
	}
}

func TestCopyToastGenerationIgnoresStaleDismissal(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	first := model.showCopyToast(clipboardConfirmed)
	firstGeneration := model.copyToast.generation
	if first == nil {
		t.Fatal("first toast timer is nil")
	}
	_ = model.showCopyToast(clipboardSent)
	updated, _ := model.Update(dismissCopyToastMsg{generation: firstGeneration})
	model = updated.(Model)
	if model.copyToast.status != clipboardSent {
		t.Fatalf("stale dismissal removed current toast: %#v", model.copyToast)
	}

	updated, _ = model.Update(dismissCopyToastMsg{generation: model.copyToast.generation})
	if updated.(Model).copyToast.visible() {
		t.Fatal("current dismissal should hide toast")
	}
}

func TestOutputSelectionRestoresManualViewportAcrossScrollResizeAndFocus(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}, {ID: "web", RelPath: "web"}}})
	model.Width = 120
	model.Height = 26
	model.Focus = FocusLogs
	model.LogFollow = false
	model.Logs["api"] = strings.Repeat("0123456789\n", 40)
	model.syncOutputViewport()
	model.outputViewport.SetYOffset(4)
	rect, ok := model.outputContentRect()
	if !ok {
		t.Fatal("Output content rect unavailable")
	}

	updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 4, Y: rect.y + 1, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 4, Y: rect.y + 1, Button: tea.MouseLeft})
	model = updated.(Model)
	wantText := model.outputSelection.text()
	beforeScroll := model.outputSelection.offset
	updated, _ = model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	model = updated.(Model)
	if model.outputSelection.offset <= beforeScroll {
		t.Fatalf("mouse wheel did not scroll snapshot: before=%d after=%d", beforeScroll, model.outputSelection.offset)
	}

	updated, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 26})
	model = updated.(Model)
	if got := model.outputSelection.text(); got != wantText {
		t.Fatalf("resize changed selection text: got %q, want %q", got, wantText)
	}
	model, _ = updateKey(model, "tab")
	if model.Focus != FocusTargets || !model.outputSelection.active() {
		t.Fatalf("focus change should preserve selection: focus=%v state=%#v", model.Focus, model.outputSelection)
	}
	model, _ = updateKey(model, "tab")
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.outputSelection.exists() || model.LogFollow || model.outputViewport.YOffset() != 4 {
		t.Fatalf("clear did not restore manual viewport: state=%#v follow=%t offset=%d", model.outputSelection, model.LogFollow, model.outputViewport.YOffset())
	}
}

func TestOutputSelectionInvalidationAndQuitOwnership(t *testing.T) {
	newSelection := func() Model {
		model := NewModel(Options{
			Command:  "echo ok",
			Targets:  []core.Target{{ID: "api", RelPath: "api", Selected: true}, {ID: "web", RelPath: "web"}},
			startRun: fakeStart(&fakeActiveRun{}, nil),
		})
		model.Width = 120
		model.Height = 26
		model.Focus = FocusLogs
		model.Logs["api"] = "alpha\nbeta\n"
		rect, _ := model.outputContentRect()
		updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
		model = updated.(Model)
		updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 3, Y: rect.y, Button: tea.MouseLeft})
		model = updated.(Model)
		updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 3, Y: rect.y, Button: tea.MouseLeft})
		return updated.(Model)
	}

	model := newSelection()
	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if cmd != nil || !model.ConfirmQuit || !model.outputSelection.active() {
		t.Fatalf("ctrl+c ownership changed: confirm=%t selection=%#v", model.ConfirmQuit, model.outputSelection)
	}

	model = newSelection()
	model.setCursor(1)
	if model.outputSelection.exists() {
		t.Fatal("Target change should invalidate Output selection")
	}

	model = newSelection()
	updated, _ = model.beginRun("echo ok", model.Targets[:1])
	if updated.(Model).outputSelection.exists() {
		t.Fatal("new Run should invalidate Output selection")
	}
}

func TestSimpleOrPaddingClickClearsSelectionAndRestoresFollow(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Width = 120
	model.Height = 26
	model.Focus = FocusLogs
	model.Logs["api"] = "alpha\nbeta\n"
	rect, _ := model.outputContentRect()

	updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.outputSelection.exists() || !model.LogFollow {
		t.Fatalf("simple click should clear and restore follow: %#v/%t", model.outputSelection, model.LogFollow)
	}

	updated, _ = model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 2, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 2, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y + 10, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.outputSelection.exists() || !model.LogFollow {
		t.Fatalf("padding click should clear and restore follow: %#v/%t", model.outputSelection, model.LogFollow)
	}
}

func TestOutputSelectionIgnoresNonLeftMouseMessages(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Width = 120
	model.Height = 26
	model.Logs["api"] = "alpha\n"
	rect, _ := model.outputContentRect()
	updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseRight})
	model = updated.(Model)
	if model.Focus != FocusTargets || model.outputSelection.exists() {
		t.Fatalf("right click should be ignored: focus=%v selection=%#v", model.Focus, model.outputSelection)
	}

	model.Focus = FocusLogs
	updated, _ = model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 2, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 2, Y: rect.y, Button: tea.MouseRight})
	model = updated.(Model)
	if !model.outputSelection.dragging {
		t.Fatal("right release should not finish left-button selection")
	}
}

func TestClipboardMessagesOnlyChangeToast(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.RunError = "keep me"
	updated, timer := model.Update(clipboardResultMsg{status: clipboardFailed, err: errors.New("denied")})
	model = updated.(Model)
	if timer == nil || model.RunError != "keep me" || model.copyToast.status != clipboardFailed {
		t.Fatalf("failure changed unrelated state: error=%q toast=%#v", model.RunError, model.copyToast)
	}

	updated, cmd := model.Update(clipboardOSCMsg{text: "payload"})
	model = updated.(Model)
	if cmd == nil || model.RunError != "keep me" || model.copyToast.status != clipboardSent {
		t.Fatalf("OSC request changed unrelated state: error=%q toast=%#v", model.RunError, model.copyToast)
	}
}

func TestOutputSelectionStyleRemainsVisibleWithoutColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	style := newOutputSelectionStyle()
	if !style.GetReverse() {
		t.Fatal("NO_COLOR selection should use reverse video")
	}
	if _, inherited := style.GetBackground().(lipgloss.NoColor); !inherited {
		t.Fatal("NO_COLOR selection should not force a color background")
	}
}

func TestOutputSelectionRendersHighlightAndTransientToastWithoutBackgroundChrome(t *testing.T) {
	if os.Getenv("RUNNY_OUTPUT_SELECTION_COLOR_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestOutputSelectionRendersHighlightAndTransientToastWithoutBackgroundChrome$")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "NO_COLOR=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "RUNNY_OUTPUT_SELECTION_COLOR_TEST=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("color subprocess: %v\n%s", err, output)
		}
		return
	}
	profile := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = profile })

	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Width = 80
	model.Height = 20
	model.Focus = FocusLogs
	model.Logs["api"] = strings.Repeat("select me\n", 4)
	rect, _ := model.outputContentRect()
	updated, _ := model.Update(tea.MouseClickMsg{X: rect.x, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseMotionMsg{X: rect.x + 4, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)
	updated, _ = model.Update(tea.MouseReleaseMsg{X: rect.x + 4, Y: rect.y, Button: tea.MouseLeft})
	model = updated.(Model)

	rendered := strings.Join(model.renderLogPanel(80, 16), "\n")
	if !containsANSIBackground(rendered) {
		t.Fatalf("selection should render explicit background: %q", rendered)
	}

	model.showCopyToast(clipboardConfirmed)
	rendered = strings.Join(model.renderLogPanel(80, 16), "\n")
	if !strings.Contains(stripANSI(rendered), "✓ Copied to clipboard") {
		t.Fatalf("toast missing:\n%s", stripANSI(rendered))
	}
	for name, style := range map[string]lipgloss.Style{
		"success": copyToastSuccessStyle,
		"sent":    copyToastSentStyle,
		"error":   copyToastErrorStyle,
	} {
		if _, inherited := style.GetBackground().(lipgloss.NoColor); !inherited {
			t.Fatalf("%s toast defines explicit background", name)
		}
	}
}

func TestOutputCopyVisualGolden(t *testing.T) {
	if os.Getenv("RUNNY_OUTPUT_COPY_GOLDEN_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestOutputCopyVisualGolden$")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "NO_COLOR=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "RUNNY_OUTPUT_COPY_GOLDEN_TEST=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("color subprocess: %v\n%s", err, output)
		}
		return
	}
	profile := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = profile })

	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Focus = FocusLogs
	model.outputSelection = outputSelection{targetID: "api", moved: true}
	parts := []string{
		"selection=" + strconv.Quote(model.renderSelectedOutputRow("alpha beta", outputVisualRow{start: 0, end: 10}, 0, 5, true)),
	}
	for _, state := range []struct {
		name   string
		status clipboardStatus
		width  int
	}{
		{name: "confirmed-wide", status: clipboardConfirmed, width: 60},
		{name: "sent-narrow", status: clipboardSent, width: 40},
		{name: "failed-wide", status: clipboardFailed, width: 60},
		{name: "empty-narrow", status: clipboardEmpty, width: 40},
	} {
		model.copyToast.status = state.status
		parts = append(parts, state.name+"="+strconv.Quote(strings.Join(model.renderCopyToast(state.width), "\n")))
	}
	parts = append(parts, "footer="+strconv.Quote(model.renderFooter(100)))
	got := strings.Join(parts, "\n")
	want, err := os.ReadFile("testdata/TestOutputCopyVisualGolden.golden")
	if err != nil {
		t.Fatalf("read golden: %v\n--- got ---\n%s", err, got)
	}
	if got != strings.TrimSpace(string(want)) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
