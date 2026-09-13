package tui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
	runpkg "github.com/theopoc/runny/internal/run"
)

func changeModel() Model {
	m := NewModel(Options{Command: "terragrunt run --all -- plan", Targets: []core.Target{
		{ID: "production", RelPath: "production", Selected: true},
		{ID: "staging", RelPath: "staging"},
		{ID: "failed", RelPath: "failed"},
		{ID: "unknown", RelPath: "unknown"},
	}})
	m.Changes["production"] = core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: 3, Change: 2, Destroy: 1}
	m.Changes["staging"] = core.ChangeSummary{Detected: true, Known: true, Phase: "apply"}
	m.Status["production"], m.Status["staging"] = core.StatusSucceeded, core.StatusSucceeded
	m.Status["failed"], m.Status["unknown"] = core.StatusFailed, core.StatusSucceeded
	m.Width, m.Height = 120, 30
	return m
}

func TestChangesLayoutWidthsAndGolden(t *testing.T) {
	for _, width := range []int{60, 80, 100, 120, 160} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := changeModel()
			m.Width = width
			rows := m.renderDirectoryPanel(m.directoryContentWidth()+4, 16)
			output := stripANSI(strings.Join(rows, "\n"))
			for _, want := range []string{"CHANGES", "P +3 ~2 -1", "A +0 ~0 -0", "—"} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q:\n%s", want, output)
				}
			}
			if maxLineWidth(output) > m.directoryContentWidth()+4 {
				t.Fatal("overflow")
			}
			if strings.Contains(output, "unit") {
				t.Fatal("units added to Tasks")
			}
			path := filepath.Join("testdata", fmt.Sprintf("TestChangesLayout_%d.golden", width))
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.WriteFile(path, []byte(output+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(want)) != strings.TrimSpace(output) {
				t.Fatalf("golden mismatch:\n%s", output)
			}
		})
	}
}

func TestLargeChangesWrapAndMouseRows(t *testing.T) {
	m := changeModel()
	m.Width, m.Height = 100, 30
	m.Changes["production"] = core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: math.MaxInt64, Change: math.MaxInt64, Destroy: math.MaxInt64}
	m.Cursor = 1
	m.ensureDirectoryOffset()
	rows := m.renderDirectoryPanel(m.directoryContentWidth()+4, 20)
	output := strings.Join(rows, "\n")
	for _, token := range []string{"+9223372036854775807", "~9223372036854775807", "-9223372036854775807"} {
		if !strings.Contains(stripANSI(output), token) {
			t.Fatalf("truncated %s:\n%s", token, stripANSI(output))
		}
	}
	if maxLineWidth(output) > m.directoryContentWidth()+4 {
		t.Fatal("large value overflow")
	}
	panelTop := strings.Count(m.renderPanelPrefix(m.Width), "\n")
	height := m.changeRowHeight(m.directoryContentWidth())
	for continuation := 0; continuation < height; continuation++ {
		id, ok := m.directoryTargetAt(2, panelTop+3+height+continuation)
		if !ok || id != 1 {
			t.Fatalf("continuation mouse selected %d %v", id, ok)
		}
	}
}

func TestChangesResetOnRerunAndStayOutOfOutput(t *testing.T) {
	m := changeModel()
	m.startLifecycle = fakeStart(&fakeActiveRun{}, nil)
	m.Logs["production"] = "unchanged output"
	next, _ := m.beginRun("tofu plan", []core.Target{m.Targets[0]})
	m = next.(Model)
	if m.Changes["production"].Known {
		t.Fatal("stale counts on rerun")
	}
	m.applyTargetSnapshot(runpkg.TargetSnapshot{Target: m.Targets[0], Status: core.StatusSucceeded, OutputTail: "original output", Changes: core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: 2}})
	if m.Logs["production"] != "original output" {
		t.Fatal("counter text leaked into Output")
	}
}

func TestHistoryChangesAndLegacy(t *testing.T) {
	m := changeModel()
	m.RunHistory = []history.RunEntry{{Command: m.Command, Total: 4, Targets: []history.TargetEntry{
		{ID: "production", RelPath: "production", Status: core.StatusSucceeded, ExitCode: 2, Changes: m.Changes["production"]},
		{ID: "old", RelPath: "old", Status: core.StatusSucceeded},
	}}}
	m.HistoryShowAll = true
	m.HistoryDepth = historyDepthTargets
	for _, width := range []int{40, 60, 100} {
		output := stripANSI(strings.Join(m.historyDiagnosticAllRows(width, 24), "\n"))
		if !strings.Contains(output, "P +3 ~2 -1") || !strings.Contains(output, "CHANGES") || !strings.Contains(output, "—") {
			t.Fatalf("%d:\n%s", width, output)
		}
		if strings.Contains(output, "unit") || maxLineWidth(output) > width {
			t.Fatalf("bad history layout:\n%s", output)
		}
	}
}

