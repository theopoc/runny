package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
)

func TestOptionsScreenOpensNavigatesTogglesAndCloses(t *testing.T) {
	model := NewModel(Options{})
	model, _ = updateKey(model, "o")
	if !model.ShowOptions || model.OptionsPos != 0 {
		t.Fatalf("options state = open %t pos %d", model.ShowOptions, model.OptionsPos)
	}

	model, _ = updateSpecialKey(model, ' ')
	if model.Mode != core.ModeSerial {
		t.Fatalf("mode = %q, want serial", model.Mode)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	model, _ = updateSpecialKey(model, tea.KeyDown)
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if !model.FailFast {
		t.Fatal("enter should toggle fail fast")
	}
	model, _ = updateKey(model, "o")
	if model.ShowOptions {
		t.Fatal("o should close options overlay")
	}
}

func TestOptionsScreenNavigationCategoriesBoundsAndEscape(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.OptionsPos != 0 {
		t.Fatalf("up at first option moved to %d", model.OptionsPos)
	}
	for range len(sessionOptions) + 2 {
		model, _ = updateSpecialKey(model, tea.KeyDown)
	}
	if model.OptionsPos != int(optionFailFast) {
		t.Fatalf("down past last option moved to %d", model.OptionsPos)
	}
	model, _ = updateSpecialKey(model, tea.KeyRight)
	if model.OptionsTab != 1 || model.OptionsPos != int(optionCaptureOutput) {
		t.Fatalf("right category = tab %d pos %d", model.OptionsTab, model.OptionsPos)
	}
	model, _ = updateKey(model, "3")
	if model.OptionsTab != 2 || model.OptionsPos != int(optionFollowOutput) {
		t.Fatalf("number category = tab %d pos %d", model.OptionsTab, model.OptionsPos)
	}
	model, _ = updateSpecialKey(model, tea.KeyLeft)
	if model.OptionsTab != 1 || model.OptionsPos != int(optionCaptureOutput) {
		t.Fatalf("left category = tab %d pos %d", model.OptionsTab, model.OptionsPos)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowOptions {
		t.Fatal("escape should close options overlay")
	}
}

func TestOptionsOverlayAdjustsWorkersAndReturnsToAuto(t *testing.T) {
	model := NewModel(Options{Mode: core.ModeSerial})
	model.ShowOptions = true
	model.OptionsPos = int(optionWorkers)

	model, _ = updateKey(model, "+")
	if model.Workers != 1 || model.Mode != core.ModeParallel || model.Notice != "workers set to 1" {
		t.Fatalf("workers/mode/notice = %d/%s/%q, want 1/parallel/set notice", model.Workers, model.Mode, model.Notice)
	}
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if model.Workers != 2 {
		t.Fatalf("enter workers = %d, want 2", model.Workers)
	}
	model, _ = updateKey(model, "-")
	model, _ = updateKey(model, "-")
	if model.Workers != 0 || model.Notice != "workers set to auto" {
		t.Fatalf("decrement workers/notice = %d/%q, want auto", model.Workers, model.Notice)
	}
	model, _ = updateKey(model, "+")
	model, _ = updateKey(model, "a")
	if model.Workers != 0 || model.Mode != core.ModeParallel || model.Notice != "workers set to auto" {
		t.Fatalf("auto workers/mode/notice = %d/%s/%q", model.Workers, model.Mode, model.Notice)
	}
}

func TestOptionsOverlayVimNavigationMovesTabsAndSelection(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true

	model, _ = updateKey(model, "l")
	if model.OptionsTab != 1 || model.OptionsPos != int(optionCaptureOutput) {
		t.Fatalf("l selection = tab %d pos %d, want logging/capture", model.OptionsTab, model.OptionsPos)
	}
	model, _ = updateKey(model, "j")
	if model.OptionsTab != 1 || model.OptionsPos != int(optionSaveLogs) {
		t.Fatalf("j selection = tab %d pos %d, want logging/save logs", model.OptionsTab, model.OptionsPos)
	}
	model, _ = updateKey(model, "h")
	if model.OptionsTab != 0 || model.OptionsPos != int(optionSerial) {
		t.Fatalf("h selection = tab %d pos %d, want execution/serial", model.OptionsTab, model.OptionsPos)
	}
}

func TestPaletteCanOpenOptionsOverlay(t *testing.T) {
	model := NewModel(Options{})
	model, _ = runPaletteCommand(model, "options")
	if !model.ShowOptions || model.OptionsPos != 0 {
		t.Fatalf("palette options state = open %t pos %d", model.ShowOptions, model.OptionsPos)
	}
}

func TestOptionsOverlayLoggingInvariants(t *testing.T) {
	model := NewModel(Options{SaveLogs: true})
	model.ShowOptions = true
	model.OptionsTab = 1
	model.OptionsPos = int(optionCaptureOutput)
	model, _ = updateSpecialKey(model, ' ')
	if !model.DisableLogging || model.SaveLogs {
		t.Fatalf("capture/save = %t/%t, want disabled/false", !model.DisableLogging, model.SaveLogs)
	}

	model.OptionsPos = int(optionSaveLogs)
	model, _ = updateSpecialKey(model, ' ')
	if model.SaveLogs || model.Notice != "save logs requires capture output" {
		t.Fatalf("save logs state/notice = %t/%q", model.SaveLogs, model.Notice)
	}
}

func TestOptionsOverlayLocksExecutionButKeepsViewMutableDuringRun(t *testing.T) {
	model := NewModel(Options{Workers: 3, Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusRunning
	model.ShowOptions = true
	model.OptionsPos = int(optionFailFast)
	model, _ = updateSpecialKey(model, ' ')
	if model.FailFast || !strings.Contains(model.Notice, "locked") {
		t.Fatalf("fail fast/notice = %t/%q", model.FailFast, model.Notice)
	}

	model.OptionsPos = int(optionWorkers)
	model, _ = updateKey(model, "+")
	model, _ = updateKey(model, "a")
	if model.Workers != 3 || !strings.Contains(model.Notice, "locked") {
		t.Fatalf("workers/notice = %d/%q, want locked at 3", model.Workers, model.Notice)
	}

	model.OptionsTab = 2
	model.OptionsPos = int(optionFollowOutput)
	model, _ = updateSpecialKey(model, ' ')
	if model.LogFollow {
		t.Fatal("view option should remain mutable during run")
	}
}

func TestOptionsOverlayRendersCompactResponsiveInheritedBackground(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true
	for _, width := range []int{60, 100} {
		rendered := model.renderOptionsOverlay(width, 17)
		plain := stripANSI(rendered)
		for _, want := range []string{"Options · session", "Execution", "Logging", "Display", "Serial execution", "Workers", "AUTO", "Stop on first failure", "○ OFF", "Runs targets one by one"} {
			if !strings.Contains(plain, want) {
				t.Fatalf("width %d missing %q:\n%s", width, want, plain)
			}
		}
		for _, unwanted := range []string{"Capture output", "Follow output", "Maximize pane", "[x]", "[ ]"} {
			if strings.Contains(plain, unwanted) {
				t.Fatalf("width %d unexpectedly contains %q:\n%s", width, unwanted, plain)
			}
		}
		if got := maxLineWidth(rendered); got > width {
			t.Fatalf("width %d rendered as %d:\n%s", width, got, plain)
		}
		wantWidth := min(68, max(52, width*2/3))
		if got := maxLineWidth(rendered); got != wantWidth {
			t.Fatalf("width %d overlay width = %d, want %d:\n%s", width, got, wantWidth, plain)
		}
		if got := len(strings.Split(rendered, "\n")); got != 10 {
			t.Fatalf("width %d overlay height = %d, want 10:\n%s", width, got, plain)
		}
		for _, line := range strings.Split(rendered, "\n") {
			if containsANSIBackground(line) {
				t.Fatalf("option screen paints background: %q", line)
			}
		}
	}
}

func TestOptionsOverlayRendersConfiguredWorkersAndGuidance(t *testing.T) {
	model := NewModel(Options{Workers: 4})
	model.ShowOptions = true
	model.OptionsPos = int(optionWorkers)
	rendered := stripANSI(model.renderOptionsOverlay(80, 17))
	for _, want := range []string{"Workers", "4", "Max parallel runs. +/- adjusts; a sets auto."} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("workers option missing %q:\n%s", want, rendered)
		}
	}
}

func TestOptionsScreenShowsOnlySelectedCategoryAndDetail(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true
	model.OptionsTab = 1
	model.OptionsPos = int(optionSaveLogs)
	rendered := stripANSI(model.renderOptionsOverlay(80, 17))
	for _, want := range []string{"Execution", "Logging", "Display", "Capture output", "Save logs", "Persists captured output after each run."} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("logging category missing %q:\n%s", want, rendered)
		}
	}
	for _, unwanted := range []string{"Serial execution", "Follow output", "Maximize pane"} {
		if strings.Contains(rendered, unwanted) {
			t.Fatalf("logging category contains %q:\n%s", unwanted, rendered)
		}
	}
}

