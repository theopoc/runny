package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
)

func TestFilterInputSelectsCopiesCutsAndPastes(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model, _ = updateKey(model, "/")
	model = typeText(model, "api-old")
	for range 3 {
		model, _ = updateModifiedKey(model, tea.KeyLeft, tea.ModShift)
	}
	if got := model.selectedFilterText(); got != "old" {
		t.Fatalf("selected filter text = %q, want old", got)
	}
	var copyCmd tea.Cmd
	model, copyCmd = updateModifiedKey(model, 'c', tea.ModCtrl)
	if copyCmd == nil {
		t.Fatal("ctrl+c with filter selection should write clipboard")
	}
	model, _ = updateModifiedKey(model, 'x', tea.ModCtrl)
	if model.Filter != "api-" {
		t.Fatalf("filter after cut = %q, want api-", model.Filter)
	}
	model, _ = updatePaste(model, "new")
	if model.Filter != "api-new" {
		t.Fatalf("filter after paste = %q, want api-new", model.Filter)
	}
	model, _ = updateModifiedKey(model, 'c', tea.ModCtrl)
	if !model.ConfirmQuit {
		t.Fatal("ctrl+c without filter selection should keep quit confirmation behavior")
	}
}

func TestFilterInputSelectionUsesBackground(t *testing.T) {
	if os.Getenv("RUNNY_FILTER_SELECTION_COLOR_TEST") != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestFilterInputSelectionUsesBackground$")
		for _, entry := range os.Environ() {
			if !strings.HasPrefix(entry, "NO_COLOR=") {
				cmd.Env = append(cmd.Env, entry)
			}
		}
		cmd.Env = append(cmd.Env, "RUNNY_FILTER_SELECTION_COLOR_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("color subprocess: %v\n%s", err, output)
		}
		return
	}

	profile := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = profile })
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "api-old"
	model.openFilterEditor()
	for range 3 {
		model, _ = updateModifiedKey(model, tea.KeyLeft, tea.ModShift)
	}
	if rendered := strings.Join(model.commandInputBoxLines(40), "\n"); !containsANSIBackground(rendered) {
		t.Fatalf("selected filter text should render selection background: %q", rendered)
	}
}

func TestLineEditorsMoveAndDeleteWholeGraphemeClusters(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(Model) Model
		value func(Model) string
	}{
		{
			name: "command",
			setup: func(model Model) Model {
				model.Command = "a👩‍💻b"
				model.openCommandOverlay()
				return model
			},
			value: func(model Model) string { return model.Command },
		},
		{
			name: "filter",
			setup: func(model Model) Model {
				model.Filter = "a👩‍💻b"
				model.openFilterEditor()
				return model
			},
			value: func(model Model) string { return model.Filter },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			model := test.setup(NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}}))
			model, _ = updateSpecialKey(model, tea.KeyHome)
			model, _ = updateSpecialKey(model, tea.KeyRight)
			model, _ = updateSpecialKey(model, tea.KeyDelete)
			if got := test.value(model); got != "ab" {
				t.Fatalf("value after grapheme delete = %q, want ab", got)
			}
		})
	}
}

func TestLineEditorsSupportShiftHomeAndEnd(t *testing.T) {
	command := NewModel(Options{Command: "alpha beta"})
	command.openCommandOverlay()
	command, _ = updateModifiedKey(command, tea.KeyHome, tea.ModShift)
	if got := command.selectedCommandText(); got != "alpha beta" {
		t.Fatalf("shift+home command selection = %q", got)
	}

	filter := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	filter.Filter = "alpha beta"
	filter.openFilterEditor()
	filter, _ = updateModifiedKey(filter, tea.KeyHome, tea.ModShift)
	if got := filter.selectedFilterText(); got != "alpha beta" {
		t.Fatalf("shift+home filter selection = %q", got)
	}
	filter, _ = updateSpecialKey(filter, tea.KeyHome)
	filter, _ = updateModifiedKey(filter, tea.KeyEnd, tea.ModShift)
	if got := filter.selectedFilterText(); got != "alpha beta" {
		t.Fatalf("shift+end filter selection = %q", got)
	}
}