func TestChangesUseForegroundOnlyOutsideSelection(t *testing.T) {
	m := changeModel()
	m.Cursor = -1
	for i := range m.Targets {
		m.Targets[i].Selected = false
	}
	for _, width := range []int{30, 46, 100} {
		for i, target := range m.Targets {
			row := m.renderTargetRow(i, target, width)
			if containsANSIBackground(row) {
				t.Fatalf("counter row paints background: %q", row)
			}
		}
	}
}

func TestChangesKeepCursorVisibleWhenRowHeightChanges(t *testing.T) {
	m := changeModel()
	m.Width, m.Height = 80, 12
	m.Cursor = len(m.Targets) - 1
	m.ensureDirectoryOffset()
	m.applyTargetSnapshot(runpkg.TargetSnapshot{Target: m.Targets[0], Status: core.StatusSucceeded,
		Changes: core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: math.MaxInt64, Change: math.MaxInt64, Destroy: math.MaxInt64}})
	if m.Cursor < m.DirectoryOffset || m.Cursor >= m.DirectoryOffset+m.directoryViewportRows() {
		t.Fatalf("cursor hidden after wrap: cursor=%d offset=%d rows=%d", m.Cursor, m.DirectoryOffset, m.directoryViewportRows())
	}
	m.startLifecycle = fakeStart(&fakeActiveRun{}, nil)
	next, _ := m.beginRun("tofu plan", m.Targets)
	m = next.(Model)
	wantOffset := max(0, len(m.Targets)-m.directoryViewportRows())
	if m.DirectoryOffset > wantOffset {
		t.Fatalf("offset not clamped after rerun: %d > %d", m.DirectoryOffset, wantOffset)
	}
}

func TestHistoryChangesDefaultResetsForLegacyRun(t *testing.T) {
	m := changeModel()
	m.HistoryTab = historyTabRuns
	m.RunHistory = []history.RunEntry{
		{Command: "tofu plan", Targets: []history.TargetEntry{{ID: "production", Changes: m.Changes["production"]}}},
		{Command: "echo legacy", Targets: []history.TargetEntry{{ID: "old", Status: core.StatusSucceeded}}},
	}
	next, _ := m.activateHistorySelection()
	m = next.(Model)
	if !m.HistoryShowAll {
		t.Fatal("summary run should start with all targets")
	}
	m.HistoryDepth, m.HistoryPos = historyDepthRuns, 1
	next, _ = m.activateHistorySelection()
	m = next.(Model)
	if m.HistoryShowAll {
		t.Fatal("summary view leaked into legacy run")
	}
}

func TestChangesKeepCursorVisibleAfterResize(t *testing.T) {
	m := changeModel()
	m.Width, m.Height = 300, 12
	m.Cursor = len(m.Targets) - 1
	m.Changes["production"] = core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: math.MaxInt64, Change: math.MaxInt64, Destroy: math.MaxInt64}
	m.ensureDirectoryOffset()
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	m = next.(Model)
	if m.Cursor < m.DirectoryOffset || m.Cursor >= m.DirectoryOffset+m.directoryViewportRows() {
		t.Fatalf("resize hides cursor: cursor=%d offset=%d rows=%d", m.Cursor, m.DirectoryOffset, m.directoryViewportRows())
	}
}

func TestChangesKeepCursorVisibleAfterOverlayResize(t *testing.T) {
	targets := make([]core.Target, 13)
	for i := range targets {
		targets[i] = core.Target{ID: fmt.Sprint(i), RelPath: fmt.Sprintf("stack-%02d", i)}
	}
	m := NewModel(Options{Targets: targets})
	m.Width, m.Height, m.Cursor = 120, 30, 12
	m.Changes["0"] = core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: 1}
	m.Status["0"] = core.StatusSucceeded
	m.ensureDirectoryOffset()
	m.ShowHistory = true
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	m = next.(Model)
	m.ShowHistory = false
	if m.Cursor < m.DirectoryOffset || m.Cursor >= m.DirectoryOffset+m.directoryViewportRows() {
		t.Fatalf("overlay closure hides cursor: cursor=%d offset=%d rows=%d", m.Cursor, m.DirectoryOffset, m.directoryViewportRows())
	}
}
