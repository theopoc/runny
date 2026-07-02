package tui

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/saewyn/runny/internal/core"
	"github.com/saewyn/runny/internal/history"
	"github.com/saewyn/runny/internal/runner"
)

var (
	runnyBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#C4B5FD"))
	headerStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB")).Background(lipgloss.Color("#111827"))
	subtleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	panelStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	panelTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#67E8F9"))
	cursorStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	selectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	unselectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	statusStyles    = map[core.Status]lipgloss.Style{
		core.StatusQueued:    lipgloss.NewStyle().Foreground(lipgloss.Color("#93C5FD")),
		core.StatusRunning:   lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24")).Bold(true),
		core.StatusSucceeded: lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true),
		core.StatusFailed:    lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185")).Bold(true),
		core.StatusCancelled: lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")),
		core.StatusSkipped:   lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")),
	}
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Background(lipgloss.Color("#1F2937"))
)

type Focus int

const (
	FocusCommand Focus = iota
	FocusTargets
	FocusFilter
	FocusLogs
)

type Options struct {
	Command            string
	Targets            []core.Target
	Mode               core.ExecutionMode
	Workers            int
	FailFast           bool
	SaveLogs           bool
	DisableLogging     bool
	LogRoot            string
	CommandHistoryPath string
	RunHistoryPath     string
}

type Model struct {
	Command            string
	Targets            []core.Target
	Status             map[string]core.Status
	Logs               map[string]string
	History            []string
	RunHistory         []history.RunEntry
	Focus              Focus
	Cursor             int
	DirectoryOffset    int
	HistoryPos         int
	Filter             string
	ShowHelp           bool
	ShowHistory        bool
	ConfirmRun         bool
	Running            bool
	PendingRuns        int
	runCtx             context.Context
	runQueue           []core.Target
	completedResults   []core.RunResult
	RunError           string
	Width              int
	Height             int
	Mode               core.ExecutionMode
	Workers            int
	FailFast           bool
	SaveLogs           bool
	DisableLogging     bool
	LogRoot            string
	CommandHistoryPath string
	RunHistoryPath     string
	cancelRun          context.CancelFunc
	targetCancels      map[string]context.CancelFunc
	runFunc            func(context.Context, core.RunRequest) ([]core.RunResult, error)
}

func NewModel(opts Options) Model {
	status := map[string]core.Status{}
	logs := map[string]string{}
	for _, target := range opts.Targets {
		status[target.ID] = core.StatusQueued
		logs[target.ID] = ""
	}
	focus := FocusTargets
	if opts.Command == "" {
		focus = FocusCommand
	}
	model := Model{
		Command:            opts.Command,
		Targets:            opts.Targets,
		Status:             status,
		Logs:               logs,
		Focus:              focus,
		Mode:               opts.Mode,
		Workers:            opts.Workers,
		FailFast:           opts.FailFast,
		SaveLogs:           opts.SaveLogs,
		DisableLogging:     opts.DisableLogging,
		LogRoot:            opts.LogRoot,
		CommandHistoryPath: opts.CommandHistoryPath,
		RunHistoryPath:     opts.RunHistoryPath,
		targetCancels:      map[string]context.CancelFunc{},
		runFunc:            runner.Run,
	}
	if opts.CommandHistoryPath != "" {
		if entries, err := history.ReadCommands(opts.CommandHistoryPath); err == nil {
			for i := len(entries) - 1; i >= 0; i-- {
				model.History = append(model.History, entries[i].Command)
			}
		}
	}
	if opts.RunHistoryPath != "" {
		if entries, err := history.ReadRuns(opts.RunHistoryPath); err == nil {
			for i := len(entries) - 1; i >= 0; i-- {
				model.RunHistory = append(model.RunHistory, entries[i])
			}
		}
	}
	return model
}

func (m Model) Init() tea.Cmd { return nil }