func TestFilterInputMovesByWord(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "alpha beta"
	model.openFilterEditor()
	model, _ = updateModifiedKey(model, tea.KeyLeft, tea.ModAlt)
	model, _ = updateKey(model, "X")
	if model.Filter != "alpha Xbeta" {
		t.Fatalf("filter after alt+left insertion = %q", model.Filter)
	}
}

func TestFilterHistoryWithoutStorageIsUniqueMRUAndRestoresDraft(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model = enterAndCommitFilter(t, model, "api")
	model = tabAndCommitFilter(t, model, "web")
	model = enterAndCommitFilter(t, model, "api")
	if got, want := strings.Join(model.filterHistory, ","), "api,web"; got != want {
		t.Fatalf("filter history = %q, want %q", got, want)
	}
	if fresh := NewModel(Options{}); len(fresh.filterHistory) != 0 {
		t.Fatalf("fresh process model inherited filter history: %#v", fresh.filterHistory)
	}

	model.openFilterEditor()
	model, _ = updateModifiedKey(model, 'u', tea.ModCtrl)
	model = typeText(model, "draft")
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Filter != "api" {
		t.Fatalf("first history item = %q, want api", model.Filter)
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Filter != "web" {
		t.Fatalf("older history item = %q, want web", model.Filter)
	}
	model, _ = updateSpecialKey(model, tea.KeyDown)
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Filter != "draft" {
		t.Fatalf("restored filter draft = %q, want draft", model.Filter)
	}
}

func TestFilterHistoryPersistsAcceptedFiltersAndRestoresDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.jsonl")
	opts := Options{FilterHistoryPath: path}
	model := NewModel(opts)
	for _, value := range []string{"api", "API", " api ", "re:^web$", "api"} {
		model.openFilterEditor()
		model, _ = updateModifiedKey(model, 'u', tea.ModCtrl)
		model = typeText(model, value)
		var save tea.Cmd
		model, save = updateSpecialKey(model, tea.KeyEnter)
		if save == nil {
			t.Fatal("accepting filter should schedule persistence")
		}
		updated, _ := model.Update(save())
		model = updated.(Model)
	}
	want := []string{"api", "re:^web$", " api ", "API"}
	model = NewModel(opts)
	if !reflect.DeepEqual(model.filterHistory, want) || model.Filter != "" {
		t.Fatalf("reloaded history/filter = %q/%q, want %q/empty", model.filterHistory, model.Filter, want)
	}
	model.openFilterEditor()
	model = typeText(model, "draft")
	for _, value := range want {
		model, _ = updateSpecialKey(model, tea.KeyUp)
		if model.Filter != value {
			t.Fatalf("recalled filter = %q, want %q", model.Filter, value)
		}
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	if model.Filter != want[len(want)-1] {
		t.Fatal("up past oldest should stop at oldest")
	}
	for range len(want) + 1 {
		model, _ = updateSpecialKey(model, tea.KeyDown)
	}
	if model.Filter != "draft" {
		t.Fatalf("restored draft = %q", model.Filter)
	}
	model, _ = updateSpecialKey(model, tea.KeyUp)
	model = typeText(model, "x")
	model, _ = updateSpecialKey(model, tea.KeyDown)
	if model.Filter != "apix" {
		t.Fatalf("editing recalled filter should reset navigation, got %q", model.Filter)
	}
}

func TestFilterHistoryDoesNotPersistCancelledOrInvalidFilters(t *testing.T) {
	for _, value := range []string{"", "re:[", "re:", "cancelled"} {
		t.Run(value, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "filters.jsonl")
			model := NewModel(Options{FilterHistoryPath: path})
			model.openFilterEditor()
			model = typeText(model, value)
			key := tea.KeyEnter
			if value == "cancelled" {
				key = tea.KeyEsc
			}
			_, save := updateSpecialKey(model, key)
			if save != nil {
				t.Fatal("empty, invalid or cancelled filter scheduled persistence")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("unaccepted filter created history: %v", err)
			}
		})
	}
}

