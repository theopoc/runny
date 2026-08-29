package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
	"github.com/theopoc/runny/internal/logs"
)

type historyTab uint8

const (
	historyTabRuns historyTab = iota
	historyTabCommands
)

type historyDepth uint8

const (
	historyDepthRuns historyDepth = iota
	historyDepthTargets
	historyDepthLogs
)

func (m *Model) openHistory() {
	m.ShowHistory = true
	m.HistoryTab = historyTabRuns
	m.HistoryDepth = historyDepthRuns
	m.HistoryPos = 0
	m.HistoryTargetPos = 0
	m.HistoryShowAll = false
	m.HistoryDetailOffset = 0
}

func (m *Model) resetHistoryPositions() {
	m.HistoryPos = 0
	m.HistoryCommandPos = 0
	m.HistoryTargetPos = 0
	m.HistoryDetailOffset = 0
}

func (m Model) hasHistorySelection() bool {
	if m.HistoryTab == historyTabCommands {
		return len(m.filteredHistoryCommands()) > 0
	}
	if m.HistoryDepth == historyDepthTargets {
		return len(m.visibleHistoryTargets()) > 0
	}
	return len(m.filteredHistoryRuns()) > 0
}

func (m *Model) moveHistorySelection(delta int) {
	switch {
	case m.HistoryTab == historyTabCommands:
		m.HistoryCommandPos = clampHistoryIndex(m.HistoryCommandPos+delta, len(m.filteredHistoryCommands()))
	case m.HistoryDepth == historyDepthTargets:
		m.HistoryTargetPos = clampHistoryIndex(m.HistoryTargetPos+delta, len(m.visibleHistoryTargets()))
		m.HistoryDetailOffset = 0
	case m.HistoryDepth == historyDepthLogs:
		m.syncHistoryLogViewport()
		if delta < 0 {
			m.historyLogViewport.ScrollUp(-delta)
		} else {
			m.historyLogViewport.ScrollDown(delta)
		}
	case m.HistoryDepth == historyDepthRuns:
		m.HistoryPos = clampHistoryIndex(m.HistoryPos+delta, len(m.filteredHistoryRuns()))
		m.HistoryTargetPos = 0
	}
}

func (m Model) activateHistorySelection() (tea.Model, tea.Cmd) {
	if m.HistoryTab == historyTabCommands {
		commands := m.filteredHistoryCommands()
		if len(commands) == 0 {
			m.RunError = "no history command matches"
			return m, nil
		}
		m.Command = commands[clampHistoryIndex(m.HistoryCommandPos, len(commands))]
		m.moveCommandCursorToEnd()
		m.RunError = ""
		m.ShowHistory = false
		m.openCommandOverlay()
		return m, nil
	}
	if _, ok := m.selectedHistoryRun(); !ok {
		m.RunError = "no project run matches"
		return m, nil
	}
	switch m.HistoryDepth {
	case historyDepthRuns:
		m.HistoryDepth = historyDepthTargets
		m.HistoryTargetPos = 0
		m.HistoryDetailOffset = 0
	case historyDepthTargets:
		return m.openSelectedHistoryLog()
	}
	return m, nil
}

func (m Model) openSelectedHistoryLog() (tea.Model, tea.Cmd) {
	run, runOK := m.selectedHistoryRun()
	target, targetOK := m.selectedHistoryTarget()
	if !runOK || !targetOK {
		m.RunError = "no historical target selected"
		return m, nil
	}
	m.HistoryDepth = historyDepthLogs
	m.HistoryLog = ""
	m.HistoryLogError = ""
	m.HistoryLogLoading = false
	m.historyLogViewport.SetContentLines(nil)
	m.historyLogViewport.SetYOffset(0)
	m.historyLogRunID = run.LogID
	m.historyLogTargetID = target.ID
	if run.LogID == "" {
		m.HistoryLogError = "logs unavailable; run with --save-logs to retain output"
		return m, nil
	}
	m.HistoryLogLoading = true
	root := m.LogRoot
	runID := run.LogID
	targetID := target.ID
	return m, func() tea.Msg {
		content, err := logs.ReadPersisted(root, runID, targetID)
		return historyLogLoadedMsg{runID: runID, targetID: targetID, content: content, err: err}
	}
}