type runDoneMsg struct {
	targetID string
	results  []core.RunResult
	err      error
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width = size.Width
		m.Height = size.Height
		return m, nil
	}
	if done, ok := msg.(runDoneMsg); ok {
		cmd := m.applyRunDone(done)
		return m, cmd
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	keyName := key.String()
	if key.Key().Text != "" {
		keyName = key.Key().Text
	}
	if m.ShowHelp || m.ShowHistory || m.ConfirmRun {
		return m.handleOverlayKey(keyName)
	}
	if m.Focus == FocusCommand {
		return m.handleCommandKey(keyName, key)
	}
	if m.Focus == FocusFilter {
		return m.handleFilterKey(keyName, key)
	}
	switch keyName {
	case "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	case "esc":
		return m, nil
	case "q":
		if !m.hasActiveRuns() && m.Focus != FocusCommand && m.Focus != FocusFilter {
			return m, tea.Quit
		}
	case "enter":
		return m.startRun(false)
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "tab":
		m.Focus = (m.Focus + 1) % 4
	case "/":
		m.Focus = FocusFilter
	case "?":
		m.ShowHelp = !m.ShowHelp
	case "H":
		m.ShowHistory = !m.ShowHistory
		m.HistoryPos = 0
	case " ":
		m.toggleFocused()
	case "a":
		m.setVisibleSelected(true)
	case "A":
		m.setVisibleSelected(false)
	case "right", "l":
		m.setFolded(false)
	case "left":
		m.setFolded(true)
	case "delete":
		m.cancelSelectedOrFocused()
	case "R":
		if !m.Running && m.failedCount() > 0 {
			m.ConfirmRun = true
		}
	}
	return m, nil
}

func (m Model) handleCommandKey(keyName string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyName {
	case "enter":
		return m.startRun(false)
	case "esc":
		m.Focus = FocusTargets
	case "tab":
		m.Focus = (m.Focus + 1) % 4
	case "backspace":
		if len(m.Command) > 0 {
			m.Command = m.Command[:len(m.Command)-1]
		}
	default:
		if key.Key().Text != "" {
			m.Command += key.Key().Text
		}
	}
	return m, nil
}

func (m Model) handleFilterKey(keyName string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyName {
	case "esc", "enter":
		m.Focus = FocusTargets
	case "tab":
		m.Focus = (m.Focus + 1) % 4
	case "backspace":
		if len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
			m.ensureCursorVisible()
		}
	default:
		if key.Key().Text != "" {
			m.Filter += key.Key().Text
			m.ensureCursorVisible()
		}
	}
	return m, nil
}

func (m Model) handleOverlayKey(keyName string) (tea.Model, tea.Cmd) {
	switch keyName {
	case "esc", "q":
		m.ShowHelp = false
		m.ShowHistory = false
		m.ConfirmRun = false
	case "?":
		m.ShowHelp = !m.ShowHelp
	case "H":
		m.ShowHelp = false
		m.ShowHistory = true
		m.HistoryPos = 0
	case "delete":
		m.cancelSelectedOrFocused()
	case "up", "k":
		if m.ShowHistory && m.HistoryPos > 0 {
			m.HistoryPos--
		}
	case "down", "j":
		if m.ShowHistory && m.HistoryPos < m.selectableHistoryLen()-1 {
			m.HistoryPos++
		}
	case "enter":
		if m.ShowHistory {
			if len(m.History) > 0 {
				m.Command = m.History[m.clampHistoryPos()]
			}
			m.ShowHistory = false
			m.Focus = FocusCommand
			return m, nil
		}
		if m.ConfirmRun {
			m.ConfirmRun = false
			return m.startRun(true)
		}
		m.ShowHelp = false
	}
	return m, nil
}

