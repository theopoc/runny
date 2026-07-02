package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/saewyn/runny/internal/core"
	"github.com/saewyn/runny/internal/history"
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

func TestCommandFocusAcceptsSlashAndSpace(t *testing.T) {
	model := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
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
	if model.Focus != FocusCommand {
		t.Fatalf("focus = %v, want command", model.Focus)
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
	if !strings.Contains(view, "›") || !strings.Contains(view, "svc-j") {
		t.Fatalf("directory panel should scroll focused row into view:\n%s", view)
	}
	if strings.Contains(view, "svc-a") {
		t.Fatalf("directory panel should scroll past first rows:\n%s", view)
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
	if !strings.Contains(model.Logs["web"], "web bad") || !strings.Contains(model.Logs["web"], "exit status 1") {
		t.Fatalf("web logs = %q", model.Logs["web"])
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
	model.runFunc = func(ctx context.Context, req core.RunRequest) ([]core.RunResult, error) {
		if len(req.Targets) != 1 || req.Targets[0].ID != "web" {
			t.Fatalf("rerun targets = %#v", req.Targets)
		}
		return []core.RunResult{{Target: req.Targets[0], Status: core.StatusSucceeded, Output: "fixed\n"}}, nil
	}
	updated, cmd := updateSpecialKey(model, tea.KeyEnter)
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
	for _, want := range []string{"Project runs", "go test", "1 ok", "1 failed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("history overlay should contain %q:\n%s", want, view)
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