func (m *Model) applyHistoryLogLoaded(loaded historyLogLoadedMsg) {
	if loaded.runID != m.historyLogRunID || loaded.targetID != m.historyLogTargetID {
		return
	}
	m.HistoryLogLoading = false
	if loaded.err != nil {
		m.HistoryLog = ""
		m.HistoryLogError = loaded.err.Error()
		return
	}
	m.HistoryLog = loaded.content
	m.HistoryLogError = ""
	m.syncHistoryLogViewport()
}

func (m Model) reuseSelectedHistoryRun() (tea.Model, tea.Cmd) {
	run, ok := m.selectedHistoryRun()
	if !ok {
		m.RunError = "no project run matches"
		return m, nil
	}
	m.Command = run.Command
	m.moveCommandCursorToEnd()
	m.RunError = ""
	m.ShowHistory = false
	m.openCommandOverlay()
	return m, nil
}

func (m *Model) prepareHistoricalRerun() {
	if m.hasActiveRuns() {
		m.Notice = "finish or cancel active run before rerun failed"
		return
	}
	run, ok := m.selectedHistoryRun()
	if !ok {
		m.Notice = "no project run selected"
		return
	}
	failed := make(map[string]bool)
	for _, target := range run.Targets {
		if target.Status == core.StatusFailed {
			failed[target.ID] = true
		}
	}
	targets := make([]core.Target, 0, len(failed))
	for _, target := range m.Targets {
		if failed[target.ID] {
			target.Selected = true
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		m.Notice = "no historical failed targets exist in current project"
		return
	}
	m.confirmRunCommand = run.Command
	m.confirmRunTargets = targets
	m.ShowHistory = false
	m.ConfirmRun = true
}

func (m Model) confirmedRunTargetCount() int {
	if len(m.confirmRunTargets) > 0 {
		return len(m.confirmRunTargets)
	}
	return m.failedCount()
}

func (m Model) confirmedRunCommand() string {
	if m.confirmRunCommand != "" {
		return m.confirmRunCommand
	}
	return m.previewCommandText()
}

func (m Model) confirmedRunTargetSummary(width int) string {
	if len(m.confirmRunTargets) == 0 {
		return m.statusTargetSummary(core.StatusFailed, width)
	}
	paths := make([]string, 0, len(m.confirmRunTargets))
	for _, target := range m.confirmRunTargets {
		paths = append(paths, target.RelPath)
	}
	return truncateVisible(strings.Join(paths, ", "), width)
}

func (m Model) historyFooterKeys() ([]string, []string) {
	global := []string{"[ ] tabs", "? keymap"}
	if m.HistoryTab == historyTabCommands {
		context := []string{"/ search", "enter reuse", "up/down choose", "ctrl+u clear", "esc close"}
		if len(m.filteredHistoryCommands()) == 0 {
			context = []string{"/ search", "no command", "ctrl+u clear", "esc close"}
		}
		return global, context
	}
	switch m.HistoryDepth {
	case historyDepthTargets:
		toggle := "a show all"
		if m.HistoryShowAll {
			toggle = "a failures"
		}
		context := []string{"enter logs", "up/down target", "pgup/pgdn details", toggle, "r reuse", "esc back"}
		if len(m.visibleHistoryTargets()) == 0 {
			context = []string{toggle, "r reuse", "esc back"}
		}
		return global, context
	case historyDepthLogs:
		return global, []string{"up/down scroll", "r reuse", "esc back"}
	default:
		context := []string{"enter inspect", "R rerun failed", "r reuse", "/ search", "esc close"}
		if len(m.filteredHistoryRuns()) == 0 {
			context = []string{"/ search", "no run", "ctrl+u clear", "esc close"}
		}
		return global, context
	}
}

func (m *Model) clearConfirmedRun() {
	m.confirmRunCommand = ""
	m.confirmRunTargets = nil
}

func (m Model) renderHistoryOverlay(width, height int) string {
	boxWidth := width - 4
	if width >= 120 {
		boxWidth = min(boxWidth, 136)
	}
	boxWidth = max(52, min(width, boxWidth))
	boxHeight := max(3, height)
	contentWidth := max(1, boxWidth-4)
	contentHeight := max(1, boxHeight-2)
	rows := m.historyOverlayRows(contentWidth, contentHeight)
	for len(rows) < contentHeight {
		rows = append(rows, "")
	}
	for i := range rows {
		rows[i] = renderHistoryRow(rows[i], contentWidth)
	}
	return strings.Join(boxLines(boxWidth, boxHeight, "History", rows, false), "\n")
}

func renderHistoryRow(row string, width int) string {
	return padRightANSI(row, width)
}

func (m Model) historyOverlayRows(width, height int) []string {
	if height <= 0 {
		return nil
	}
	rows := []string{m.historyTabRow(width)}
	bodyHeight := height - 1
	if bodyHeight <= 0 {
		return rows
	}
	if m.HistoryTab == historyTabCommands {
		return append(rows, m.historyCommandListRows(width, bodyHeight)...)
	}
	if width >= 112 {
		divider := subtleStyle.Render(" │ ")
		leftWidth := (width - lipgloss.Width(divider)) * 46 / 100
		rightWidth := width - lipgloss.Width(divider) - leftWidth
		left := m.historyRunListRows(leftWidth, bodyHeight)
		right := m.historyDiagnosticRows(rightWidth, bodyHeight)
		if m.HistoryDepth == historyDepthLogs {
			right = m.historyLogRows(rightWidth, bodyHeight)
		}
		for i := 0; i < bodyHeight; i++ {
			leftRow := ""
			if i < len(left) {
				leftRow = left[i]
			}
			rightRow := ""
			if i < len(right) {
				rightRow = right[i]
			}
			rows = append(rows, padRightANSI(leftRow, leftWidth)+divider+padRightANSI(rightRow, rightWidth))
		}
		return rows
	}
	if m.HistoryDepth == historyDepthTargets {
		return append(rows, m.historyDiagnosticRows(width, bodyHeight)...)
	}
	if m.HistoryDepth == historyDepthLogs {
		return append(rows, m.historyLogRows(width, bodyHeight)...)
	}
	return append(rows, m.historyRunListRows(width, bodyHeight)...)
}

func (m Model) historyTabRow(width int) string {
	runs := historyTabLabel("Project runs", len(m.filteredHistoryRuns()), len(m.RunHistory), m.HistoryFilter != "")
	commands := historyTabLabel("Commands", len(m.filteredHistoryCommands()), len(m.History), m.HistoryFilter != "")
	if m.HistoryTab == historyTabRuns {
		runs = paletteActiveStyle.Render(" " + runs + " ")
		commands = subtleStyle.Render(" " + commands + " ")
	} else {
		runs = subtleStyle.Render(" " + runs + " ")
		commands = paletteActiveStyle.Render(" " + commands + " ")
	}
	query := "/ filter"
	if m.HistoryFilter != "" || m.HistorySearching {
		query = "/ " + m.HistoryFilter
		if m.HistorySearching {
			query += "▌"
		}
	}
	return visibleJoin(runs+"  "+commands, subtleStyle.Render(query), width)
}

func historyTabLabel(label string, visible, total int, filtering bool) string {
	if filtering {
		return fmt.Sprintf("%s %d/%d", label, visible, total)
	}
	return fmt.Sprintf("%s %d", label, total)
}

func (m Model) historyRunListRows(width, height int) []string {
	if height <= 0 {
		return nil
	}
	runs := m.filteredHistoryRuns()
	rows := []string{subtleStyle.Render(truncateVisible("  WHEN   RESULT          TARGETS  COMMAND", width))}
	if len(runs) == 0 {
		message := "No project runs yet."
		if len(m.RunHistory) > 0 {
			message = "No project runs match."
		}
		return append(rows, "  "+message)
	}
	position := clampHistoryIndex(m.HistoryPos, len(runs))
	visible := max(1, height-1)
	start := historyWindowStart(position, visible, len(runs))
	end := min(len(runs), start+visible)
	for i := start; i < end; i++ {
		run := runs[i]
		prefix := "  "
		if i == position {
			prefix = "› "
		}
		when := padRightVisible(formatHistoryTime(run.Time), 6)
		result := padRightVisible(m.historyResultLabel(run), 15)
		targets := padLeftVisible(fmt.Sprintf("%d/%d", run.Succeeded, run.Total), 7)
		commandWidth := max(1, width-2-6-1-15-1-7-2)
		line := prefix + when + " " + result + " " + targets + "  " + truncateVisible(m.highlightHistoryMatch(run.Command), commandWidth)
		if i == position {
			line = paletteActiveStyle.Render(padRightVisible(line, width))
		}
		rows = append(rows, line)
	}
	return rows
}

func (m Model) historyCommandListRows(width, height int) []string {
	if height <= 0 {
		return nil
	}
	commands := m.filteredHistoryCommands()
	rows := []string{subtleStyle.Render(truncateVisible("  #   COMMAND", width))}
	if len(commands) == 0 {
		message := "No command history yet."
		if len(m.History) > 0 {
			message = "No commands match."
		}
		return append(rows, "  "+message)
	}
	position := clampHistoryIndex(m.HistoryCommandPos, len(commands))
	visible := max(1, height-1)
	start := historyWindowStart(position, visible, len(commands))
	end := min(len(commands), start+visible)
	for i := start; i < end; i++ {
		prefix := fmt.Sprintf("  %-3d", i+1)
		if i == position {
			prefix = fmt.Sprintf("› %-3d", i+1)
		}
		line := prefix + " " + truncateVisible(m.highlightHistoryMatch(commands[i]), max(1, width-5))
		if i == position {
			line = paletteActiveStyle.Render(padRightVisible(line, width))
		}
		rows = append(rows, line)
	}
	return rows
}

func (m Model) historyDiagnosticRows(width, height int) []string {
	rows := m.historyDiagnosticAllRows(width, height)
	if len(rows) <= height {
		return rows
	}
	maxStart := len(rows) - height
	offsetFromEnd := min(max(0, m.HistoryDetailOffset), maxStart)
	start := maxStart - offsetFromEnd
	return rows[start : start+height]
}

func (m Model) historyDiagnosticAllRows(width, height int) []string {
	if height <= 0 {
		return nil
	}
	run, ok := m.selectedHistoryRun()
	if !ok {
		return []string{sectionStyle.Render("Diagnostic"), "No project run selected."}
	}
	rows := []string{sectionStyle.Render("Diagnostic")}
	commandWidth := max(1, width-lipgloss.Width("command  "))
	commandRows := wrapHistoryText(run.Command, commandWidth)
	if len(commandRows) == 0 {
		commandRows = []string{"-"}
	}
	rows = append(rows, subtleStyle.Render("command")+"  "+commandRows[0])
	for _, line := range commandRows[1:] {
		rows = append(rows, strings.Repeat(" ", lipgloss.Width("command  "))+line)
	}
	completed := run.Time
	if !run.Ended.IsZero() {
		completed = run.Ended
	}
	rows = append(rows,
		subtleStyle.Render("ended")+"    "+formatHistoryExactTime(completed),
		subtleStyle.Render("duration")+" "+formatHistoryDuration(run.Started, run.Ended),
		fmt.Sprintf("%s  %d ok  %d failed  %d cancelled", m.historyResultLabel(run), run.Succeeded, run.Failed, run.Cancelled),
	)
	logs := "unavailable"
	if run.LogID != "" {
		logs = "retained"
	}
	rows = append(rows, subtleStyle.Render("logs")+"     "+logs)
	if len(run.Targets) == 0 {
		rows = append(rows, "", "target details unavailable for legacy run")
		return rows
	}
	label := "Failed and cancelled targets"
	if m.HistoryShowAll {
		label = "All targets"
	}
	visibleTargets := m.visibleHistoryTargets()
	rows = append(rows, "", sectionStyle.Render(fmt.Sprintf("%s (%d/%d)", label, len(visibleTargets), len(run.Targets))))
	if len(visibleTargets) == 0 {
		rows = append(rows, "No failed or cancelled targets. Press a to show all.")
		return rows
	}
	rows = append(rows, subtleStyle.Render(truncateVisible("  STATUS      EXIT  DURATION  TARGET", width)))
	return m.appendHistoryTargetTableRows(rows, visibleTargets, width, height)
}

func (m Model) appendHistoryTargetTableRows(rows []string, targets []history.TargetEntry, width, height int) []string {
	selectedTarget, _ := m.selectedHistoryTarget()
	errorRows := 0
	if selectedTarget.Error != "" {
		errorRows = 1
	}
	remaining := max(1, height-len(rows)-errorRows)
	position := clampHistoryIndex(m.HistoryTargetPos, len(targets))
	start := historyWindowStart(position, remaining, len(targets))
	end := min(len(targets), start+remaining)
	for i := start; i < end; i++ {
		target := targets[i]
		prefix := "  "
		if i == position {
			prefix = "› "
		}
		status := padRightVisible(historyTargetStatus(target.Status), 11)
		exit := "-"
		if target.Status == core.StatusSucceeded || target.Status == core.StatusFailed {
			exit = fmt.Sprintf("%d", target.ExitCode)
		}
		exit = padLeftVisible(exit, 4)
		duration := padLeftVisible(formatHistoryDuration(target.Started, target.Ended), 8)
		pathWidth := max(1, width-2-11-2-4-2-8-2)
		line := prefix + status + "  " + exit + "  " + duration + "  " + truncateVisible(target.RelPath, pathWidth)
		if i == position {
			line = paletteActiveStyle.Render(padRightVisible(line, width))
		}
		rows = append(rows, line)
	}
	if selectedTarget.Error != "" {
		rows = append(rows, metricFailedStyle.Render("error")+"  "+truncateVisible(selectedTarget.Error, max(1, width-7)))
	}
	return rows
}

func (m Model) historyLogRows(width, height int) []string {
	target, ok := m.selectedHistoryTarget()
	if !ok {
		return []string{sectionStyle.Render("Logs"), "No target selected."}
	}
	rows := []string{sectionStyle.Render("Logs · " + target.RelPath)}
	switch {
	case m.HistoryLogLoading:
		rows = append(rows, "Loading persisted logs...")
	case m.HistoryLogError != "":
		rows = append(rows, m.HistoryLogError)
	case m.HistoryLog == "":
		rows = append(rows, "(empty log)")
	default:
		visible := max(1, height-1)
		rows = append(rows, viewportRows(m.configuredHistoryLogViewport(width, visible))...)
	}
	return truncateHistoryRows(rows, height)
}

func (m Model) maxHistoryLogOffset() int {
	if m.HistoryLog == "" {
		return 0
	}
	panelHeight, _, _ := m.panelDimensions(m.Width, m.Height)
	bodyHeight := max(1, panelHeight-3)
	visible := max(1, bodyHeight-1)
	model := m.configuredHistoryLogViewport(1, visible)
	model.GotoBottom()
	return model.YOffset()
}

func (m Model) maxHistoryDetailOffset() int {
	width := m.Width
	if width == 0 {
		width = 80
	}
	panelHeight, _, _ := m.panelDimensions(width, m.Height)
	boxWidth := width - 4
	if width >= 120 {
		boxWidth = min(boxWidth, 136)
	}
	boxWidth = max(52, min(width, boxWidth))
	contentWidth := max(1, boxWidth-4)
	bodyHeight := max(1, panelHeight-3)
	diagnosticWidth := contentWidth
	if contentWidth >= 112 {
		dividerWidth := lipgloss.Width(" │ ")
		leftWidth := (contentWidth - dividerWidth) * 46 / 100
		diagnosticWidth = contentWidth - dividerWidth - leftWidth
	}
	return max(0, len(m.historyDiagnosticAllRows(diagnosticWidth, bodyHeight))-bodyHeight)
}

func (m Model) filteredHistoryRuns() []history.RunEntry {
	runs := make([]history.RunEntry, 0, len(m.RunHistory))
	for _, run := range m.RunHistory {
		if m.HistoryFilter == "" || filterMatches(run.Command, m.HistoryFilter) {
			runs = append(runs, run)
		}
	}
	return runs
}

func (m Model) filteredHistoryCommands() []string {
	commands := make([]string, 0, len(m.History))
	for _, command := range m.History {
		if m.HistoryFilter == "" || filterMatches(command, m.HistoryFilter) {
			commands = append(commands, command)
		}
	}
	return commands
}

func (m Model) selectedHistoryRun() (history.RunEntry, bool) {
	runs := m.filteredHistoryRuns()
	if len(runs) == 0 {
		return history.RunEntry{}, false
	}
	return runs[clampHistoryIndex(m.HistoryPos, len(runs))], true
}

func (m Model) visibleHistoryTargets() []history.TargetEntry {
	run, ok := m.selectedHistoryRun()
	if !ok {
		return nil
	}
	targets := make([]history.TargetEntry, 0, len(run.Targets))
	for _, target := range run.Targets {
		if !m.HistoryShowAll && target.Status != core.StatusFailed && target.Status != core.StatusCancelled {
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func (m Model) selectedHistoryTarget() (history.TargetEntry, bool) {
	targets := m.visibleHistoryTargets()
	if len(targets) == 0 {
		return history.TargetEntry{}, false
	}
	return targets[clampHistoryIndex(m.HistoryTargetPos, len(targets))], true
}

func (m Model) historyResultLabel(run history.RunEntry) string {
	switch {
	case run.Failed > 0:
		return metricFailedStyle.Render(fmt.Sprintf("%d failed", run.Failed))
	case run.Cancelled > 0:
		return statusStyles[core.StatusCancelled].Render(fmt.Sprintf("%d cancelled", run.Cancelled))
	case run.Total > 0 && run.Succeeded == run.Total:
		return metricSuccessStyle.Render("ok")
	case run.Succeeded > 0:
		return metricSuccessStyle.Render("partial")
	default:
		return subtleStyle.Render("unknown")
	}
}

func historyTargetStatus(status core.Status) string {
	switch status {
	case core.StatusSucceeded:
		return metricSuccessStyle.Render("ok")
	case core.StatusFailed:
		return metricFailedStyle.Render("failed")
	case core.StatusCancelled:
		return statusStyles[core.StatusCancelled].Render("cancelled")
	default:
		return string(status)
	}
}

func historyWindowStart(position, visible, total int) int {
	if visible <= 0 || total <= visible {
		return 0
	}
	return min(max(0, position-visible+1), total-visible)
}

func clampHistoryIndex(position, total int) int {
	if total <= 0 {
		return 0
	}
	return min(max(0, position), total-1)
}

func truncateHistoryRows(rows []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(rows) <= height {
		return rows
	}
	return rows[:height]
}

func wrapHistoryText(value string, width int) []string {
	width = max(1, width)
	if value == "" {
		return nil
	}
	rows := []string{}
	var row strings.Builder
	rowWidth := 0
	for _, r := range value {
		cellWidth := ansi.StringWidth(string(r))
		if rowWidth > 0 && rowWidth+cellWidth > width {
			rows = append(rows, row.String())
			row.Reset()
			rowWidth = 0
		}
		row.WriteRune(r)
		rowWidth += cellWidth
	}
	if row.Len() > 0 {
		rows = append(rows, row.String())
	}
	return rows
}

func formatHistoryExactTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func formatHistoryDuration(started, ended time.Time) string {
	if started.IsZero() || ended.IsZero() || ended.Before(started) {
		return "-"
	}
	duration := ended.Sub(started)
	if duration < time.Second {
		return duration.Round(time.Millisecond).String()
	}
	return duration.Round(time.Second).String()
}