func (m Model) startRun(failedOnly bool) (tea.Model, tea.Cmd) {
	if m.Running || strings.TrimSpace(m.Command) == "" {
		return m, nil
	}
	targets := m.targetsForRun(failedOnly)
	if len(targets) == 0 {
		m.RunError = "no selected targets"
		return m, nil
	}
	reqTargets := append([]core.Target(nil), targets...)
	ctx, cancel := context.WithCancel(context.Background())
	if m.targetCancels == nil {
		m.targetCancels = map[string]context.CancelFunc{}
	}
	m.cancelRun = cancel
	m.runCtx = ctx
	m.targetCancels = map[string]context.CancelFunc{}
	m.Running = true
	m.PendingRuns = len(reqTargets)
	m.runQueue = append([]core.Target(nil), reqTargets...)
	m.completedResults = nil
	m.RunError = ""
	command := strings.TrimSpace(m.Command)
	m.addHistory(command)
	if m.CommandHistoryPath != "" {
		if err := history.AppendCommand(m.CommandHistoryPath, history.CommandEntry{Command: command, Time: time.Now()}); err != nil {
			m.RunError = err.Error()
		}
	}
	for _, target := range reqTargets {
		m.Status[target.ID] = core.StatusQueued
		m.Logs[target.ID] = ""
	}
	return m.startQueuedRuns()
}

func (m Model) targetsForRun(failedOnly bool) []core.Target {
	targets := make([]core.Target, 0, len(m.Targets))
	for _, target := range m.Targets {
		if failedOnly {
			if m.Status[target.ID] != core.StatusFailed {
				continue
			}
			target.Selected = true
		}
		if target.Selected {
			targets = append(targets, target)
		}
	}
	return targets
}

func (m *Model) applyRunDone(done runDoneMsg) tea.Cmd {
	if done.targetID != "" {
		delete(m.targetCancels, done.targetID)
	}
	if m.PendingRuns > 0 {
		m.PendingRuns--
	}
	if done.err != nil {
		m.RunError = done.err.Error()
	}
	for _, result := range done.results {
		if m.Status[result.Target.ID] == core.StatusCancelled && result.Status != core.StatusCancelled {
			result.Status = core.StatusCancelled
		}
		m.Status[result.Target.ID] = result.Status
		m.completedResults = append(m.completedResults, result)
		var log strings.Builder
		if result.Output != "" {
			log.WriteString(result.Output)
		}
		if result.Error != "" {
			if log.Len() > 0 && !strings.HasSuffix(log.String(), "\n") {
				log.WriteByte('\n')
			}
			log.WriteString(result.Error)
		}
		m.Logs[result.Target.ID] = log.String()
	}
	if m.PendingRuns == 0 {
		m.appendRunHistory()
		m.Running = false
		m.runCtx = nil
		m.runQueue = nil
		m.cancelRun = nil
		m.targetCancels = map[string]context.CancelFunc{}
		return nil
	}
	next, cmd := m.startQueuedRuns()
	*m = next
	return cmd
}

func (m Model) startQueuedRuns() (Model, tea.Cmd) {
	if !m.Running {
		return m, nil
	}
	limit := m.workerLimit()
	if limit <= 0 {
		limit = 1
	}
	available := limit - len(m.targetCancels)
	if available <= 0 || len(m.runQueue) == 0 {
		return m, nil
	}
	cmds := make([]tea.Cmd, 0, available)
	for available > 0 && len(m.runQueue) > 0 {
		target := m.runQueue[0]
		m.runQueue = m.runQueue[1:]
		cmds = append(cmds, m.startTargetCmd(target))
		available--
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) startTargetCmd(target core.Target) tea.Cmd {
	ctx := m.runCtx
	if ctx == nil {
		ctx = context.Background()
	}
	targetCtx, targetCancel := context.WithCancel(ctx)
	if m.targetCancels == nil {
		m.targetCancels = map[string]context.CancelFunc{}
	}
	m.targetCancels[target.ID] = targetCancel
	m.Status[target.ID] = core.StatusRunning
	runFunc := m.runFunc
	if runFunc == nil {
		runFunc = runner.Run
	}
	req := core.RunRequest{
		Command:        strings.TrimSpace(m.Command),
		Targets:        []core.Target{target},
		Mode:           core.ModeSerial,
		Workers:        1,
		FailFast:       m.FailFast,
		SaveLogs:       m.SaveLogs,
		DisableLogging: m.DisableLogging,
		LogRoot:        m.LogRoot,
	}
	return func() tea.Msg {
		results, err := runFunc(targetCtx, req)
		return runDoneMsg{targetID: target.ID, results: results, err: err}
	}
}

func (m Model) workerLimit() int {
	if m.mode() == core.ModeSerial {
		return 1
	}
	if m.Workers > 0 {
		return m.Workers
	}
	cpus := runtime.NumCPU()
	if cpus < 1 {
		return 1
	}
	return cpus
}

func (m *Model) appendRunHistory() {
	summary := history.RunEntry{Command: m.Command, Time: time.Now()}
	for _, result := range m.completedResults {
		summary.Total++
		switch result.Status {
		case core.StatusSucceeded:
			summary.Succeeded++
		case core.StatusFailed:
			summary.Failed++
		case core.StatusCancelled:
			summary.Cancelled++
		}
	}
	if summary.Total == 0 {
		return
	}
	if m.RunHistoryPath != "" {
		if err := history.AppendRun(m.RunHistoryPath, summary); err != nil && m.RunError == "" {
			m.RunError = err.Error()
		}
	}
	m.RunHistory = append([]history.RunEntry{summary}, m.RunHistory...)
	if len(m.RunHistory) > 100 {
		m.RunHistory = m.RunHistory[:100]
	}
}

func (m *Model) addHistory(command string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return
	}
	for _, item := range m.History {
		if item == command {
			return
		}
	}
	m.History = append([]string{command}, m.History...)
	if len(m.History) > 50 {
		m.History = m.History[:50]
	}
}

