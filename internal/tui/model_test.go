package tui

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
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
	if model.Focus != FocusCommand || model.Notice != "editing command" {
		t.Fatalf("focus/notice = %v/%q, want command/editing command", model.Focus, model.Notice)
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

func TestFooterIsContextual(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	tasksFooter := stripANSI(model.renderFooter(120))
	if got := len(strings.Split(tasksFooter, "\n")); got != 3 {
		t.Fatalf("tasks footer lines = %d, want 3:\n%s", got, tasksFooter)
	}
	normalizedTasksFooter := strings.Join(strings.Fields(tasksFooter), " ")
	for _, want := range []string{"H History", "tab Panels", "z Maximize panel", "space Select", "a Select All", "←/→ Fold", "enter Run"} {
		if !strings.Contains(normalizedTasksFooter, want) {
			t.Fatalf("tasks footer should contain %q:\n%s", want, tasksFooter)
		}
	}
	assertFooterColumnsAligned(t, tasksFooter)
	for _, hidden := range []string{"tab focus", "c Command", "a Toggle", "h/l Fold"} {
		if strings.Contains(tasksFooter, hidden) {
			t.Fatalf("tasks footer should not use stale label %q:\n%s", hidden, tasksFooter)
		}
	}
	compactTasksFooter := stripANSI(model.renderFooter(80))
	if got := len(strings.Split(compactTasksFooter, "\n")); got != 3 {
		t.Fatalf("compact tasks footer lines = %d, want 3:\n%s", got, compactTasksFooter)
	}
	for _, want := range []string{"tab Output", ": Cmd", "enter Run"} {
		if !strings.Contains(strings.Join(strings.Fields(compactTasksFooter), " "), want) {
			t.Fatalf("compact tasks footer should contain %q:\n%s", want, compactTasksFooter)
		}
	}
	if got := maxLineWidth(compactTasksFooter); got > 80 {
		t.Fatalf("compact tasks footer width = %d:\n%s", got, compactTasksFooter)
	}

	model.Status["api"] = core.StatusFailed
	failedFooter := stripANSI(model.renderFooter(120))
	if !strings.Contains(strings.Join(strings.Fields(failedFooter), " "), "R Failed") {
		t.Fatalf("failed footer should expose rerun failed shortcut:\n%s", failedFooter)
	}

	model.Status["api"] = core.StatusRunning
	model.Targets = append(model.Targets, core.Target{ID: "web", RelPath: "web", Selected: true})
	model.Status["web"] = core.StatusFailed
	activeFooter := stripANSI(model.renderFooter(120))
	if strings.Contains(strings.Join(strings.Fields(activeFooter), " "), "R Failed") {
		t.Fatalf("active footer should hide rerun failed while work is active:\n%s", activeFooter)
	}
	if !strings.Contains(strings.Join(strings.Fields(activeFooter), " "), "ctrl+c Cancel+quit") {
		t.Fatalf("active footer should keep stop hint visible:\n%s", activeFooter)
	}
	if strings.Count(strings.Join(strings.Fields(activeFooter), " "), "ctrl+c Cancel+quit") != 1 {
		t.Fatalf("active footer should show one stop hint:\n%s", activeFooter)
	}

	filterModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	filterModel.Focus = FocusFilter
	filterFooter := stripANSI(filterModel.renderFooter(120))
	for _, want := range []string{"Type fuzzy", "' Exact", "ctrl+u Clear", "↑↓/nN Matches", "enter/esc Tasks"} {
		if !strings.Contains(strings.Join(strings.Fields(filterFooter), " "), want) {
			t.Fatalf("filter footer should contain %q:\n%s", want, filterFooter)
		}
	}
	compactFilterFooter := stripANSI(filterModel.renderFooter(80))
	for _, want := range []string{"Type fuzzy", "' Exact", "↑↓/nN", "ctrl+u Clear", "enter/esc"} {
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
	if !strings.Contains(strings.Join(strings.Fields(filteredTasksFooter), " "), "a Select All") {
		t.Fatalf("filtered task footer should keep select-all label:\n%s", filteredTasksFooter)
	}

	historyModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	historyModel.ShowHistory = true
	emptyHistoryFooter := stripANSI(historyModel.renderFooter(120))
	normalizedEmptyHistoryFooter := strings.Join(strings.Fields(emptyHistoryFooter), " ")
	if !strings.Contains(normalizedEmptyHistoryFooter, "No reuse") || strings.Contains(normalizedEmptyHistoryFooter, "enter Reuse") {
		t.Fatalf("empty history footer should not promise reuse:\n%s", emptyHistoryFooter)
	}
	historyModel.History = []string{"go test"}
	historyFooter := stripANSI(historyModel.renderFooter(120))
	if !strings.Contains(strings.Join(strings.Fields(historyFooter), " "), "enter Reuse") {
		t.Fatalf("history footer should expose reuse when selectable:\n%s", historyFooter)
	}

	commandModel := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	commandModel.Focus = FocusCommand
	commandFooter := stripANSI(commandModel.renderFooter(80))
	for _, want := range []string{"enter Run", "esc Tasks", "↑↓ Hist", "ctrl+u Clear"} {
		if !strings.Contains(strings.Join(strings.Fields(commandFooter), " "), want) {
			t.Fatalf("compact command footer should contain %q:\n%s", want, commandFooter)
		}
	}
	if got := maxLineWidth(commandFooter); got > 80 {
		t.Fatalf("compact command footer width = %d:\n%s", got, commandFooter)
	}

	logsModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	logsModel.Focus = FocusLogs
	logsFooter := stripANSI(logsModel.renderFooter(80))
	for _, want := range []string{"pgup/pgdn Scroll", "f Tail", "tab Tasks"} {
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

func TestFooterShortcutColorsUseTrueColorAndHelperBackground(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	footer := model.renderFooter(80)

	for _, want := range []string{
		"\x1b[48;2;36;47;56m",
		"\x1b[38;2;224;224;224m",
		"\x1b[38;2;196;181;253m",
		"\x1b[1menter\x1b[22m",
	} {
		if !strings.Contains(footer, want) {
			t.Fatalf("footer should contain ANSI sequence %q:\n%q", want, footer)
		}
	}
}

func TestNoticeBarUsesPurpleBackgroundAndWhiteText(t *testing.T) {
	if noticeForegroundHex != "#FFFFFF" {
		t.Fatalf("notice foreground = %q, want white", noticeForegroundHex)
	}
	if primaryAccentHex != "#A78BFA" {
		t.Fatalf("notice background = %q, want purple", primaryAccentHex)
	}
}

func TestRunningRowsUseWhiteTextOnNaturalBackground(t *testing.T) {
	if noticeForegroundHex != "#FFFFFF" {
		t.Fatalf("running foreground = %q, want white", noticeForegroundHex)
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
	for _, want := range []string{":run", ":workers N|auto", ":rerun-failed", ":cancel", ":cancel-all", ":history"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help should mention palette command %q:\n%s", want, help)
		}
	}
}

func TestHelpMentionsHistoryKeys(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	help := stripANSI(strings.Join(model.helpRows(), "\n"))
	for _, want := range []string{"H open history", "/ search runs", "' exact match", "enter reuse command"} {
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
	for _, want := range []string{"? Close", "q Close", "esc Close", "H History"} {
		if !strings.Contains(normalizedHelpFooter, want) {
			t.Fatalf("help footer should contain %q:\n%s", want, helpFooter)
		}
	}
	if strings.Contains(normalizedHelpFooter, "space Select") {
		t.Fatalf("help footer should not show task actions:\n%s", helpFooter)
	}

	model.ShowHelp = false
	model.ShowPalette = true
	paletteFooter := stripANSI(model.renderFooter(120))
	normalizedPaletteFooter := normalizeFooterText(paletteFooter)
	for _, want := range []string{"enter Choose", "up/down Choose", "ctrl+u Clear", "esc Close"} {
		if !strings.Contains(normalizedPaletteFooter, want) {
			t.Fatalf("palette footer should contain %q:\n%s", want, paletteFooter)
		}
	}

	model.ShowPalette = false
	model.ShowHistory = true
	model.History = []string{"go test"}
	historyFooter := stripANSI(model.renderFooter(120))
	normalizedHistoryFooter := normalizeFooterText(historyFooter)
	for _, want := range []string{"enter Reuse", "up/down Choose", "esc Close", "? Keymap"} {
		if !strings.Contains(normalizedHistoryFooter, want) {
			t.Fatalf("history footer should contain %q:\n%s", want, historyFooter)
		}
	}

	model.ShowHistory = false
	model.ConfirmCancelAll = true
	cancelFooter := stripANSI(model.renderFooter(120))
	normalizedCancelFooter := normalizeFooterText(cancelFooter)
	for _, want := range []string{"y Confirm", "enter Confirm", "n Cancel", "esc Cancel"} {
		if !strings.Contains(normalizedCancelFooter, want) {
			t.Fatalf("cancel-all footer should contain %q:\n%s", want, cancelFooter)
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
	model, _ = updateKey(model, ":")
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
	if view := stripANSI(model.renderSubHeader(100)); !strings.Contains(view, "/ w▌") {
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
	if !strings.Contains(normalizeFooterText(footer), "↑↓/nN Matches") {
		t.Fatalf("filter footer should mention arrow match navigation:\n%s", footer)
	}
}

func TestTargetFooterShortcutLabels(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
	}})
	footer := stripANSI(model.renderFooter(140))

	for _, want := range []string{"? Keymap", "/ Search", ": Command", "a Select All", "←/→ Fold"} {
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

func TestSubHeaderShowsOnlyCommandPrompt(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
	}})
	model.Cursor = 1
	model.Filter = "api"

	subheader := stripANSI(model.renderSubHeader(140))
	for _, want := range []string{"Command", "test"} {
		if !strings.Contains(subheader, want) {
			t.Fatalf("subheader should show %q:\n%s", want, subheader)
		}
	}
	for _, hidden := range []string{"focus", "visible", "selected", "match", "filter", "path"} {
		if strings.Contains(subheader, hidden) {
			t.Fatalf("subheader should hide %q metadata:\n%s", hidden, subheader)
		}
	}
	compact := stripANSI(model.renderSubHeader(80))
	if strings.Contains(compact, "path api") {
		t.Fatalf("compact subheader should omit path breadcrumb:\n%s", compact)
	}
	if got := maxLineWidth(model.renderSubHeader(80)); got > 80 {
		t.Fatalf("compact subheader width = %d:\n%s", got, compact)
	}
}

func TestCommandInputStartsEmptyWithoutCursor(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	rendered := model.renderSubHeader(80)
	plain := stripANSI(rendered)
	for _, want := range []string{"Command", "┌", "│", "└"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("command input should show %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"▌", "<type command", ">"} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("empty unfocused command input should hide %q:\n%s", hidden, plain)
		}
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("command input should include styling:\n%s", rendered)
	}
}

func TestHeaderDoesNotShowFocusedPath(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true, Children: []string{"api/cmd"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Selected: true},
	}})
	model.Cursor = 1

	header := stripANSI(model.renderHeader(140))
	if strings.Contains(header, "path") || strings.Contains(header, "api/cmd") {
		t.Fatalf("header should not show focused path:\n%s", header)
	}
	for _, want := range []string{"runny", "mode", "workers", "targets"} {
		if !strings.Contains(header, want) {
			t.Fatalf("header should keep %q:\n%s", want, header)
		}
	}
}

func TestCommandFocusAcceptsSlashAndSpace(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusCommand
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
	if view := stripANSI(model.renderSubHeader(100)); !strings.Contains(view, "./sh -c") {
		t.Fatalf("command focus should show command input:\n%s", view)
	}
	if view := stripANSI(model.renderSubHeader(100)); !strings.Contains(view, "./sh -c▌") {
		t.Fatalf("command focus should show cursor:\n%s", view)
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
	model.Focus = FocusCommand
	model = typeText(model, "echo")
	model, _ = updateSpecialKey(model, tea.KeySpace)

	view := stripANSI(model.renderSubHeader(100))
	if !strings.Contains(view, "echo ▌") {
		t.Fatalf("command focus should show trailing space before cursor:\n%s", view)
	}
}

func TestCommandFocusNavigatesHistory(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Focus = FocusCommand
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
	model.Focus = FocusCommand
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
	for _, want := range []string{"api", "api/cmd", "−"} {
		if !strings.Contains(view, want) {
			t.Fatalf("filter should reveal folded match/context %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "+") {
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
	targets := make([]core.Target, 0, 12)
	for i := 0; i < 12; i++ {
		id := "svc-" + string(rune('a'+i))
		targets = append(targets, core.Target{ID: id, RelPath: id, Selected: true})
	}
	model := NewModel(Options{Command: "test", Targets: targets})
	model, _ = updateWindowSize(model, 80, 20)
	for i := 0; i < 9; i++ {
		model, _ = updateSpecialKey(model, tea.KeyDown)
	}

	view := stripANSI(model.render())
	if strings.Contains(view, "›") || !strings.Contains(view, "svc-j") {
		t.Fatalf("directory panel should scroll focused row into view:\n%s", view)
	}
	if strings.Contains(view, "svc-a") {
		t.Fatalf("directory panel should scroll past first rows:\n%s", view)
	}
	for _, want := range []string{"showing", "of 12", "↑", "↓"} {
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
	statusIndex := strings.Index(row, "● running")
	if headerIndex < 0 || statusIndex < 0 || lipgloss.Width(header[:headerIndex]) != lipgloss.Width(row[:statusIndex]) {
		t.Fatalf("status column header=%d row=%d\n%s\n%s", headerIndex, statusIndex, header, row)
	}
	if strings.Contains(row[:statusIndex], "●") || strings.Contains(row[:statusIndex], "○") {
		t.Fatalf("target row should not include selection marker before status:\n%s", row)
	}
}

func TestTargetTreeShowsDeepContinuationGuide(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Children: []string{"api/cmd", "api/pkg"}},
		{ID: "api/cmd", RelPath: "api/cmd", ParentID: "api", Depth: 2, Children: []string{"api/cmd/foo"}},
		{ID: "api/cmd/foo", RelPath: "api/cmd/foo", ParentID: "api/cmd", Depth: 3},
		{ID: "api/pkg", RelPath: "api/pkg", ParentID: "api", Depth: 2},
	}})
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	if !strings.Contains(view, "│ └─ 📁 api/cmd/foo") {
		t.Fatalf("deep tree should keep a continuation guide:\n%s", view)
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
	if !strings.Contains(unfocusedSelected, "\x1b[1;") && !strings.Contains(unfocusedSelected, "\x1b[1m") {
		t.Fatalf("unfocused selected target should keep selected emphasis:\n%q", unfocusedSelected)
	}
}

func TestNavigationHighlightDiffersFromSelectionHighlight(t *testing.T) {
	selectedModel := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})

	selected := selectedModel.renderTargetRow(0, selectedModel.Targets[0], 70)
	if rowSelectedStyle.GetBackground() != runnyTheme.bgSelection {
		t.Fatalf("selected highlight should keep purple background")
	}
	if rowActiveStyle.GetBackground() != runnyTheme.bgFocus {
		t.Fatalf("navigation highlight should use blue background")
	}
	if rowActiveSelectedStyle.GetBackground() != runnyTheme.bgFocus {
		t.Fatalf("focused selected target should use navigation highlight")
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
	if strings.Contains(row, "+") {
		t.Fatalf("leaf target should not show fold marker:\n%s", row)
	}
}

func TestSelectedTargetRowsDoNotRenderSelectionMarkers(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: false},
	}})
	view := stripANSI(strings.Join(model.renderDirectoryPanel(80, 10), "\n"))

	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "📁 api") || strings.Contains(line, "📁 web") {
			statusIndex := strings.LastIndex(line, "○ idle")
			if statusIndex < 0 {
				t.Fatalf("target row should keep status text:\n%s", line)
			}
			beforeStatus := line[:statusIndex]
			if strings.Contains(beforeStatus, "●") || strings.Contains(beforeStatus, "○") {
				t.Fatalf("target row should not render selection marker:\n%s", line)
			}
		}
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
	model, _ = updateKey(model, ":")
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
	model, _ = updateKey(model, ":")
	model, _ = updateKey(model, "q")
	if !model.ShowPalette || model.Palette != "q" {
		t.Fatalf("q should type inside command palette, show/palette = %v/%q", model.ShowPalette, model.Palette)
	}
	model, _ = updateSpecialKey(model, tea.KeyEsc)
	if model.ShowPalette {
		t.Fatal("escape should close command palette")
	}

	model, _ = runPaletteCommand(model, "command")
	if model.Focus != FocusCommand {
		t.Fatalf(":command should focus command input, focus = %v", model.Focus)
	}
	model.Focus = FocusTargets

	model, _ = runPaletteCommand(model, "workers")
	if model.RunError != "usage: :workers N|auto" {
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
	model, _ = updateKey(model, ":")
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
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, cmd := runPaletteCommand(model, "run")
	if cmd == nil {
		t.Fatal(":run should start selected targets")
	}
	if !model.Running || model.Status["api"] != core.StatusRunning {
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
	cancelled := false
	model.Running = true
	model.PendingRuns = 1
	model.Status["api"] = core.StatusRunning
	model.targetCancels = map[string]context.CancelFunc{"api": func() { cancelled = true }}

	model, _ = runPaletteCommand(model, "cancel")
	if !cancelled {
		t.Fatal(":cancel should call target cancel function")
	}
	if model.Status["api"] != core.StatusCancelled {
		t.Fatalf("status = %s, want cancelled", model.Status["api"])
	}
}

func TestModelCommandPaletteConfirmsCancelAll(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	cancelled := false
	model.Running = true
	model.PendingRuns = 2
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusQueued
	model.cancelRun = func() { cancelled = true }

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
	if cancelled || model.Status["api"] != core.StatusRunning || model.Status["web"] != core.StatusQueued {
		t.Fatalf("n should cancel confirmation only, cancelled/statuses = %v/%#v", cancelled, model.Status)
	}
	if model.Notice != "confirmation cancelled" {
		t.Fatalf("notice = %q", model.Notice)
	}
	model.ConfirmCancelAll = true
	model, _ = updateKey(model, "y")
	if !cancelled {
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

	model, _ = updateKey(model, ":")
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
	model, _ = updateKey(model, "pagedown")
	if model.PreviewOffset != 5 {
		t.Fatalf("preview offset = %d, want 5", model.PreviewOffset)
	}
	if model.LogFollow {
		t.Fatal("manual preview scroll should disable tail mode")
	}
	model, _ = updateKey(model, "f")
	if !model.LogFollow {
		t.Fatal("f should re-enable tail mode")
	}
}

func TestCtrlCCancelsAndQuitsFromOverlay(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	cancelled := false
	model.Running = true
	model.ShowHelp = true
	model.cancelRun = func() { cancelled = true }
	model.Status["api"] = core.StatusRunning

	updated, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("ctrl+c should quit even when overlay is open")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c should return quit message")
	}
	if !cancelled {
		t.Fatal("ctrl+c should cancel active run")
	}
	if model.Status["api"] != core.StatusCancelled {
		t.Fatalf("status = %s", model.Status["api"])
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
	for _, want := range []string{"Keymap", ": palette", "H history", "del/x cancel selected"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("view content should contain %q:\n%s", want, plain)
		}
	}
	for _, want := range []string{"Runs and status", "▶ running", "… queued", "! failed", "× cancelled"} {
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
	rows = stripANSI(strings.Join(model.paletteRows(), "\n"))
	if !strings.Contains(rows, "more command(s); keep typing to narrow") {
		t.Fatalf("truncated palette rows should show a narrow hint:\n%s", rows)
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
		{name: "confirm", setup: func(m *Model) { m.ConfirmRun = true }, want: "Rerun failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := base
			tc.setup(&model)
			model, _ = updateWindowSize(model, 80, 20)
			view := stripANSI(model.View().Content)
			if !strings.Contains(view, tc.want) {
				t.Fatalf("overlay should contain %q:\n%s", tc.want, view)
			}
			if lines := strings.Count(view, "\n") + 1; lines > 20 {
				t.Fatalf("%s overlay should fit minimum height, lines = %d:\n%s", tc.name, lines, view)
			}
			if got := maxLineWidth(view); got > 80 {
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
		{79, 24},
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
	for _, width := range []int{80, 100, 120} {
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

func TestOutputPanelShowsOnlyCommandOutput(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Status["api"] = core.StatusFailed
	model.Logs["api"] = "ok line\nexit status 1\n"
	model.TargetStarted["api"] = time.Date(2026, 7, 3, 14, 5, 6, 0, time.Local)
	model, _ = updateWindowSize(model, 90, 24)
	view := model.renderLogPanel(40, 16)
	rendered := strings.Join(view, "\n")
	plain := stripANSI(rendered)
	for _, want := range []string{"Output", "ok line", "exit status 1"} {
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

func TestPreviewOutputRangeShowsManualScroll(t *testing.T) {
	model := NewModel(Options{Command: "go test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.Logs["api"] = strings.Join([]string{
		"line-01", "line-02", "line-03", "line-04", "line-05", "line-06", "line-07", "line-08",
		"line-09", "line-10", "line-11", "line-12", "line-13", "line-14", "line-15",
		"line-16", "line-17", "line-18", "line-19", "line-20",
	}, "\n")
	model.LogFollow = false
	model.PreviewOffset = 2

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
	model.PendingRuns = 3
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusQueued
	model.Status["worker"] = core.StatusFailed
	dashboard := stripANSI(model.renderDashboard(120))
	if !strings.Contains(dashboard, "running 1/3") {
		t.Fatalf("dashboard should show running progress:\n%s", dashboard)
	}
	if !strings.Contains(dashboard, "1/3 33%") {
		t.Fatalf("dashboard should show progress percent:\n%s", dashboard)
	}
	rows := stripANSI(strings.Join(model.renderDirectoryPanel(80, 12), "\n"))
	for _, want := range []string{"▶", "…", "!"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("task rows should contain activity marker %q:\n%s", want, rows)
		}
	}
}

func TestDashboardProgressUsesSelectedTargetsOnly(t *testing.T) {
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
	if !strings.Contains(dashboard, "9/9 100%") {
		t.Fatalf("dashboard progress should use selected targets only:\n%s", dashboard)
	}
	if strings.Contains(dashboard, "9/24 37%") {
		t.Fatalf("dashboard progress should not use total discovered targets:\n%s", dashboard)
	}
}

func TestModelRunsCommandAndShowsResults(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		result := core.RunResult{Target: req.Targets[0], Status: core.StatusSucceeded, Output: req.Targets[0].ID + " ok\n"}
		if req.Targets[0].ID == "web" {
			result.Status = core.StatusFailed
			result.Output = "web bad\n"
			result.Error = "exit status 1"
		}
		return []core.RunResult{
			result,
		}, nil
	}

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	if cmd == nil {
		t.Fatal("enter should start a run")
	}
	if !model.Running {
		t.Fatal("model should be running")
	}
	if model.Status["api"] != core.StatusRunning || model.Status["web"] != core.StatusRunning {
		t.Fatalf("statuses = %#v", model.Status)
	}

	model = applyCmd(t, model, cmd)
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
	if model.RunError != "command is empty; press c to edit" {
		t.Fatalf("run error = %q", model.RunError)
	}
	if model.Focus != FocusCommand {
		t.Fatalf("focus = %v, want command input after empty run", model.Focus)
	}
	if subheader := stripANSI(model.renderSubHeader(100)); !strings.Contains(subheader, "Command") || !strings.Contains(subheader, "▌") || strings.Contains(subheader, "<type command") {
		t.Fatalf("empty run should show command cursor:\n%s", subheader)
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

func TestMessageBarsAreFullWidthAndStyled(t *testing.T) {
	model := NewModel(Options{Command: "test", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model, _ = updateWindowSize(model, 80, 24)
	model.RunError = "command is empty; press c to edit"
	view := model.View().Content
	plain := stripANSI(view)
	if !strings.Contains(plain, "ERROR  command is empty; press c to edit") {
		t.Fatalf("error bar missing:\n%s", plain)
	}
	if !strings.Contains(view, "\x1b[") {
		t.Fatal("error bar should be styled")
	}
	if got := maxLineWidth(view); got > 80 {
		t.Fatalf("error view width = %d:\n%s", got, plain)
	}

	model.RunError = ""
	model.Notice = "workers set to 2"
	view = model.View().Content
	plain = stripANSI(view)
	if !strings.Contains(plain, "INFO  workers set to 2") {
		t.Fatalf("notice bar missing:\n%s", plain)
	}
	if got := maxLineWidth(view); got > 80 {
		t.Fatalf("notice view width = %d:\n%s", got, plain)
	}

	model.Status["api"] = core.StatusFailed
	model.Notice = "run complete: 0 ok, 1 failed, 0 cancelled · press R to rerun failed"
	view = model.View().Content
	plain = stripANSI(view)
	if !strings.Contains(plain, "WARN  run complete: 0 ok, 1 failed") {
		t.Fatalf("failed completion should render warning bar:\n%s", plain)
	}
	if got := maxLineWidth(view); got > 80 {
		t.Fatalf("warning view width = %d:\n%s", got, plain)
	}

	model.Status["api"] = core.StatusCancelled
	model.Notice = "run complete: 0 ok, 0 failed, 1 cancelled"
	view = model.View().Content
	plain = stripANSI(view)
	if !strings.Contains(plain, "WARN  run complete: 0 ok, 0 failed, 1 cancelled") {
		t.Fatalf("cancelled completion should render warning bar:\n%s", plain)
	}
}

func TestModelCancelsOnlySelectedRunningTarget(t *testing.T) {
	model := NewModel(Options{Command: "sleep 10", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: false},
	}})
	model.Running = true
	model.PendingRuns = 2
	model.Status["api"] = core.StatusRunning
	model.Status["web"] = core.StatusRunning
	apiCancelled := false
	webCancelled := false
	model.targetCancels = map[string]context.CancelFunc{
		"api": func() { apiCancelled = true },
		"web": func() { webCancelled = true },
	}

	model, _ = updateKey(model, "delete")
	if !apiCancelled {
		t.Fatal("selected api target should be cancelled")
	}
	if webCancelled {
		t.Fatal("unselected web target should keep running")
	}
	if model.Status["api"] != core.StatusCancelled || model.Status["web"] != core.StatusRunning {
		t.Fatalf("statuses = %#v", model.Status)
	}

	updated, _ := model.Update(runDoneMsg{targetID: "api", results: []core.RunResult{{Target: model.Targets[0], Status: core.StatusCancelled}}})
	model = updated.(Model)
	updated, _ = model.Update(runDoneMsg{targetID: "web", results: []core.RunResult{{Target: model.Targets[1], Status: core.StatusSucceeded}}})
	model = updated.(Model)
	if model.Running {
		t.Fatal("model should stop after remaining target completes")
	}
	if model.Status["web"] != core.StatusSucceeded {
		t.Fatalf("web status = %s", model.Status["web"])
	}
}

func TestModelHonorsSerialMode(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Mode: core.ModeSerial, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		return []core.RunResult{{Target: req.Targets[0], Status: core.StatusSucceeded}}, nil
	}

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	if model.Status["api"] != core.StatusRunning || model.Status["web"] != core.StatusQueued {
		t.Fatalf("serial initial statuses = %#v", model.Status)
	}
	model, next := applyOneCmd(t, model, cmd)
	if model.Status["api"] != core.StatusSucceeded || model.Status["web"] != core.StatusRunning {
		t.Fatalf("serial after first completion statuses = %#v", model.Status)
	}
	if next == nil {
		t.Fatal("serial should schedule next target")
	}
	model, next = applyOneCmd(t, model, next)
	if next != nil {
		t.Fatal("serial should have no more commands")
	}
	if model.Running {
		t.Fatal("serial run should be complete")
	}
	if model.Status["web"] != core.StatusSucceeded {
		t.Fatalf("web status = %s", model.Status["web"])
	}
}

func TestModelHonorsWorkerLimit(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Workers: 2, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	if model.Status["api"] != core.StatusRunning || model.Status["web"] != core.StatusRunning || model.Status["worker"] != core.StatusQueued {
		t.Fatalf("worker-limited statuses = %#v", model.Status)
	}
}

func TestModelCancelsSelectedQueuedTargets(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Workers: 1, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	model.Targets[0].Selected = false

	model, _ = updateKey(model, "delete")
	if model.Status["api"] != core.StatusRunning {
		t.Fatalf("active unselected target should keep running: %#v", model.Status)
	}
	if model.Status["web"] != core.StatusCancelled || model.Status["worker"] != core.StatusCancelled {
		t.Fatalf("queued selected targets should be cancelled: %#v", model.Status)
	}
	if model.PendingRuns != 1 {
		t.Fatalf("pending runs = %d, want only active target", model.PendingRuns)
	}
	if len(model.runQueue) != 0 {
		t.Fatalf("queue should be empty after cancelling queued targets: %#v", model.runQueue)
	}

	updatedModel, next := model.Update(runDoneMsg{targetID: "api", results: []core.RunResult{{Target: model.Targets[0], Status: core.StatusSucceeded}}})
	model = updatedModel.(Model)
	if next != nil {
		t.Fatal("no follow-up command expected after queue was cancelled")
	}
	if model.Running {
		t.Fatal("model should stop after active target completes")
	}
	if len(model.completedResults) != 3 {
		t.Fatalf("completed results = %#v", model.completedResults)
	}
}

func TestCancelAllClearsQueuedWork(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Workers: 1, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
		{ID: "worker", RelPath: "worker", Selected: true},
	}})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	cancelled := false
	model.cancelRun = func() { cancelled = true }

	model.cancelAll()
	if !cancelled {
		t.Fatal("cancelAll should call root cancel function")
	}
	if len(model.runQueue) != 0 {
		t.Fatalf("run queue should be cleared: %#v", model.runQueue)
	}
	if model.PendingRuns != 1 {
		t.Fatalf("pending runs = %d, want active target only", model.PendingRuns)
	}
	for _, id := range []string{"api", "web", "worker"} {
		if model.Status[id] != core.StatusCancelled {
			t.Fatalf("%s status = %s, want cancelled", id, model.Status[id])
		}
	}

	updatedModel, next := model.Update(runDoneMsg{targetID: "api", results: []core.RunResult{{Target: model.Targets[0], Status: core.StatusCancelled}}})
	model = updatedModel.(Model)
	if next != nil {
		t.Fatal("cancelAll should not schedule queued work after active target completes")
	}
	if model.Running {
		t.Fatal("model should stop after active cancellation completes")
	}
	if len(model.completedResults) != 3 {
		t.Fatalf("completed results = %#v, want active plus queued cancellations", model.completedResults)
	}
}

func TestCtrlCCancelsActiveWorkAndQuits(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Workers: 1, Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
		{ID: "web", RelPath: "web", Selected: true},
	}})
	updated, _ := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	cancelled := false
	model.cancelRun = func() { cancelled = true }

	updated, cmd := updateKey(model, "ctrl+c")
	model = updated

	if !cancelled {
		t.Fatal("ctrl+c should cancel the root run context")
	}
	if model.Status["api"] != core.StatusCancelled || model.Status["web"] != core.StatusCancelled {
		t.Fatalf("statuses = %#v, want all active work cancelled", model.Status)
	}
	if len(model.runQueue) != 0 {
		t.Fatalf("run queue should be cleared: %#v", model.runQueue)
	}
	if cmd == nil {
		t.Fatal("ctrl+c should return tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c command = %T, want tea.QuitMsg", cmd())
	}
}

func TestModelDeduplicatesCompletedResults(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{
		{ID: "api", RelPath: "api", Selected: true},
	}})
	model.Running = true
	model.PendingRuns = 2
	model.recordCompletedResult(core.RunResult{Target: model.Targets[0], Status: core.StatusQueued})

	updated, _ := model.Update(runDoneMsg{targetID: "api", results: []core.RunResult{{Target: model.Targets[0], Status: core.StatusSucceeded}}})
	model = updated.(Model)
	if len(model.completedResults) != 1 {
		t.Fatalf("completed results = %#v, want one result per target", model.completedResults)
	}
	if model.completedResults[0].Status != core.StatusSucceeded {
		t.Fatalf("status = %s, want latest result", model.completedResults[0].Status)
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
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		if len(req.Targets) != 1 || req.Targets[0].ID != "web" {
			t.Fatalf("rerun targets = %#v", req.Targets)
		}
		return []core.RunResult{{Target: req.Targets[0], Status: core.StatusSucceeded, Output: "fixed\n"}}, nil
	}
	updated, cmd := updateKey(model, "y")
	model = updated
	if cmd == nil {
		t.Fatal("confirm should start rerun")
	}
	model = applyCmd(t, model, cmd)
	if model.Status["web"] != core.StatusSucceeded {
		t.Fatalf("web status = %s", model.Status["web"])
	}
}