func TestOptionsDisplaySubmenuOnlyShowsFollowOutput(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true
	model.OptionsTab = 2
	model.OptionsPos = int(optionFollowOutput)
	rendered := stripANSI(model.renderOptionsOverlay(60, 17))
	for _, want := range []string{"[3 Display]", "Follow output", "Keeps output pinned to newest line."} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("display category missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "Maximize") {
		t.Fatalf("display category should not contain maximize option:\n%s", rendered)
	}
}

func TestOptionsOverlayKeepsPanelsVisibleAroundIt(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Width = 100
	model.Height = 26
	model.ShowOptions = true
	rendered := stripANSI(model.render())
	for _, want := range []string{"Options · session", "Serial execution", "Tasks", "Output", "api"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("options screen missing %q:\n%s", want, rendered)
		}
	}
	if got := strings.Count(rendered, "Options · session"); got != 1 {
		t.Fatalf("options overlay title count = %d, want 1:\n%s", got, rendered)
	}
	if got := len(strings.Split(rendered, "\n")); got != model.Height {
		t.Fatalf("rendered height = %d, want %d", got, model.Height)
	}
}

func TestOptionsScreenShowsLockedStateWithoutDuplicateToggle(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusRunning
	model.ShowOptions = true
	rendered := stripANSI(model.renderOptionsOverlay(80, 17))
	if !strings.Contains(rendered, "◇ LOCKED") || !strings.Contains(rendered, "Locked until active runs finish") {
		t.Fatalf("locked option state missing:\n%s", rendered)
	}
}

func TestOptionsOverlayFooterFitsNarrowWidth(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true
	footer := model.renderFooter(60)
	if got := maxLineWidth(footer); got > 60 {
		t.Fatalf("options footer width = %d:\n%s", got, stripANSI(footer))
	}
	for _, want := range []string{"[space] Toggle", "[right] Next", "[esc] Close", "[?] Help"} {
		if !strings.Contains(stripANSI(footer), want) {
			t.Fatalf("options footer missing %q:\n%s", want, stripANSI(footer))
		}
	}
}

func TestOptionsOverlayWorkersFooterFitsNarrowWidth(t *testing.T) {
	model := NewModel(Options{})
	model.ShowOptions = true
	model.OptionsPos = int(optionWorkers)
	footer := model.renderFooter(60)
	if got := maxLineWidth(footer); got > 60 {
		t.Fatalf("workers footer width = %d:\n%s", got, stripANSI(footer))
	}
	for _, want := range []string{"[+/-] Adjust", "[a] Auto", "[up/down] Choose", "[esc] Close"} {
		if !strings.Contains(stripANSI(footer), want) {
			t.Fatalf("workers footer missing %q:\n%s", want, stripANSI(footer))
		}
	}
}