func (m Model) View() tea.View {
	content := m.render()
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "runny"
	return view
}

func (m Model) render() string {
	var b strings.Builder
	width := m.Width
	if width < 80 {
		width = 80
	}
	height := m.Height
	if height < 20 {
		height = 20
	}
	panelHeight := max(10, height-8)
	leftWidth := width * 58 / 100
	if leftWidth < 50 {
		leftWidth = 50
	}
	rightWidth := width - leftWidth - 4
	if rightWidth < 32 {
		rightWidth = 32
		leftWidth = width - rightWidth - 4
	}

	b.WriteString(m.renderHeader(width))
	b.WriteByte('\n')
	b.WriteString(m.renderSubHeader(width))
	b.WriteByte('\n')
	left := m.renderDirectoryPanel(leftWidth, panelHeight)
	right := m.renderLogPanel(rightWidth, panelHeight)
	b.WriteString(joinPanels(left, right))
	if m.ShowHelp {
		b.WriteString("\n\n")
		b.WriteString(renderBox(width, "Shortcuts", []string{
			"tab focus command/directories/filter/logs   enter run",
			"space toggle   a select all   A deselect all   / filter",
			"up/down or j/k move   left fold   right/l unfold",
			"H history   del cancel selected running   R rerun failed",
			"ctrl+c cancel active runs and quit   q quit when idle",
		}))
	}
	if m.ShowHistory {
		b.WriteString("\n\n")
		b.WriteString(renderBox(width, "History", m.historyRows()))
	}
	if m.ConfirmRun {
		b.WriteString("\n\n")
		b.WriteString(renderBox(width, "Rerun failed", []string{
			fmt.Sprintf("%d failed target(s) will run again.", m.failedCount()),
			"enter confirm   esc cancel",
		}))
	}
	if m.RunError != "" {
		b.WriteString("\n")
		b.WriteString(statusStyles[core.StatusFailed].Render(" " + m.RunError))
	}
	b.WriteByte('\n')
	b.WriteString(renderFooter(width))
	return b.String()
}

func (m Model) renderHeader(width int) string {
	command := m.Command
	if command == "" {
		command = "<enter command>"
	}
	selected := 0
	running := 0
	failed := 0
	for _, target := range m.Targets {
		if target.Selected {
			selected++
		}
		switch m.Status[target.ID] {
		case core.StatusRunning:
			running++
		case core.StatusFailed:
			failed++
		}
	}
	left := " " + runnyBadgeStyle.Render("runny") + "  " + command
	right := fmt.Sprintf("%d/%d selected  running %d  failed %d", selected, len(m.Targets), running, failed)
	line := visibleJoin(left, right, width)
	return headerStyle.Render(line)
}