func TestFilterHistoryPersistsOnEditorAcceptance(t *testing.T) {
	for _, name := range []string{"enter", "tab", "shift+tab", "click"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "filters.jsonl")
			model := NewModel(Options{FilterHistoryPath: path, Targets: []core.Target{{ID: "api", RelPath: "api"}}})
			model.Width, model.Height = 120, 26
			model.openFilterEditor()
			model = typeText(model, "api")
			var save tea.Cmd
			switch name {
			case "enter":
				model, save = updateSpecialKey(model, tea.KeyEnter)
			case "tab":
				model, save = updateSpecialKey(model, tea.KeyTab)
			case "shift+tab":
				model, save = updateModifiedKey(model, tea.KeyTab, tea.ModShift)
			case "click":
				panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")
				updated, cmd := model.Update(tea.MouseClickMsg{X: 2, Y: panelTop + 3, Button: tea.MouseLeft})
				model, save = updated.(Model), cmd
			}
			if save == nil || model.Focus != FocusTargets {
				t.Fatalf("acceptance did not schedule save and return to tasks: %v", model.Focus)
			}
			save()
			if reloaded := NewModel(Options{FilterHistoryPath: path}); !reflect.DeepEqual(reloaded.filterHistory, []string{"api"}) {
				t.Fatalf("reloaded history = %q", reloaded.filterHistory)
			}
		})
	}
}

func TestFilterHistoryPersistenceOrdersSnapshotsAndReportsFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.jsonl")
	model := NewModel(Options{FilterHistoryPath: path})
	model.Filter = "old"
	olderSave := model.commitFilterHistory()
	model.Filter = "new"
	newerSave := model.commitFilterHistory()
	newerSave()
	olderSave()
	got, err := history.ReadFilters(path)
	if err != nil || !reflect.DeepEqual(got, []string{"new", "old"}) {
		t.Fatalf("out of order saves = %q, %v", got, err)
	}

	model = NewModel(Options{FilterHistoryPath: t.TempDir()})
	if !strings.Contains(model.Notice, "read filter history:") {
		t.Fatalf("missing load failure notice: %q", model.Notice)
	}
	model.Filter = "api"
	save := model.commitFilterHistory()
	updated, _ := model.Update(save())
	model = updated.(Model)
	if !strings.Contains(model.Notice, "save filter history:") || model.filterHistory[0] != "api" {
		t.Fatalf("failed save should report error and keep session history: %q/%q", model.Notice, model.filterHistory)
	}
}

func TestFilterHistoryIgnoresEmptyAndInvalidFilters(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.openFilterEditor()
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	model.openFilterEditor()
	model = typeText(model, "re:[")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	model, _ = updateSpecialKey(model, tea.KeyTab)
	model.openFilterEditor()
	model, _ = updateModifiedKey(model, 'u', tea.ModCtrl)
	model = typeText(model, "re:")
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	if len(model.filterHistory) != 0 {
		t.Fatalf("filter history = %#v, want empty", model.filterHistory)
	}
}

func TestFilterHistoryKeepsFiftyNewestEntries(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	for i := range filterHistoryLimit + 1 {
		model = enterAndCommitFilter(t, model, "filter-"+strconv.Itoa(i))
	}
	if len(model.filterHistory) != filterHistoryLimit {
		t.Fatalf("filter history length = %d, want %d", len(model.filterHistory), filterHistoryLimit)
	}
	if model.filterHistory[0] != "filter-50" || model.filterHistory[len(model.filterHistory)-1] != "filter-1" {
		t.Fatalf("filter history bounds = %q..%q", model.filterHistory[0], model.filterHistory[len(model.filterHistory)-1])
	}
}

func TestFilterHistoryCommitsWhenTasksAreClicked(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Width = 120
	model.Height = 26
	model.openFilterEditor()
	model = typeText(model, "api")
	panelTop := strings.Count(model.renderPanelPrefix(model.Width), "\n")

	updated, _ := model.Update(tea.MouseClickMsg{X: 2, Y: panelTop + 3, Button: tea.MouseLeft})
	model = updated.(Model)
	if model.Focus != FocusTargets || len(model.filterHistory) != 1 || model.filterHistory[0] != "api" {
		t.Fatalf("click commit = focus %v history %#v", model.Focus, model.filterHistory)
	}
}