func TestModelPersistsHistory(t *testing.T) {
	tmp := t.TempDir()
	commandHistory := filepath.Join(tmp, "home-history.jsonl")
	runHistory := filepath.Join(tmp, "project-history.jsonl")
	model := NewModel(Options{
		Command:            "echo ok",
		CommandHistoryPath: commandHistory,
		RunHistoryPath:     runHistory,
		Targets: []core.Target{
			{ID: "api", RelPath: "api", Selected: true},
			{ID: "web", RelPath: "web", Selected: true},
		},
	})
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		result := core.RunResult{Target: req.Targets[0], Status: core.StatusSucceeded, Output: req.Targets[0].ID + " ok\n"}
		if req.Targets[0].ID == "web" {
			result.Status = core.StatusFailed
			result.Error = "exit status 1"
		}
		return []core.RunResult{result}, nil
	}

	updated, cmd := updateSpecialKey(model, tea.KeyEnter)
	model = updated
	model = applyCmd(t, model, cmd)
	if model.RunError != "" {
		t.Fatalf("run error = %q", model.RunError)
	}

	commands, err := history.ReadCommands(commandHistory)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Command != "echo ok" {
		t.Fatalf("commands = %#v", commands)
	}
	runs, err := history.ReadRuns(runHistory)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].Succeeded != 1 || runs[0].Failed != 1 || runs[0].Total != 2 {
		t.Fatalf("runs = %#v", runs)
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
	for _, want := range []string{"Project runs (1/1)", "when", "go test"} {
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
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.HistoryPos != 1 {
		t.Fatalf("history pos = %d, want first project run", model.HistoryPos)
	}
	view := strings.Join(model.historyRows(), "\n")
	if !strings.Contains(stripANSI(view), "›") || !strings.Contains(view, "\x1b[") {
		t.Fatalf("selected project run should be highlighted:\n%s", view)
	}
	model, _ = updateSpecialKey(model, tea.KeyEnter)
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
	view := strings.Join(model.historyRows(), "\n")
	plain := stripANSI(view)
	for _, want := range []string{"/ pt", "Commands (1/1)", "pnpm lint", "Project runs (1/1)", "pnpm test"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("history search should contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "terraform plan") || strings.Contains(plain, "docker build") {
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
	if commands := model.visibleHistoryCommands(); len(commands) != 0 {
		t.Fatalf("exact history search should not fuzzy match commands: %#v", commands)
	}
	if runs := model.visibleRunHistory(); len(runs) != 0 {
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
	if model.RunError != "no history command matches" {
		t.Fatalf("run error = %q", model.RunError)
	}
	view := stripANSI(model.View().Content)
	for _, want := range []string{"History", "No command matches.", "No project runs match.", "ERROR  no history command matches"} {
		if !strings.Contains(view, want) {
			t.Fatalf("history no-match view should contain %q:\n%s", want, view)
		}
	}

	model, _ = updateSpecialKey(model, tea.KeyBackspace)
	if model.RunError != "" {
		t.Fatalf("editing history search should clear no-match error, got %q", model.RunError)
	}
}

func TestHistoryRowsAreScannableAndHighlightSelection(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"go test ./...", "pnpm test"}
	model.HistoryPos = 1
	model.RunHistory = []history.RunEntry{
		{Command: "go test ./...", Total: 3, Succeeded: 2, Failed: 1},
		{Command: "pnpm test", Total: 2, Succeeded: 2},
		{Command: "sleep 10", Total: 1, Cancelled: 1},
	}
	rows := strings.Join(model.historyRows(), "\n")
	plain := stripANSI(rows)
	for _, want := range []string{"Commands (2/2)", "#  command", "› 2", "Project runs (3/3)", "result", "failed", "ok", "cancelled"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("history rows should contain %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(rows, "\x1b[") {
		t.Fatalf("selected history row and outcomes should be styled:\n%s", rows)
	}
}

func TestHistoryRowsShowVisibleTotalsWhenTruncated(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	model.History = []string{"cmd-1", "cmd-2", "cmd-3", "cmd-4", "cmd-5", "cmd-6", "cmd-7"}
	model.RunHistory = []history.RunEntry{
		{Command: "run-1", Total: 1},
		{Command: "run-2", Total: 1},
		{Command: "run-3", Total: 1},
		{Command: "run-4", Total: 1},
		{Command: "run-5", Total: 1},
		{Command: "run-6", Total: 1},
	}
	rows := stripANSI(strings.Join(model.historyRows(), "\n"))
	for _, want := range []string{"Commands (6/7)", "... 1 more command(s)", "Project runs (5/6)", "... 1 more project run(s)"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("history rows should show visible total %q:\n%s", want, rows)
		}
	}
}

func TestHistoryRowsEmptyFooterDoesNotPromiseReuse(t *testing.T) {
	model := NewModel(Options{Command: "echo ok", Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	rows := stripANSI(strings.Join(model.historyRows(), "\n"))
	for _, want := range []string{"No command history yet.", "No project runs yet.", "no command to reuse"} {
		if !strings.Contains(rows, want) {
			t.Fatalf("empty history should contain %q:\n%s", want, rows)
		}
	}
	if strings.Contains(rows, "enter reuse selected command") {
		t.Fatalf("empty history should not promise command reuse:\n%s", rows)
	}

	model.History = []string{"go test"}
	model.RunHistory = []history.RunEntry{{Command: "go test", Total: 1}}
	model.HistoryFilter = "zzzz"
	rows = stripANSI(strings.Join(model.historyRows(), "\n"))
	if !strings.Contains(rows, "no command to reuse") {
		t.Fatalf("no-match history should not promise command reuse:\n%s", rows)
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
	model, _ = updateKey(model, ":")
	model = typeText(model, command)
	return updateSpecialKey(model, tea.KeyEnter)
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