func (m Model) renderSubHeader(width int) string {
	filterText := m.Filter
	if filterText == "" {
		filterText = "<none>"
	}
	focusText := "directories"
	if m.Focus == FocusCommand {
		focusText = "command"
	} else if m.Focus == FocusFilter {
		focusText = "filter"
	} else if m.Focus == FocusLogs {
		focusText = "logs"
	}
	workers := "auto"
	if m.Workers > 0 {
		workers = fmt.Sprintf("%d", m.Workers)
	}
	line := " filter " + filterText + "  focus " + focusText + "  mode " + string(m.mode()) + "  workers " + workers
	return subtleStyle.Render(padRightVisible(truncateVisible(line, width), width))
}

func (m Model) mode() core.ExecutionMode {
	if m.Mode == "" {
		return core.ModeParallel
	}
	return m.Mode
}

func (m Model) renderDirectoryPanel(width int, height int) []string {
	rows := []string{panelTitleStyle.Render(padRightVisible("SEL  DIR", width-18) + "STATUS")}
	visibleIndexes := m.visibleTargetIndexes()
	limit := max(1, height-4)
	offset := m.DirectoryOffset
	if offset < 0 {
		offset = 0
	}
	if offset > max(0, len(visibleIndexes)-1) {
		offset = max(0, len(visibleIndexes)-1)
	}
	count := 0
	for _, targetIndex := range visibleIndexes[offset:] {
		if count >= limit {
			break
		}
		target := m.Targets[targetIndex]
		rows = append(rows, m.renderTargetRow(targetIndex, target, width-4))
		count++
	}
	if count == 0 {
		rows = append(rows, "  no directories")
	}
	return boxLines(width, height, "Directories", rows, true)
}

func (m Model) renderTargetRow(index int, target core.Target, width int) string {
	cursor := " "
	if index == m.Cursor {
		cursor = cursorStyle.Render("›")
	}
	selected := unselectedStyle.Render("○")
	if target.Selected {
		selected = selectedStyle.Render("●")
	}
	fold := " "
	if target.Folded {
		fold = "+"
	} else if len(target.Children) > 0 {
		fold = "−"
	}
	name := strings.Repeat("  ", max(0, target.Depth-1)) + target.RelPath
	status := m.Status[target.ID]
	nameWidth := max(10, width-17)
	statusText := padRightVisible(string(status), 8)
	if style, ok := statusStyles[status]; ok {
		statusText = style.Render(statusText)
	}
	return cursor + " " + selected + "  " + fold + " " + padRightVisible(truncateVisible(name, nameWidth), nameWidth) + " " + statusText
}

func (m Model) renderLogPanel(width int, height int) []string {
	lines := []string{}
	if len(m.Targets) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Targets) {
		target := m.Targets[m.Cursor]
		lines = append(lines, "focused "+target.RelPath)
		lines = append(lines, "status "+string(m.Status[target.ID]))
		if log := strings.TrimRight(m.Logs[target.ID], "\n"); log != "" {
			lines = append(lines, "", "output")
			lines = append(lines, strings.Split(log, "\n")...)
			return boxLines(width, height, "Logs", lines, false)
		}
	} else {
		lines = append(lines, "focused none")
		lines = append(lines, "status none")
	}
	lines = append(lines, "", "output", "No output yet.")
	return boxLines(width, height, "Logs", lines, false)
}