func TestFilterInputViewportFollowsCursorAndMarksHiddenText(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "prefix-0123456789-suffix"
	model.openFilterEditor()

	atEnd := stripANSI(strings.Join(model.commandInputBoxLines(18), "\n"))
	if !strings.Contains(atEnd, "‹") || strings.Contains(atEnd, "prefix") || !strings.Contains(atEnd, "suffix") {
		t.Fatalf("filter viewport at end should expose suffix and left marker:\n%s", atEnd)
	}
	for range 8 {
		model, _ = updateSpecialKey(model, tea.KeyLeft)
	}
	inMiddle := strings.Join(model.commandInputBoxLines(18), "\n")
	if plain := stripANSI(inMiddle); !strings.Contains(plain, "‹") || !strings.Contains(plain, "›") {
		t.Fatalf("middle viewport should mark both hidden sides:\n%s", plain)
	}
	for _, line := range strings.Split(inMiddle, "\n") {
		if width := ansi.StringWidth(line); width > 18 {
			t.Fatalf("filter viewport width = %d, want <= 18:\n%s", width, stripANSI(inMiddle))
		}
	}
}

func TestFilterInputCursorDoesNotInsertVisualCell(t *testing.T) {
	profile := lipgloss.Writer.Profile
	lipgloss.Writer.Profile = colorprofile.TrueColor
	t.Cleanup(func() { lipgloss.Writer.Profile = profile })

	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "api"
	model.openFilterEditor()
	model, _ = updateSpecialKey(model, tea.KeyLeft)

	rendered := model.renderFilterInputValue(20)
	if got := stripANSI(rendered); got != "api" {
		t.Fatalf("filter cursor inserted a visual cell: %q, want api", got)
	}
	if !containsANSIReverse(rendered) {
		t.Fatalf("filter cursor should reverse character under cursor: %q", rendered)
	}

	model.setFilterCursor(len(splitGraphemes(model.Filter)), false)
	rendered = model.renderFilterInputValue(20)
	if got := stripANSI(rendered); got != "api" || ansi.StringWidth(rendered) != 4 || !containsANSIReverse(rendered) {
		t.Fatalf("filter end cursor = %q, want reverse-video trailing cell", rendered)
	}
}

func containsANSIReverse(value string) bool {
	for _, match := range ansiSGRPattern.FindAllStringSubmatch(value, -1) {
		for _, parameter := range strings.Split(match[1], ";") {
			code, err := strconv.Atoi(parameter)
			if err == nil && code == 7 {
				return true
			}
		}
	}
	return false
}

func TestFilterInputEditingGolden(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api"}}})
	model.Filter = "prefix-0123456789-suffix"
	model.openFilterEditor()
	for range 14 {
		model, _ = updateSpecialKey(model, tea.KeyLeft)
	}
	for range 2 {
		model, _ = updateModifiedKey(model, tea.KeyRight, tea.ModShift)
	}

	got := stripANSI(strings.Join(model.commandInputBoxLines(24), "\n")) + "\n" + stripANSI(model.renderFooter(60))
	want, err := os.ReadFile("testdata/TestFilterInputEditingGolden.golden")
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.TrimRight(string(want), "\n") {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func enterAndCommitFilter(t *testing.T, model Model, value string) Model {
	t.Helper()
	model.openFilterEditor()
	model, _ = updateModifiedKey(model, 'u', tea.ModCtrl)
	model = typeText(model, value)
	model, _ = updateSpecialKey(model, tea.KeyEnter)
	return model
}

func tabAndCommitFilter(t *testing.T, model Model, value string) Model {
	t.Helper()
	model.openFilterEditor()
	model, _ = updateModifiedKey(model, 'u', tea.ModCtrl)
	model = typeText(model, value)
	model, _ = updateSpecialKey(model, tea.KeyTab)
	return model
}