func (m Model) hiddenByFold(target core.Target) bool {
	parentID := target.ParentID
	for parentID != "" {
		found := false
		for _, candidate := range m.Targets {
			if candidate.ID == parentID {
				if candidate.Folded {
					return true
				}
				parentID = candidate.ParentID
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return false
}

func joinPanels(left []string, right []string) string {
	var b strings.Builder
	height := max(len(left), len(right))
	for i := range height {
		if i < len(left) {
			b.WriteString(left[i])
		} else {
			b.WriteString(strings.Repeat(" ", len(left[0])))
		}
		b.WriteString("  ")
		if i < len(right) {
			b.WriteString(right[i])
		}
		if i < height-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderBox(width int, title string, rows []string) string {
	return strings.Join(boxLines(width, len(rows)+2, title, rows, false), "\n")
}

func boxLines(width int, height int, title string, rows []string, active bool) []string {
	width = max(width, len(title)+6)
	height = max(height, 3)
	lines := make([]string, 0, height)
	titleText := " " + panelTitleStyle.Render(title) + " "
	topFill := max(0, width-lipgloss.Width(titleText)-2)
	borderStyle := panelStyle
	if active {
		borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#67E8F9"))
	}
	lines = append(lines, borderStyle.Render("╭─")+titleText+borderStyle.Render(strings.Repeat("─", topFill)+"╮"))
	contentWidth := width - 4
	for i := 0; i < height-2; i++ {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		lines = append(lines, borderStyle.Render("│")+" "+padRightVisible(truncateVisible(row, contentWidth), contentWidth)+" "+borderStyle.Render("│"))
	}
	lines = append(lines, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return lines
}

func renderFooter(width int) string {
	text := " Shortcuts space toggle / filter a all A none enter run ? help H history del cancel ctrl+c quit"
	return footerStyle.Render(padRightVisible(truncateVisible(text, width), width))
}

func (m Model) historyRows() []string {
	rows := []string{"Commands"}
	if len(m.History) == 0 {
		rows = append(rows, "  No command history yet.")
	} else {
		for i, command := range m.History {
			if i >= 6 {
				break
			}
			prefix := "  "
			if i == m.HistoryPos {
				prefix = "› "
			}
			rows = append(rows, prefix+command)
		}
	}
	rows = append(rows, "", "Project runs")
	if len(m.RunHistory) == 0 {
		rows = append(rows, "  No project runs yet.")
	} else {
		for i, run := range m.RunHistory {
			if i >= 5 {
				break
			}
			rows = append(rows, fmt.Sprintf("  %s  %d total  %d ok  %d failed  %d cancelled", run.Command, run.Total, run.Succeeded, run.Failed, run.Cancelled))
		}
	}
	rows = append(rows, "", "enter reuse command   esc close")
	return rows
}

func (m Model) selectableHistoryLen() int {
	if len(m.History) > 6 {
		return 6
	}
	return len(m.History)
}

func (m Model) clampHistoryPos() int {
	limit := m.selectableHistoryLen()
	if limit == 0 {
		return 0
	}
	if m.HistoryPos >= limit {
		return limit - 1
	}
	return m.HistoryPos
}

func visibleJoin(left string, right string, width int) string {
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return truncateVisible(left+strings.Repeat(" ", space)+right, width)
}

func truncateVisible(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if lipgloss.Width(b.String()+string(r)+"~") > width {
			break
		}
		b.WriteRune(r)
	}
	b.WriteString("~")
	return b.String()
}

func padRightVisible(value string, width int) string {
	current := lipgloss.Width(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

func (m *Model) toggleFocused() {
	if len(m.Targets) == 0 {
		return
	}
	m.ensureCursorVisible()
	m.Targets[m.Cursor].Selected = !m.Targets[m.Cursor].Selected
}

func (m *Model) setVisibleSelected(selected bool) {
	for i := range m.Targets {
		if m.isVisibleTarget(m.Targets[i]) {
			m.Targets[i].Selected = selected
		}
	}
}

func (m *Model) setFolded(folded bool) {
	if len(m.Targets) == 0 {
		return
	}
	m.ensureCursorVisible()
	m.Targets[m.Cursor].Folded = folded
}

func (m *Model) cancelSelectedOrFocused() {
	cancelled := false
	for _, target := range m.Targets {
		if target.Selected {
			cancelled = m.cancelTarget(target) || cancelled
		}
	}
	if !cancelled && len(m.Targets) > 0 {
		m.ensureCursorVisible()
		m.cancelTarget(m.Targets[m.Cursor])
	}
}

func (m *Model) cancelTarget(target core.Target) bool {
	switch m.Status[target.ID] {
	case core.StatusRunning:
		if cancel := m.targetCancels[target.ID]; cancel != nil {
			cancel()
		}
		m.Status[target.ID] = core.StatusCancelled
		return true
	case core.StatusQueued:
		if !m.removeQueuedTarget(target.ID) {
			return false
		}
		m.Status[target.ID] = core.StatusCancelled
		if m.PendingRuns > 0 {
			m.PendingRuns--
		}
		m.completedResults = append(m.completedResults, core.RunResult{Target: target, Status: core.StatusCancelled})
		if m.PendingRuns == 0 {
			m.appendRunHistory()
			m.Running = false
			m.runCtx = nil
			m.cancelRun = nil
			m.runQueue = nil
			m.targetCancels = map[string]context.CancelFunc{}
		}
		return true
	}
	return false
}

func (m *Model) removeQueuedTarget(id string) bool {
	for index, target := range m.runQueue {
		if target.ID == id {
			m.runQueue = append(m.runQueue[:index], m.runQueue[index+1:]...)
			return true
		}
	}
	return false
}

func (m *Model) cancelAll() {
	if m.cancelRun != nil {
		m.cancelRun()
	}
	for id, status := range m.Status {
		if status == core.StatusRunning || status == core.StatusQueued {
			m.Status[id] = core.StatusCancelled
		}
	}
}

func (m Model) hasActiveRuns() bool {
	if m.Running {
		return true
	}
	for _, status := range m.Status {
		if status == core.StatusRunning {
			return true
		}
	}
	return false
}

func (m Model) failedCount() int {
	count := 0
	for _, status := range m.Status {
		if status == core.StatusFailed {
			count++
		}
	}
	return count
}

func (m Model) visible(target core.Target) bool {
	if m.Filter == "" || strings.Contains(target.RelPath, m.Filter) {
		return true
	}
	for _, candidate := range m.Targets {
		if candidate.ParentID == target.ID && m.visible(candidate) {
			return true
		}
	}
	return false
}

func (m *Model) moveCursor(delta int) {
	if len(m.Targets) == 0 {
		return
	}
	next := m.Cursor
	for range m.Targets {
		next += delta
		if next < 0 {
			next = len(m.Targets) - 1
		}
		if next >= len(m.Targets) {
			next = 0
		}
		if m.isVisibleTarget(m.Targets[next]) {
			m.Cursor = next
			m.ensureDirectoryOffset()
			return
		}
	}
}

func (m *Model) ensureCursorVisible() {
	if len(m.Targets) == 0 {
		m.Cursor = 0
		return
	}
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) || !m.isVisibleTarget(m.Targets[m.Cursor]) {
		for i, target := range m.Targets {
			if m.isVisibleTarget(target) {
				m.Cursor = i
				m.ensureDirectoryOffset()
				return
			}
		}
		m.Cursor = 0
	}
	m.ensureDirectoryOffset()
}

func (m *Model) ensureDirectoryOffset() {
	indexes := m.visibleTargetIndexes()
	if len(indexes) == 0 {
		m.DirectoryOffset = 0
		return
	}
	visiblePosition := 0
	found := false
	for position, targetIndex := range indexes {
		if targetIndex == m.Cursor {
			visiblePosition = position
			found = true
			break
		}
	}
	if !found {
		m.DirectoryOffset = 0
		return
	}
	limit := max(1, m.directoryViewportRows())
	if visiblePosition < m.DirectoryOffset {
		m.DirectoryOffset = visiblePosition
	}
	if visiblePosition >= m.DirectoryOffset+limit {
		m.DirectoryOffset = visiblePosition - limit + 1
	}
	maxOffset := max(0, len(indexes)-limit)
	if m.DirectoryOffset > maxOffset {
		m.DirectoryOffset = maxOffset
	}
	if m.DirectoryOffset < 0 {
		m.DirectoryOffset = 0
	}
}

func (m Model) directoryViewportRows() int {
	height := m.Height
	if height < 20 {
		height = 20
	}
	panelHeight := max(10, height-8)
	return max(1, panelHeight-4)
}

func (m Model) visibleTargetIndexes() []int {
	indexes := make([]int, 0, len(m.Targets))
	for i, target := range m.Targets {
		if m.isVisibleTarget(target) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m Model) isVisibleTarget(target core.Target) bool {
	return m.visible(target) && !m.hiddenByFold(target)
}
