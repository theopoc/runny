package tui

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
	"github.com/theopoc/runny/internal/runner"
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
	TargetStarted      map[string]time.Time
	History            []string
	RunHistory         []history.RunEntry
	Focus              Focus
	Cursor             int
	DirectoryOffset    int
	HistoryPos         int
	CommandHistoryPos  int
	CommandDraft       string
	Filter             string
	ShowHelp           bool
	ShowHistory        bool
	ShowPalette        bool
	ConfirmRun         bool
	ConfirmCancelAll   bool
	Zoom               bool
	Palette            string
	PalettePos         int
	HistoryFilter      string
	HistorySearching   bool
	PreviewOffset      int
	LogFollow          bool
	Running            bool
	PendingRuns        int
	runCtx             context.Context
	runQueue           []core.Target
	completedResults   []core.RunResult
	RunError           string
	Notice             string
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
	started := map[string]time.Time{}
	for _, target := range opts.Targets {
		status[target.ID] = core.StatusIdle
		logs[target.ID] = ""
	}
	model := Model{
		Command:            opts.Command,
		Targets:            opts.Targets,
		Status:             status,
		Logs:               logs,
		TargetStarted:      started,
		Focus:              FocusTargets,
		Mode:               opts.Mode,
		Workers:            opts.Workers,
		FailFast:           opts.FailFast,
		SaveLogs:           opts.SaveLogs,
		DisableLogging:     opts.DisableLogging,
		LogRoot:            opts.LogRoot,
		CommandHistoryPath: opts.CommandHistoryPath,
		RunHistoryPath:     opts.RunHistoryPath,
		CommandHistoryPos:  -1,
		targetCancels:      map[string]context.CancelFunc{},
		runFunc:            runner.Run,
		LogFollow:          true,
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

type paletteCommand struct {
	Name        string
	Description string
}

var paletteCommands = []paletteCommand{
	{Name: "run", Description: "run selected targets"},
	{Name: "command", Description: "edit command input"},
	{Name: "failed", Description: "select failed targets"},
	{Name: "rerun-failed", Description: "rerun failed targets with confirmation"},
	{Name: "cancel", Description: "cancel selected running or queued targets"},
	{Name: "cancel-all", Description: "cancel all active and queued work"},
	{Name: "workers N|auto", Description: "set max parallel target runs or auto"},
	{Name: "serial", Description: "switch to serial mode"},
	{Name: "parallel", Description: "switch to parallel mode"},
	{Name: "logs", Description: "focus output"},
	{Name: "history", Description: "open command and run history"},
	{Name: "clear-filter", Description: "clear current filter"},
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
	if keyName == "ctrl+c" {
		m.cancelAll()
		return m, tea.Quit
	}
	if keyName == "?" {
		m.ShowHelp = !m.ShowHelp
		return m, nil
	}
	if m.ShowHelp || m.ShowHistory || m.ConfirmRun || m.ConfirmCancelAll {
		return m.handleOverlayKey(keyName, key)
	}
	if m.ShowPalette {
		return m.handlePaletteKey(keyName, key)
	}
	if m.Focus == FocusCommand {
		return m.handleCommandKey(keyName, key)
	}
	if m.Focus == FocusFilter {
		return m.handleFilterKey(keyName, key)
	}
	switch keyName {
	case "esc":
		return m, nil
	case "q":
		if !m.hasActiveRuns() && m.Focus != FocusCommand && m.Focus != FocusFilter {
			return m, tea.Quit
		}
	case ":":
		m.ShowPalette = true
		m.Palette = ""
		m.PalettePos = 0
	case "c":
		m.Focus = FocusCommand
		m.Notice = "editing command"
	case "enter":
		return m.startRun(false)
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "n":
		if m.Filter != "" {
			m.moveFilterMatch(1)
		}
	case "N":
		if m.Filter != "" {
			m.moveFilterMatch(-1)
		}
	case "home", "g":
		m.moveCursorToEdge(false)
	case "end", "G":
		m.moveCursorToEdge(true)
	case "tab":
		m.cycleFocus(1)
	case "shift+tab":
		m.cycleFocus(-1)
	case "/":
		m.Focus = FocusFilter
	case "H":
		m.ShowHistory = !m.ShowHistory
		m.HistoryPos = 0
	case "z":
		m.Zoom = !m.Zoom
		if m.Zoom {
			m.Notice = "zoom enabled"
		} else {
			m.Notice = "split view enabled"
		}
	case " ", "space":
		m.toggleFocused()
	case "a":
		m.toggleVisibleSelected()
	case "right", "l":
		m.setFolded(false)
	case "left", "h":
		m.setFolded(true)
	case "delete", "x":
		m.cancelSelectedOrFocused()
	case "R":
		if !m.hasActiveRuns() && m.failedCount() > 0 {
			m.ConfirmRun = true
		} else if m.hasActiveRuns() {
			m.Notice = "finish or cancel active run before rerun failed"
		} else {
			m.Notice = "no failed targets to rerun"
		}
	case "pageup", "ctrl+b":
		m.scrollPreview(-5)
	case "pagedown", "ctrl+f":
		m.scrollPreview(5)
	case "ctrl+u":
		m.scrollPreview(-3)
	case "ctrl+d":
		m.scrollPreview(3)
	case "f":
		m.LogFollow = !m.LogFollow
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
		m.cycleFocus(1)
	case "shift+tab":
		m.cycleFocus(-1)
	case "up":
		m.previousCommandHistory()
	case "down":
		m.nextCommandHistory()
	case "backspace":
		if len(m.Command) > 0 {
			m.Command = m.Command[:len(m.Command)-1]
		}
		m.resetCommandHistoryNavigation()
	case "ctrl+u":
		m.Command = ""
		m.Notice = "command cleared"
		m.RunError = ""
		m.resetCommandHistoryNavigation()
	case "ctrl+w":
		m.Command = trimLastWord(m.Command)
		m.resetCommandHistoryNavigation()
	case " ", "space":
		m.Command += " "
		m.resetCommandHistoryNavigation()
	default:
		if key.Key().Text != "" {
			m.Command += key.Key().Text
			m.resetCommandHistoryNavigation()
		}
	}
	return m, nil
}

func (m Model) handleFilterKey(keyName string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyName {
	case "esc", "enter":
		m.Focus = FocusTargets
	case "ctrl+u":
		m.Filter = ""
		m.ensureCursorVisible()
		m.Notice = "filter cleared"
		m.RunError = ""
	case "tab":
		m.cycleFocus(1)
	case "shift+tab":
		m.cycleFocus(-1)
	case "up":
		m.moveFilterMatch(-1)
	case "down":
		m.moveFilterMatch(1)
	case "backspace":
		if len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
			m.ensureCursorVisible()
		}
	case "ctrl+w":
		m.Filter = trimLastWord(m.Filter)
		m.ensureCursorVisible()
	default:
		if key.Key().Text != "" {
			m.Filter += key.Key().Text
			m.ensureCursorVisible()
		}
	}
	return m, nil
}

func (m Model) handlePaletteKey(keyName string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyName {
	case "esc":
		m.ShowPalette = false
	case "ctrl+u":
		m.Palette = ""
		m.PalettePos = 0
		m.RunError = ""
	case "backspace":
		if len(m.Palette) > 0 {
			m.Palette = m.Palette[:len(m.Palette)-1]
			m.PalettePos = 0
			m.RunError = ""
		}
	case "up":
		if m.PalettePos > 0 {
			m.PalettePos--
		}
	case "down":
		if m.PalettePos < len(m.filteredPaletteCommands())-1 {
			m.PalettePos++
		}
	case "enter":
		command := strings.TrimSpace(m.Palette)
		matches := m.filteredPaletteCommands()
		if len(matches) == 0 && !m.paletteInputIsRunnable(command) {
			m.RunError = "no palette command matches"
			m.Notice = ""
			return m, nil
		}
		if len(matches) > 0 && (command == "" || !m.paletteInputIsRunnable(command)) {
			command = matches[m.PalettePos].Name
		}
		m.ShowPalette = false
		return m.executePaletteCommand(command)
	default:
		if key.Key().Text != "" {
			m.Palette += key.Key().Text
			m.PalettePos = 0
			m.RunError = ""
		}
	}
	return m, nil
}

func (m Model) paletteInputIsRunnable(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	if fields[0] == "workers" {
		return len(fields) == 2
	}
	for _, candidate := range paletteCommands {
		if fields[0] == candidate.Name {
			return true
		}
	}
	return false
}

func (m Model) filteredPaletteCommands() []paletteCommand {
	query := strings.TrimSpace(m.Palette)
	if query == "" {
		return paletteCommands
	}
	matches := make([]paletteCommand, 0, len(paletteCommands))
	for _, command := range paletteCommands {
		if filterMatches(command.Name, query) || filterMatches(command.Description, query) {
			matches = append(matches, command)
		}
	}
	return matches
}

func (m Model) executePaletteCommand(command string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return m, nil
	}
	switch fields[0] {
	case "run":
		return m.startRun(false)
	case "failed":
		m.selectFailedTargets()
	case "command":
		m.Focus = FocusCommand
		m.Notice = "editing command"
	case "rerun-failed":
		if !m.hasActiveRuns() && m.failedCount() > 0 {
			m.ConfirmRun = true
		}
	case "cancel":
		m.cancelSelectedOrFocused()
	case "cancel-all":
		if m.hasActiveRuns() {
			m.ConfirmCancelAll = true
		} else {
			m.cancelAll()
		}
	case "workers":
		if len(fields) != 2 || fields[1] == "N" || fields[1] == "N|auto" {
			m.RunError = "usage: :workers N|auto"
			m.Notice = ""
			return m, nil
		}
		if strings.EqualFold(fields[1], "auto") {
			m.Workers = 0
			m.Mode = core.ModeParallel
			m.RunError = ""
			m.Notice = "workers set to auto"
			return m, nil
		}
		workers, err := strconv.Atoi(fields[1])
		if err != nil || workers < 1 {
			m.RunError = "workers must be >= 1 or auto"
			m.Notice = ""
			return m, nil
		}
		m.Workers = workers
		m.Mode = core.ModeParallel
		m.RunError = ""
		m.Notice = fmt.Sprintf("workers set to %d", workers)
	case "serial":
		m.Mode = core.ModeSerial
		m.Workers = 0
		m.Notice = "execution mode set to serial"
		m.RunError = ""
	case "parallel":
		m.Mode = core.ModeParallel
		m.Notice = "execution mode set to parallel"
		m.RunError = ""
	case "logs":
		m.Focus = FocusLogs
		m.Notice = "focused output"
	case "history":
		m.ShowHistory = true
		m.HistoryPos = 0
		m.Notice = "opened history"
	case "clear-filter":
		m.Filter = ""
		m.ensureCursorVisible()
		m.Notice = "filter cleared"
		m.RunError = ""
	default:
		m.RunError = "unknown command: " + fields[0]
	}
	return m, nil
}

func (m Model) handleOverlayKey(keyName string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.ShowHelp {
		switch keyName {
		case "esc", "q", "enter":
			m.ShowHelp = false
		case "H":
			m.ShowHelp = false
			m.ShowHistory = true
			m.HistoryPos = 0
		}
		return m, nil
	}
	if m.ShowHistory {
		return m.handleHistoryKey(keyName, key)
	}
	switch keyName {
	case "esc":
		m.ConfirmRun = false
		m.ConfirmCancelAll = false
	case "n":
		m.ConfirmRun = false
		m.ConfirmCancelAll = false
		m.Notice = "confirmation cancelled"
	case "enter", "y":
		if m.ConfirmRun {
			m.ConfirmRun = false
			return m.startRun(true)
		}
		if m.ConfirmCancelAll {
			m.ConfirmCancelAll = false
			m.cancelAll()
			return m, nil
		}
		m.ShowHelp = false
	}
	return m, nil
}

func (m Model) handleHistoryKey(keyName string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.HistorySearching {
		switch keyName {
		case "esc":
			m.HistorySearching = false
		case "enter":
			if command := m.selectedHistoryCommand(); command != "" {
				m.Command = command
				m.RunError = ""
				m.ShowHistory = false
				m.HistorySearching = false
				m.Focus = FocusCommand
			} else {
				m.RunError = "no history command matches"
			}
		case "ctrl+u":
			m.HistoryFilter = ""
			m.HistoryPos = 0
			m.RunError = ""
		case "backspace":
			if len(m.HistoryFilter) > 0 {
				m.HistoryFilter = m.HistoryFilter[:len(m.HistoryFilter)-1]
				m.HistoryPos = 0
				m.RunError = ""
			}
		case "up", "k":
			if m.HistoryPos > 0 {
				m.HistoryPos--
			}
		case "down", "j":
			if m.HistoryPos < m.selectableHistoryLen()-1 {
				m.HistoryPos++
			}
		default:
			if key.Key().Text != "" {
				m.HistoryFilter += key.Key().Text
				m.HistoryPos = 0
				m.RunError = ""
			}
		}
		return m, nil
	}
	switch keyName {
	case "esc":
		m.ShowHistory = false
	case "/":
		m.HistorySearching = true
	case "ctrl+u":
		m.HistoryFilter = ""
		m.HistoryPos = 0
		m.RunError = ""
	case "up", "k":
		if m.HistoryPos > 0 {
			m.HistoryPos--
		}
	case "down", "j":
		if m.HistoryPos < m.selectableHistoryLen()-1 {
			m.HistoryPos++
		}
	case "enter":
		if command := m.selectedHistoryCommand(); command != "" {
			m.Command = command
			m.RunError = ""
			m.ShowHistory = false
			m.Focus = FocusCommand
		} else {
			m.RunError = "no history command matches"
		}
	}
	return m, nil
}

func (m Model) startRun(failedOnly bool) (tea.Model, tea.Cmd) {
	if m.Running || strings.TrimSpace(m.Command) == "" {
		if m.Running {
			m.Notice = "run already in progress"
		} else {
			m.Focus = FocusCommand
			m.RunError = "command is empty; press c to edit"
		}
		return m, nil
	}
	targets := m.targetsForRun(failedOnly)
	if len(targets) == 0 {
		if len(m.Targets) == 0 {
			m.RunError = "no target directories found"
		} else if m.Filter != "" {
			m.RunError = "no selected targets; press a to toggle matching"
		} else {
			m.RunError = "no selected targets; press a to toggle visible"
		}
		m.Notice = ""
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
	m.Notice = fmt.Sprintf("started %d target(s)", len(reqTargets))
	command := strings.TrimSpace(m.Command)
	m.addHistory(command)
	m.resetCommandHistoryNavigation()
	if m.CommandHistoryPath != "" {
		if err := history.AppendCommand(m.CommandHistoryPath, history.CommandEntry{Command: command, Time: time.Now()}); err != nil {
			m.RunError = err.Error()
		}
	}
	for _, target := range reqTargets {
		m.Status[target.ID] = core.StatusQueued
		m.Logs[target.ID] = ""
		delete(m.TargetStarted, target.ID)
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
		if result.Started.IsZero() {
			result.Started = m.TargetStarted[result.Target.ID]
		}
		m.Status[result.Target.ID] = result.Status
		m.recordCompletedResult(result)
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
		m.Notice = m.completionNotice()
		return nil
	}
	next, cmd := m.startQueuedRuns()
	*m = next
	return cmd
}

func (m *Model) recordCompletedResult(result core.RunResult) {
	for i, existing := range m.completedResults {
		if existing.Target.ID == result.Target.ID {
			m.completedResults[i] = result
			return
		}
	}
	m.completedResults = append(m.completedResults, result)
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
	if m.TargetStarted == nil {
		m.TargetStarted = map[string]time.Time{}
	}
	m.TargetStarted[target.ID] = time.Now()
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
	if width > 0 && width < 80 {
		return m.renderTooSmall(width, m.Height)
	}
	if width == 0 {
		width = 80
	}
	height := m.Height
	if height > 0 && height < 20 {
		return m.renderTooSmall(width, height)
	}
	if height == 0 {
		height = 20
	}
	panelHeight := max(10, height-9)
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
	b.WriteString(m.renderDashboard(width))
	b.WriteByte('\n')
	b.WriteString(m.renderSubHeader(width))
	b.WriteByte('\n')
	panels := m.renderPanelArea(width, panelHeight, leftWidth, rightWidth)
	if overlay := m.renderOverlay(width, panelHeight); overlay != "" {
		b.WriteString(placeOverlay(panels, overlay, width))
	} else {
		b.WriteString(panels)
	}
	if m.RunError != "" {
		b.WriteString("\n")
		b.WriteString(m.renderMessageBar(width, "ERROR", m.RunError, errorBarStyle))
	} else if m.Notice != "" {
		b.WriteString("\n")
		label := "INFO"
		style := noticeBarStyle
		if strings.HasPrefix(m.Notice, "run complete:") && m.completionNeedsAttention() {
			label = "WARN"
			style = warningBarStyle
		}
		b.WriteString(m.renderMessageBar(width, label, m.Notice, style))
	}
	b.WriteByte('\n')
	b.WriteString(m.renderFooter(width))
	return b.String()
}

func (m Model) renderPanelArea(width int, panelHeight int, leftWidth int, rightWidth int) string {
	compact := m.compactMode(width)
	if (m.Zoom || compact) && m.Focus == FocusLogs {
		return strings.Join(m.renderLogPanel(m.singlePanelWidth(width), panelHeight), "\n")
	}
	if m.Zoom || compact {
		return strings.Join(m.renderDirectoryPanel(m.singlePanelWidth(width), panelHeight), "\n")
	}
	left := m.renderDirectoryPanel(leftWidth, panelHeight)
	right := m.renderLogPanel(rightWidth, panelHeight)
	return joinPanels(left, right)
}

func (m Model) renderMessageBar(width int, label string, message string, style lipgloss.Style) string {
	text := fmt.Sprintf(" %s  %s", label, message)
	return style.Render(padRightVisible(truncateVisible(text, width), width))
}

func (m Model) completionNeedsAttention() bool {
	stats := m.statusCounts()
	return stats[core.StatusFailed] > 0 || stats[core.StatusCancelled] > 0
}

func (m Model) renderTooSmall(width int, height int) string {
	if width <= 0 {
		width = 40
	}
	if height <= 0 {
		height = 10
	}
	rows := []string{
		runnyBadgeStyle.Render("runny"),
		"terminal too small",
		fmt.Sprintf("need at least 80x20, got %dx%d", width, height),
		"resize terminal to continue",
	}
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		lines = append(lines, padRightVisible(truncateVisible(" "+row, width), width))
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderOverlay(width int, height int) string {
	title := ""
	var rows []string
	switch {
	case m.ShowHelp:
		title = "Keymap"
		rows = m.helpRows(width)
	case m.ShowHistory:
		title = "History"
		rows = m.historyRows()
	case m.ShowPalette:
		title = "Command palette"
		rows = m.paletteRows()
	case m.ConfirmRun:
		title = "Rerun failed"
		rows = []string{
			fmt.Sprintf("%d failed target(s) will run again.", m.failedCount()),
			"command: " + truncateVisible(m.previewCommandText(), 64),
			"targets: " + m.statusTargetSummary(core.StatusFailed, 64),
			"y/enter confirm   n/esc cancel",
		}
	case m.ConfirmCancelAll:
		title = "Cancel all"
		rows = []string{
			fmt.Sprintf("%d active target(s) will be cancelled.", m.activeCount()),
			fmt.Sprintf("breakdown: %d running, %d queued", m.statusCount(core.StatusRunning), m.statusCount(core.StatusQueued)),
			"scope: running and queued targets only",
			"targets: " + m.activeTargetSummary(64),
			"y/enter confirm   n/esc cancel",
		}
	default:
		return ""
	}
	maxBoxHeight := height
	if len(rows)+2 <= height-4 {
		maxBoxHeight = height - 4
	}
	boxHeight := max(3, min(maxBoxHeight, len(rows)+2))
	return renderFloatingBox(width, title, clipOverlayRows(rows, boxHeight))
}

func (m Model) renderHeader(width int) string {
	segments := []string{
		runnyBadgeStyle.Render("runny"),
		m.dashboardWidget("mode", string(m.mode()), subtleStyle),
		m.dashboardWidget("workers", m.workersLabel(), subtleStyle),
		m.dashboardWidget("targets", fmt.Sprintf("%d", len(m.Targets)), subtleStyle),
	}
	line := " " + strings.Join(segments, subtleStyle.Render(" "))
	return headerStyle.Render(padRightVisible(truncateVisible(line, width), width))
}

func (m Model) renderDashboard(width int) string {
	stats := m.statusCounts()
	active := stats[core.StatusRunning] + stats[core.StatusQueued]
	done, total := m.progressCounts()
	progressWidth := 14
	if width >= 140 {
		progressWidth = 18
	} else if width < 96 {
		progressWidth = 10
	}
	progress := m.progressBar(done, total, progressWidth)
	stateLabel := m.executionState()
	if m.Running {
		stateLabel = fmt.Sprintf("running %d/%d", done, total)
	}
	chips := []string{
		m.dashboardWidget("active", fmt.Sprintf("%d", active), metricRunningStyle),
		m.dashboardWidget("queue", fmt.Sprintf("%d", stats[core.StatusQueued]), metricQueuedStyle),
		m.dashboardWidget("ok", fmt.Sprintf("%d", stats[core.StatusSucceeded]), metricSuccessStyle),
		m.dashboardWidget("failed", fmt.Sprintf("%d", stats[core.StatusFailed]), metricFailedStyle),
		m.dashboardWidget("progress", fmt.Sprintf("%s %d/%d %s", progress, done, total, m.progressPercent(done, total)), subtleStyle),
	}
	if width >= 120 {
		chips = append(chips, m.dashboardWidget("idle", fmt.Sprintf("%d", stats[core.StatusIdle]), metricIdleStyle))
	}
	if m.Running || width >= 100 {
		chips = append([]string{m.dashboardWidget("state", stateLabel, subtleStyle)}, chips...)
	}
	line := strings.Join(chips, subtleStyle.Render(" "))
	return padRightVisible(truncateVisible(line, width), width)
}

func (m Model) progressCounts() (int, int) {
	doneStatuses := map[core.Status]bool{
		core.StatusSucceeded: true,
		core.StatusFailed:    true,
		core.StatusCancelled: true,
	}
	if m.Running || len(m.completedResults) > 0 {
		completed := map[string]bool{}
		for _, result := range m.completedResults {
			if doneStatuses[result.Status] {
				completed[result.Target.ID] = true
			}
		}
		if len(completed) == 0 {
			for _, target := range m.Targets {
				if target.Selected && doneStatuses[m.Status[target.ID]] {
					completed[target.ID] = true
				}
			}
		}
		active := 0
		for _, status := range m.Status {
			if status == core.StatusRunning || status == core.StatusQueued {
				active++
			}
		}
		total := len(completed) + active
		if total > 0 {
			return len(completed), total
		}
	}

	done := 0
	total := 0
	for _, target := range m.Targets {
		if !target.Selected {
			continue
		}
		total++
		if doneStatuses[m.Status[target.ID]] {
			done++
		}
	}
	return done, total
}

func (m Model) dashboardWidget(label string, value string, valueStyle lipgloss.Style) string {
	valueStyle = valueStyle.Background(runnyTheme.bgElevated)
	text := " " + dashboardLabelStyle.Render(label+" ") + valueStyle.Render(value) + " "
	return dashboardWidgetStyle.Render(text)
}

func (m Model) renderSubHeader(width int) string {
	return strings.Join(m.commandInputBoxLines(width), "\n")
}

func (m Model) commandInputBoxLines(width int) []string {
	title := " " + commandInputTitleStyle.Render(m.commandInputTitle()) + " "
	width = max(width, lipgloss.Width(title)+2)
	contentWidth := max(0, width-4)
	topFill := max(0, width-lipgloss.Width(title)-2)
	value := commandInputStyle.Render(m.commandInputValue())
	value = padRightVisible(truncateVisible(value, contentWidth), contentWidth)

	return []string{
		commandInputBorderStyle.Render("┌") + title + commandInputBorderStyle.Render(strings.Repeat("─", topFill)+"┐"),
		commandInputBorderStyle.Render("│") + " " + value + " " + commandInputBorderStyle.Render("│"),
		commandInputBorderStyle.Render("└" + strings.Repeat("─", max(0, width-2)) + "┘"),
	}
}

func (m Model) commandInputTitle() string {
	if m.ShowPalette {
		return "Palette"
	}
	if m.Focus == FocusFilter {
		return "Filter"
	}
	return "Command"
}

func (m Model) commandInputValue() string {
	if m.ShowPalette {
		palette := strings.TrimSpace(m.Palette)
		if palette == "" {
			palette = "<command>"
		}
		return ": " + palette
	}
	if m.Focus == FocusFilter {
		filterText := m.Filter
		if filterText == "" {
			filterText = "<filter>"
		}
		return "/ " + filterText + "▌"
	}
	if m.Focus == FocusCommand {
		return m.Command + "▌"
	}
	return strings.TrimSpace(m.Command)
}

func (m Model) focusedPathLabel() string {
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) {
		return ""
	}
	path := strings.TrimSpace(m.Targets[m.Cursor].RelPath)
	if path == "" {
		return ""
	}
	return "path " + strings.ReplaceAll(path, "/", " › ")
}

func (m Model) mode() core.ExecutionMode {
	if m.Mode == "" {
		return core.ModeParallel
	}
	return m.Mode
}

func (m Model) workersLabel() string {
	if m.Workers > 0 {
		return fmt.Sprintf("%d", m.Workers)
	}
	return "auto"
}

func (m Model) executionConfigLabel() string {
	return fmt.Sprintf("%s · workers %s", m.mode(), m.workersLabel())
}

func (m Model) renderDirectoryPanel(width int, height int) []string {
	rows := []string{panelTitleStyle.Render(m.taskHeader(width - 4))}
	visibleIndexes := m.visibleTargetIndexes()
	limit := max(1, height-4)
	if len(visibleIndexes) > 0 {
		limit = max(1, height-5)
	}
	offset := m.DirectoryOffset
	if offset < 0 {
		offset = 0
	}
	if offset > max(0, len(visibleIndexes)-1) {
		offset = max(0, len(visibleIndexes)-1)
	}
	if len(visibleIndexes) > 0 {
		rows = append(rows, subtleStyle.Render(m.directoryScrollLabel(offset, limit, len(visibleIndexes))))
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
		if m.Filter == "" {
			rows = append(rows, sectionStyle.Render("No target directories found"))
			rows = append(rows, "  runny executes inside child directories of current cwd")
			rows = append(rows, "  create directories or run from a project root")
		} else {
			rows = append(rows, sectionStyle.Render("No matches for /"+m.Filter))
			rows = append(rows, "  "+m.filterModeLabel()+" filter has no visible target")
			rows = append(rows, "  edit query, ctrl+u clears filter, esc returns to tasks")
		}
	}
	return boxLines(width, height, "Tasks", rows, m.Focus == FocusTargets)
}

func (m Model) filterModeLabel() string {
	_, exact := filterQuery(strings.TrimSpace(m.Filter))
	if exact {
		return "exact"
	}
	return "fuzzy"
}

func (m Model) taskHeader(width int) string {
	left := "DIRECTORY"
	return fixedStatusJoin(left, statusHeaderStyle.Render("STATUS"), width)
}

func (m Model) directoryScrollLabel(offset int, limit int, total int) string {
	end := min(total, offset+limit)
	markers := ""
	if offset > 0 {
		markers += "↑"
	}
	if end < total {
		markers += "↓"
	}
	if markers == "" {
		markers = "•"
	}
	return fmt.Sprintf("showing %d-%d of %d %s", offset+1, end, total, markers)
}

func (m Model) renderTargetRow(index int, target core.Target, width int) string {
	active := index == m.Cursor
	status := m.Status[target.ID]
	partial := !target.Selected && m.targetHasSelectedDescendant(target)
	activity := " "
	switch status {
	case core.StatusQueued:
		activity = targetRowInlineStyle(subtleStyle, status).Render("…")
	case core.StatusRunning:
		activity = targetRowInlineStyle(metricRunningStyle, status).Render("▶")
	case core.StatusCancelled:
		activity = targetRowInlineStyle(subtleStyle, status).Render("×")
	case core.StatusFailed:
		activity = targetRowInlineStyle(metricFailedStyle, status).Render("!")
	}
	fold := " "
	if len(target.Children) > 0 && target.Folded && m.Filter == "" {
		fold = "+"
	} else if len(target.Children) > 0 {
		fold = "−"
	}
	name := m.renderTargetName(target)
	statusText := m.renderRowStatus(status, active)
	if active || target.Selected || partial {
		activity = m.activitySymbol(status)
		fold = m.foldSymbol(target)
		name = m.renderTargetNamePlain(target)
		statusText = padRightVisible(m.statusLabel(status), 12)
	}
	left := "  " + activity + " " + fold + "  " + name
	row := fixedStatusJoin(left, statusText, width)
	if active {
		if target.Selected || partial {
			return rowActiveSelectedStyle.Render(padRightVisible(row, width))
		}
		return rowActiveStyle.Render(padRightVisible(row, width))
	}
	if target.Selected {
		return rowSelectedStyle.Render(padRightVisible(row, width))
	}
	if partial {
		return rowPartialStyle.Render(padRightVisible(row, width))
	}
	if status == core.StatusRunning {
		return rowRunningStyle.Render(padRightVisible(row, width))
	}
	return row
}

func targetRowInlineStyle(style lipgloss.Style, status core.Status) lipgloss.Style {
	return style
}

func (m Model) activitySymbol(status core.Status) string {
	switch status {
	case core.StatusQueued:
		return "…"
	case core.StatusRunning:
		return "▶"
	case core.StatusCancelled:
		return "×"
	case core.StatusFailed:
		return "!"
	default:
		return " "
	}
}

func (m Model) foldSymbol(target core.Target) string {
	if len(target.Children) > 0 && target.Folded && m.Filter == "" {
		return "+"
	}
	if len(target.Children) > 0 {
		return "−"
	}
	return " "
}

func (m Model) renderRowStatus(status core.Status, active bool) string {
	label := padRightVisible(m.statusLabel(status), 12)
	if active {
		return lipgloss.NewStyle().Foreground(runnyTheme.fgInverse).Background(runnyTheme.bgFocus).Bold(true).Render(label)
	}
	if style, ok := statusStyles[status]; ok {
		style = targetRowInlineStyle(style, status)
		return style.Render(label)
	}
	return label
}

func (m Model) renderTargetName(target core.Target) string {
	guide := m.treeGuide(target)
	icon := m.folderIcon(target)
	name := targetName(target)
	if target.Name != "" {
		name = target.Name
	}
	return guide + folderIconStyle.Render(icon) + " " + m.renderTargetDisplayName(name)
}

func (m Model) renderTargetNamePlain(target core.Target) string {
	name := targetName(target)
	if target.Name != "" {
		name = target.Name
	}
	return m.treeGuidePlain(target) + m.folderIcon(target) + " " + name
}

func (m Model) folderIcon(target core.Target) string {
	if len(target.Children) > 0 {
		if target.Folded && m.Filter == "" {
			return "📁"
		}
		return "📂"
	}
	return "📁"
}

func (m Model) treeGuide(target core.Target) string {
	return treeGuideStyle.Render(m.treeGuidePlain(target))
}

func (m Model) treeGuidePlain(target core.Target) string {
	if target.Depth <= 1 || target.ParentID == "" {
		return ""
	}
	ancestors := m.targetAncestors(target)
	var b strings.Builder
	for _, ancestor := range ancestors[min(1, len(ancestors)):] {
		if m.isLastChild(ancestor) {
			b.WriteString("  ")
		} else {
			b.WriteString("│ ")
		}
	}
	branch := "└─ "
	if !m.isLastChild(target) {
		branch = "├─ "
	}
	b.WriteString(branch)
	return b.String()
}

func (m Model) targetAncestors(target core.Target) []core.Target {
	var reversed []core.Target
	seen := map[string]bool{}
	parentID := target.ParentID
	for parentID != "" && !seen[parentID] {
		seen[parentID] = true
		parent, ok := m.targetByID(parentID)
		if !ok {
			break
		}
		reversed = append(reversed, parent)
		parentID = parent.ParentID
	}
	ancestors := make([]core.Target, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		ancestors = append(ancestors, reversed[i])
	}
	return ancestors
}

func (m Model) targetByID(id string) (core.Target, bool) {
	for _, target := range m.Targets {
		if target.ID == id {
			return target, true
		}
	}
	return core.Target{}, false
}

func (m Model) targetIndexByID(id string) (int, bool) {
	for i, target := range m.Targets {
		if target.ID == id {
			return i, true
		}
	}
	return 0, false
}

func (m Model) targetSubtreeIndexes(index int) []int {
	if index < 0 || index >= len(m.Targets) {
		return nil
	}
	indexes := []int{index}
	seen := map[string]bool{m.Targets[index].ID: true}
	var walk func(core.Target)
	walk = func(target core.Target) {
		for _, childID := range target.Children {
			if seen[childID] {
				continue
			}
			childIndex, ok := m.targetIndexByID(childID)
			if !ok {
				continue
			}
			seen[childID] = true
			indexes = append(indexes, childIndex)
			walk(m.Targets[childIndex])
		}
	}
	walk(m.Targets[index])
	return indexes
}

func (m Model) targetHasSelectedDescendant(target core.Target) bool {
	for _, childID := range target.Children {
		childIndex, ok := m.targetIndexByID(childID)
		if !ok {
			continue
		}
		child := m.Targets[childIndex]
		if child.Selected || m.targetHasSelectedDescendant(child) {
			return true
		}
	}
	return false
}

func (m Model) renderTargetDisplayName(name string) string {
	if strings.TrimSpace(m.Filter) != "" {
		return folderNameStyle.Render(m.highlightMatch(name))
	}
	parent, base := splitTargetPath(name)
	if parent == "" {
		return folderNameStyle.Render(base)
	}
	return folderPathStyle.Render(parent+"/") + folderNameStyle.Render(base)
}

func (m Model) isLastChild(target core.Target) bool {
	if target.ParentID == "" {
		return true
	}
	for _, candidate := range m.Targets {
		if candidate.ID != target.ParentID {
			continue
		}
		if len(candidate.Children) == 0 {
			return true
		}
		return candidate.Children[len(candidate.Children)-1] == target.ID
	}
	return true
}

func targetName(target core.Target) string {
	value := strings.Trim(target.RelPath, "/")
	if value == "" {
		return target.ID
	}
	return value
}

func splitTargetPath(value string) (string, string) {
	value = strings.Trim(value, "/")
	index := strings.LastIndex(value, "/")
	if index < 0 {
		return "", value
	}
	return value[:index], value[index+1:]
}

func (m Model) renderLogPanel(width int, height int) []string {
	lines := []string{}
	if len(m.Targets) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Targets) {
		target := m.Targets[m.Cursor]
		lines = append(lines, m.renderOutputLines(target.ID, height)...)
	}
	return boxLines(width, height, "Output", lines, m.Focus == FocusLogs)
}

func (m Model) previewCommandText() string {
	command := strings.TrimSpace(m.Command)
	if command == "" {
		return subtleStyle.Render("(not set)")
	}
	return command
}

func (m Model) previewCommandLine(target core.Target) string {
	command := strings.TrimSpace(m.Command)
	if command == "" {
		return subtleStyle.Render("--:--:--  (not set)")
	}
	started := m.TargetStarted[target.ID]
	if started.IsZero() {
		return commandDisplayStyle.Render(" --:--:--  " + command + " ")
	}
	return commandDisplayStyle.Render(" " + started.Format("15:04:05") + "  " + command + " ")
}

func (m Model) previewNextAction(target core.Target, status core.Status, width int) string {
	if strings.TrimSpace(m.Command) == "" {
		return "c edit command   ? keymap"
	}
	switch status {
	case core.StatusQueued, core.StatusRunning:
		if width < 30 {
			return "del/x target  ctrl+c all"
		}
		return "del/x target   ctrl+c cancel+quit"
	case core.StatusFailed:
		return "R rerun failed   enter run selected"
	}
	if target.Selected {
		return "enter run selected   space deselect"
	}
	return "space select   a " + m.bulkSelectionLabel()
}

func (m Model) outputRangeLabel(targetID string, height int) string {
	output := strings.Split(strings.TrimRight(m.Logs[targetID], "\n"), "\n")
	if len(output) == 1 && output[0] == "" {
		return "(empty)"
	}
	visible := max(1, height-14)
	offset := m.previewOutputOffset(len(output), visible)
	end := min(len(output), offset+visible)
	markers := ""
	if offset > 0 {
		markers += "↑"
	}
	if end < len(output) {
		markers += "↓"
	}
	if markers == "" {
		markers = "•"
	}
	mode := "manual"
	if m.LogFollow {
		mode = "tail"
	}
	return fmt.Sprintf("(%d-%d/%d %s %s)", offset+1, end, len(output), mode, markers)
}

func (m Model) renderOutputLines(targetID string, height int) []string {
	output := strings.Split(strings.TrimRight(m.Logs[targetID], "\n"), "\n")
	if len(output) == 1 && output[0] == "" {
		return nil
	}
	visible := max(1, height-2)
	offset := m.previewOutputOffset(len(output), visible)
	rows := make([]string, 0, visible)
	for i, line := range output[offset:] {
		if i >= visible {
			break
		}
		rows = append(rows, m.styleLogLine(line))
	}
	return rows
}

func (m Model) previewOutputOffset(total int, visible int) int {
	maxOffset := max(0, total-visible)
	if m.LogFollow {
		return maxOffset
	}
	if m.PreviewOffset > maxOffset {
		return maxOffset
	}
	if m.PreviewOffset < 0 {
		return 0
	}
	return m.PreviewOffset
}

func (m Model) previewScrollLabel(targetID string, height int) string {
	output := strings.Split(strings.TrimRight(m.Logs[targetID], "\n"), "\n")
	if len(output) == 1 && output[0] == "" {
		output = nil
	}
	if len(output) == 0 {
		if m.LogFollow {
			return "tail"
		}
		return "manual"
	}
	visible := max(1, height-14)
	maxOffset := max(0, len(output)-visible)
	offset := m.PreviewOffset
	if m.LogFollow {
		offset = maxOffset
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	return fmt.Sprintf("%d/%d %s", offset+1, maxOffset+1, ternary(m.LogFollow, "tail", "manual"))
}

func (m Model) styleLogLine(line string) string {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "exit status") {
		return logErrorStyle.Render(line)
	}
	return logInfoStyle.Render(line)
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

func (m Model) renderFooter(width int) string {
	activeStopHint := "ctrl+c all+quit"
	if width >= 110 {
		activeStopHint = "ctrl+c cancel+quit"
	}
	zoomLabel := "z zoom"
	if m.Zoom {
		zoomLabel = "z split"
	}
	globalKeys := []string{"? help", "/ search", ": cmd", "H hist", zoomLabel}
	contextKeys := []string{}
	statusKeys := []string{}
	if width < 110 && m.hasActiveRuns() {
		globalKeys = []string{"? help", "/ search", ": cmd", "H hist"}
	}
	if width >= 110 {
		globalKeys = []string{"? keymap", "/ search", ": command", "H history", "tab panels", zoomLabel}
	}
	if m.ShowHelp {
		globalKeys = []string{"? close", "q close", "esc close", "H history"}
	} else if m.ShowHistory {
		globalKeys = []string{"/ search", "? keymap"}
		contextKeys = []string{"enter reuse", "up/down choose", "ctrl+u clear", "esc close"}
		if m.selectableHistoryLen() == 0 {
			contextKeys = []string{"no reuse", "ctrl+u clear", "esc close"}
		}
	} else if m.ShowPalette {
		globalKeys = []string{": command", "? keymap"}
		contextKeys = []string{"enter choose", "up/down choose", "ctrl+u clear", "esc close"}
	} else if m.ConfirmRun {
		globalKeys = []string{"? keymap"}
		contextKeys = []string{"y confirm", "enter confirm", "n cancel", "esc cancel"}
	} else if m.ConfirmCancelAll {
		globalKeys = []string{"? keymap"}
		contextKeys = []string{"y confirm", "enter confirm", "n cancel", "esc cancel"}
	}
	if m.hasActiveRuns() {
		statusKeys = append(statusKeys, activeStopHint)
	}
	if !m.ShowHelp && !m.ShowHistory && !m.ShowPalette && !m.ConfirmRun && !m.ConfirmCancelAll {
		switch m.Focus {
		case FocusCommand:
			if width >= 110 {
				contextKeys = []string{"enter run", "esc tasks", "up/down hist", "backspace edit"}
			} else {
				globalKeys = []string{"? help", "H hist"}
				contextKeys = []string{"enter run", "esc tasks", "↑↓ hist", "ctrl+u clear"}
			}
		case FocusFilter:
			if width >= 110 {
				globalKeys = []string{"? keymap", "H history"}
				if m.hasActiveRuns() {
					statusKeys = append(statusKeys, activeStopHint)
				}
				contextKeys = []string{"type fuzzy", "' exact", "↑↓/nN matches", "ctrl+u clear", "enter/esc tasks"}
			} else {
				globalKeys = []string{"? help", "H hist"}
				contextKeys = []string{"type fuzzy", "' exact", "↑↓/nN", "ctrl+u clear", "enter/esc"}
			}
		case FocusLogs:
			if width >= 110 {
				contextKeys = []string{"pgup/pgdn scroll", "f tail", "tab tasks"}
			} else {
				globalKeys = []string{"? help", "H hist"}
				contextKeys = []string{"pgup/pgdn scroll", "f tail", "tab tasks"}
			}
		default:
			if width >= 110 {
				contextKeys = []string{"space select", "a Select All", "←/→ fold", "enter run", "del/x cancel"}
			} else {
				globalKeys = []string{"? help", "H hist"}
				contextKeys = []string{"space select", "tab output", ": cmd", "enter run", "del/x cancel"}
			}
			if !m.hasActiveRuns() && m.failedCount() > 0 {
				statusKeys = append(statusKeys, "R failed")
			}
		}
	}
	if !m.hasActiveRuns() {
		statusKeys = append(statusKeys, "q quit")
	}
	hints := make([]string, 0, len(globalKeys)+len(contextKeys)+len(statusKeys))
	hints = append(hints, globalKeys...)
	hints = append(hints, contextKeys...)
	hints = append(hints, statusKeys...)
	return renderFooterHintGrid(hints, width, 3)
}

type footerHint struct {
	key   string
	label string
}

func renderFooterHintGrid(rawHints []string, width int, rows int) string {
	if rows <= 0 {
		rows = 1
	}
	hints := make([]footerHint, 0, len(rawHints))
	for _, hint := range rawHints {
		hints = append(hints, parseFooterHint(hint))
	}
	if len(hints) == 0 {
		return strings.Repeat(" ", width)
	}
	keyWidth := 0
	labelWidth := 0
	for _, hint := range hints {
		keyWidth = max(keyWidth, lipgloss.Width(hint.key))
		labelWidth = max(labelWidth, lipgloss.Width(hint.label))
	}
	columns := (len(hints) + rows - 1) / rows
	cellWidth := keyWidth + 1 + labelWidth
	totalWidth := 1 + columns*cellWidth + max(0, columns-1)*2
	if totalWidth > width && columns > 1 {
		cellWidth = max(1, (width-1-max(0, columns-1)*2)/columns)
	}
	lines := make([]string, 0, rows)
	for row := 0; row < rows; row++ {
		cells := make([]footerHint, 0, columns)
		for column := 0; column < columns; column++ {
			index := column*rows + row
			if index >= len(hints) {
				continue
			}
			cells = append(cells, hints[index])
		}
		lines = append(lines, styleFooterHintCells(cells, width, keyWidth, labelWidth, cellWidth))
	}
	return strings.Join(lines, "\n")
}

func parseFooterHint(hint string) footerHint {
	key, label, ok := strings.Cut(hint, " ")
	if !ok {
		return footerHint{key: hint}
	}
	if key == "no" || key == "type" {
		return footerHint{label: titleFooterLabel(hint)}
	}
	return footerHint{key: key, label: titleFooterLabel(label)}
}

func titleFooterLabel(label string) string {
	if label == "" {
		return label
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func styleFooterHintCells(cells []footerHint, width int, keyWidth int, labelWidth int, cellWidth int) string {
	if len(cells) == 0 {
		return strings.Repeat(" ", width)
	}
	if noColorEnabled() {
		return plainFooterHintCells(cells, width, keyWidth, labelWidth, cellWidth)
	}
	labelCellWidth := max(0, cellWidth-keyWidth-1)
	var b strings.Builder
	background := ansiBackgroundHex(footerBackgroundHex)
	labelStyle := ansiForegroundHex(footerLabelHex)
	shortcutStyle := ansiForegroundHex(footerShortcutHex)
	b.WriteString(background)
	b.WriteString(labelStyle)
	b.WriteByte(' ')
	for i, cell := range cells {
		if i > 0 {
			b.WriteString("  ")
		}
		if cell.key != "" {
			b.WriteString(shortcutStyle)
			b.WriteString("\x1b[1m")
			b.WriteString(padRightVisible(cell.key, keyWidth))
			b.WriteString("\x1b[22m")
		} else {
			b.WriteString(strings.Repeat(" ", keyWidth))
		}
		b.WriteString(labelStyle)
		b.WriteByte(' ')
		label := truncateVisible(cell.label, labelCellWidth)
		b.WriteString(padRightVisible(label, labelCellWidth))
	}
	missing := width - lipgloss.Width(stripANSIForWidth(b.String()))
	if missing > 0 {
		b.WriteString(strings.Repeat(" ", missing))
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

func plainFooterHintCells(cells []footerHint, width int, keyWidth int, labelWidth int, cellWidth int) string {
	labelCellWidth := max(0, cellWidth-keyWidth-1)
	var b strings.Builder
	b.WriteByte(' ')
	for i, cell := range cells {
		if i > 0 {
			b.WriteString("  ")
		}
		label := truncateVisible(cell.label, labelCellWidth)
		text := padRightVisible(cell.key, keyWidth) + " " + padRightVisible(label, labelCellWidth)
		b.WriteString(truncateVisible(text, cellWidth))
	}
	return padRightVisible(truncateVisible(b.String(), width), width)
}

func stripANSIForWidth(value string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(value); i++ {
		char := value[i]
		if inEscape {
			if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
				inEscape = false
			}
			continue
		}
		if char == '\x1b' {
			inEscape = true
			continue
		}
		b.WriteByte(char)
	}
	return b.String()
}

func (m Model) compactMode(width int) bool {
	return width < 96
}

func (m Model) singlePanelWidth(width int) int {
	if width <= 80 {
		return width - 1
	}
	return width
}

func (m Model) highlightMatch(value string) string {
	if m.Filter == "" {
		return value
	}
	query, exact := filterQuery(m.Filter)
	if query == "" {
		return value
	}
	index := indexFold(value, query)
	if index < 0 {
		if exact {
			return value
		}
		return highlightFuzzyMatch(value, query)
	}
	before := value[:index]
	match := value[index : index+len(query)]
	after := value[index+len(query):]
	return before + matchStyle.Render(match) + after
}

func highlightFuzzyMatch(value string, query string) string {
	indexes := fuzzyIndexesFold(value, query)
	if len(indexes) == 0 {
		return value
	}
	matched := make(map[int]struct{}, len(indexes))
	for _, index := range indexes {
		matched[index] = struct{}{}
	}
	var b strings.Builder
	for index, r := range value {
		if _, ok := matched[index]; ok {
			b.WriteString(matchStyle.Render(string(r)))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (m Model) focusName() string {
	switch m.Focus {
	case FocusCommand:
		return "command"
	case FocusFilter:
		return "filter"
	case FocusLogs:
		return "output"
	default:
		return "tasks"
	}
}

func (m Model) selectedCount() int {
	selected := 0
	for _, target := range m.Targets {
		if target.Selected {
			selected++
		}
	}
	return selected
}

func (m Model) executionState() string {
	if m.Running {
		return "running"
	}
	if m.failedCount() > 0 {
		return "failed"
	}
	stats := m.statusCounts()
	if len(m.Targets) > 0 && stats[core.StatusSucceeded] == len(m.Targets) {
		return "succeeded"
	}
	return "ready"
}

func (m Model) statusLabel(status core.Status) string {
	if status == "" {
		status = core.StatusIdle
	}
	switch status {
	case core.StatusIdle:
		return "○ idle"
	case core.StatusQueued:
		return "◌ queued"
	case core.StatusRunning:
		return "● running"
	case core.StatusSucceeded:
		return "✓ ok"
	case core.StatusFailed:
		return "✕ failed"
	case core.StatusCancelled:
		return "– cancelled"
	case core.StatusSkipped:
		return "○ skipped"
	default:
		return string(status)
	}
}

func (m Model) progressBar(done int, total int, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat("░", max(0, width))
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	filled := done * width / total
	if done > 0 && filled == 0 {
		filled = 1
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (m Model) progressPercent(done int, total int) string {
	if total <= 0 {
		return "0%"
	}
	if done < 0 {
		done = 0
	}
	if done > total {
		done = total
	}
	return fmt.Sprintf("%d%%", done*100/total)
}

func (m Model) helpRows(width ...int) []string {
	columns := 3
	if len(width) > 0 && width[0] < 96 {
		columns = 2
	}
	sections := []helpSection{
		{
			id:    "global",
			title: "Global",
			bindings: []helpBinding{
				{"?", "keymap"}, {"/", "filter"}, {":", "palette"}, {"H", "history"},
				{"tab", "tasks/output"}, {"z", "zoom/split"}, {"esc", "close mode"}, {"q", "quit idle"},
				{"ctrl+c", "cancel + quit"},
			},
		},
		{
			id:    "input",
			title: "Input and filter",
			bindings: []helpBinding{
				{"c", "edit command"}, {"type", "insert text"}, {"backspace", "edit"},
				{"up/down", "command history"}, {"ctrl+w", "word back"}, {"ctrl+u", "clear"},
				{"/", "fuzzy filter"}, {"'", "exact filter"}, {"n/N", "next/prev match"},
			},
		},
		{
			id:    "tasks",
			title: "Tasks",
			bindings: []helpBinding{
				{"up/down", "move"}, {"j/k", "move"}, {"g/G", "first/last"},
				{"space", "toggle select tree"}, {"a", "toggle visible/matches"},
				{"left/h", "fold"}, {"right/l", "unfold"}, {"enter", "run selected"},
				{"del/x", "cancel selected"},
			},
		},
		{
			id:    "runs",
			title: "Runs and status",
			bindings: []helpBinding{
				{"R", "rerun failed"}, {"▶", "running"}, {"…", "queued"}, {"✓", "ok"},
				{"!", "failed"}, {"×", "cancelled"},
			},
		},
		{
			id:    "palette",
			title: "Palette",
			bindings: []helpBinding{
				{":run", "run"}, {":workers N|auto", "workers"}, {":serial", "serial"},
				{":parallel", "parallel"}, {":failed", "select failed"},
				{":rerun-failed", "rerun failed"}, {":cancel", "cancel target"},
				{":cancel-all", "cancel active"}, {":history", "history"},
				{"type", "fuzzy"}, {"'", "exact"}, {"enter", "choose"}, {"esc", "close"},
			},
		},
		{
			id:    "history",
			title: "History",
			bindings: []helpBinding{
				{"H", "open history"}, {"/", "search runs"}, {"'", "exact match"},
				{"up/down", "choose"}, {"enter", "reuse command"}, {"ctrl+u", "clear"},
				{"esc", "close"},
			},
		},
		{
			id:    "preview",
			title: "Output",
			bindings: []helpBinding{
				{"pageup", "scroll up"}, {"ctrl+b", "scroll up"},
				{"pagedown", "scroll down"}, {"ctrl+f", "scroll down"},
				{"ctrl+u", "half up"}, {"ctrl+d", "half down"}, {"f", "tail"},
			},
		},
	}
	activeID := m.activeHelpSectionID()
	ordered := make([]helpSection, 0, len(sections))
	for _, section := range sections {
		if section.id == activeID {
			ordered = append(ordered, section)
			break
		}
	}
	for _, section := range sections {
		if section.id != activeID {
			ordered = append(ordered, section)
		}
	}
	rows := make([]string, 0, 24)
	for i, section := range ordered {
		if i > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, sectionStyle.Render(section.title))
		rows = append(rows, formatHelpBindings(section.bindings, columns)...)
	}
	return rows
}

func formatHelpBindings(bindings []helpBinding, columns int) []string {
	if columns < 1 {
		columns = 1
	}
	rows := make([]string, 0, (len(bindings)+columns-1)/columns)
	cellWidth := 28
	for i := 0; i < len(bindings); i += columns {
		cells := make([]string, 0, columns)
		for j := 0; j < columns && i+j < len(bindings); j++ {
			binding := bindings[i+j]
			cell := helpKeyStyle.Render(binding.key) + " " + helpDescStyle.Render(binding.description)
			cells = append(cells, padRightVisible(cell, cellWidth))
		}
		rows = append(rows, "  "+strings.Join(cells, " "))
	}
	return rows
}

type helpSection struct {
	id       string
	title    string
	bindings []helpBinding
}

type helpBinding struct {
	key         string
	description string
}

func (m Model) activeHelpSectionID() string {
	switch {
	case m.ShowPalette:
		return "palette"
	case m.ShowHistory:
		return "history"
	case m.ConfirmRun || m.ConfirmCancelAll || m.hasActiveRuns():
		return "runs"
	case m.Focus == FocusCommand || m.Focus == FocusFilter:
		return "input"
	case m.Focus == FocusLogs:
		return "preview"
	default:
		return "tasks"
	}
}

func (m Model) paletteRows() []string {
	input := m.Palette
	if input == "" {
		input = "<type command>"
	}
	matches := m.filteredPaletteCommands()
	helpText := "0 matches   ctrl+u clear   esc close"
	if len(matches) > 0 {
		selected := matches[min(m.PalettePos, len(matches)-1)].Name
		if strings.Contains(selected, " ") {
			selected = "command"
		}
		helpText = fmt.Sprintf("%d fuzzy match(es)   enter %s   esc close   ↑↓ choose", len(matches), selected)
	}
	rows := []string{
		commandPromptStyle.Render(" : " + input + " "),
		subtleStyle.Render(helpText),
		"",
	}
	if len(matches) == 0 {
		return append(rows, "  no commands; backspace or ctrl+u to edit")
	}
	visibleLimit := 8
	for i, command := range matches {
		if i >= visibleLimit {
			break
		}
		prefix := "  "
		if i == m.PalettePos {
			prefix = "› "
		}
		name := m.highlightPaletteMatch(command.Name)
		description := m.highlightPaletteMatch(command.Description)
		line := prefix + padRightVisible(name, 16) + description
		if i == m.PalettePos {
			line = paletteActiveStyle.Render(padRightVisible(line, 72))
		}
		rows = append(rows, line)
	}
	if hidden := len(matches) - min(len(matches), visibleLimit); hidden > 0 {
		rows = append(rows, subtleStyle.Render(fmt.Sprintf("  ... %d more command(s); keep typing to narrow", hidden)))
	}
	return rows
}

func (m Model) highlightPaletteMatch(value string) string {
	query := strings.TrimSpace(m.Palette)
	if query == "" {
		return value
	}
	query, exact := filterQuery(query)
	if query == "" {
		return value
	}
	index := indexFold(value, query)
	if index >= 0 {
		before := value[:index]
		match := value[index : index+len(query)]
		after := value[index+len(query):]
		return before + matchStyle.Render(match) + after
	}
	if exact {
		return value
	}
	return highlightFuzzyMatch(value, query)
}

func (m Model) historyRows() []string {
	commands := m.visibleHistoryCommands()
	commandTotal := m.filteredHistoryCommandCount()
	commandTitle := fmt.Sprintf("Commands (%d/%d)", len(commands), commandTotal)
	prompt := "/ " + m.HistoryFilter
	if m.HistorySearching {
		prompt += "▌"
	}
	searchHelp := subtleStyle.Render(" / search   ' exact   ctrl+u clear")
	rows := []string{
		commandPromptStyle.Render(" "+prompt+" ") + " " + searchHelp,
		sectionStyle.Render(commandTitle),
		subtleStyle.Render("  #  command"),
	}
	if len(m.History) == 0 {
		rows = append(rows, "  No command history yet.")
	} else if len(commands) == 0 {
		rows = append(rows, "  No command matches.")
	} else {
		for i, command := range commands {
			prefix := fmt.Sprintf("  %d  ", i+1)
			if i == m.HistoryPos {
				prefix = fmt.Sprintf("› %d  ", i+1)
			}
			line := prefix + truncateVisible(m.highlightHistoryMatch(command), 72)
			if i == m.HistoryPos {
				line = paletteActiveStyle.Render(padRightVisible(line, 76))
			}
			rows = append(rows, line)
		}
		if hidden := commandTotal - len(commands); hidden > 0 {
			rows = append(rows, subtleStyle.Render(fmt.Sprintf("  ... %d more command(s); search to narrow", hidden)))
		}
	}
	runs := m.visibleRunHistory()
	runTotal := m.filteredRunHistoryCount()
	runTitle := fmt.Sprintf("Project runs (%d/%d)", len(runs), runTotal)
	rows = append(rows, "", sectionStyle.Render(runTitle), subtleStyle.Render("  when     result     total  ok  fail  cancel  command"))
	if len(m.RunHistory) == 0 {
		rows = append(rows, "  No project runs yet.")
	} else if len(runs) == 0 {
		rows = append(rows, "  No project runs match.")
	} else {
		for i, run := range runs {
			when := formatHistoryTime(run.Time)
			result := padRightVisible(m.runHistoryOutcome(run), 8)
			line := fmt.Sprintf("  %-8s %s %5d %3d %5d %7d  %s", when, result, run.Total, run.Succeeded, run.Failed, run.Cancelled, truncateVisible(m.highlightHistoryMatch(run.Command), 26))
			if m.HistoryPos == len(commands)+i {
				line = paletteActiveStyle.Render(padRightVisible("›"+line[1:], 76))
			}
			rows = append(rows, line)
		}
		if hidden := runTotal - len(runs); hidden > 0 {
			rows = append(rows, subtleStyle.Render(fmt.Sprintf("  ... %d more project run(s); search to narrow", hidden)))
		}
	}
	footer := "/ search   enter reuse selected command   up/down choose   esc close"
	if len(commands)+len(runs) == 0 {
		footer = "/ search   no command to reuse   esc close"
	}
	rows = append(rows, "", subtleStyle.Render(footer))
	return rows
}

func (m Model) runHistoryOutcome(run history.RunEntry) string {
	switch {
	case run.Failed > 0:
		return metricFailedStyle.Render("failed")
	case run.Cancelled > 0:
		return statusStyles[core.StatusCancelled].Render("cancelled")
	case run.Total > 0 && run.Succeeded == run.Total:
		return metricSuccessStyle.Render("ok")
	case run.Succeeded > 0:
		return metricSuccessStyle.Render("partial")
	default:
		return subtleStyle.Render("unknown")
	}
}

func (m Model) highlightHistoryMatch(value string) string {
	query := strings.TrimSpace(m.HistoryFilter)
	if query == "" {
		return value
	}
	query, exact := filterQuery(query)
	if query == "" {
		return value
	}
	index := indexFold(value, query)
	if index >= 0 {
		before := value[:index]
		match := value[index : index+len(query)]
		after := value[index+len(query):]
		return before + matchStyle.Render(match) + after
	}
	if exact {
		return value
	}
	return highlightFuzzyMatch(value, query)
}

func formatHistoryTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	now := time.Now()
	if value.After(now) {
		return "now"
	}
	delta := now.Sub(value)
	switch {
	case delta < time.Minute:
		return "now"
	case delta < time.Hour:
		return fmt.Sprintf("%dm", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh", int(delta.Hours()))
	default:
		return value.Format("Jan02")
	}
}

func (m Model) selectableHistoryLen() int {
	return len(m.visibleHistoryCommands()) + len(m.visibleRunHistory())
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

func (m Model) selectedHistoryCommand() string {
	pos := m.clampHistoryPos()
	commands := m.visibleHistoryCommands()
	if pos < len(commands) {
		return commands[pos]
	}
	runs := m.visibleRunHistory()
	runIndex := pos - len(commands)
	if runIndex >= 0 && runIndex < len(runs) {
		return runs[runIndex].Command
	}
	return ""
}

func (m Model) visibleHistoryCommands() []string {
	commands := make([]string, 0, len(m.History))
	for _, command := range m.History {
		if m.HistoryFilter == "" || filterMatches(command, m.HistoryFilter) {
			commands = append(commands, command)
		}
	}
	if len(commands) > 6 {
		return commands[:6]
	}
	return commands
}

func (m Model) filteredHistoryCommandCount() int {
	count := 0
	for _, command := range m.History {
		if m.HistoryFilter == "" || filterMatches(command, m.HistoryFilter) {
			count++
		}
	}
	return count
}

func (m Model) visibleRunHistory() []history.RunEntry {
	runs := make([]history.RunEntry, 0, len(m.RunHistory))
	for _, run := range m.RunHistory {
		if m.HistoryFilter == "" || filterMatches(run.Command, m.HistoryFilter) {
			runs = append(runs, run)
		}
	}
	if len(runs) > 5 {
		return runs[:5]
	}
	return runs
}

func (m Model) filteredRunHistoryCount() int {
	count := 0
	for _, run := range m.RunHistory {
		if m.HistoryFilter == "" || filterMatches(run.Command, m.HistoryFilter) {
			count++
		}
	}
	return count
}

func (m *Model) previousCommandHistory() {
	if len(m.History) == 0 {
		return
	}
	if m.CommandHistoryPos < 0 {
		m.CommandDraft = m.Command
		m.CommandHistoryPos = 0
	} else if m.CommandHistoryPos < len(m.History)-1 {
		m.CommandHistoryPos++
	}
	m.Command = m.History[m.CommandHistoryPos]
}

func (m *Model) nextCommandHistory() {
	if m.CommandHistoryPos < 0 {
		return
	}
	if m.CommandHistoryPos == 0 {
		m.CommandHistoryPos = -1
		m.Command = m.CommandDraft
		m.CommandDraft = ""
		return
	}
	m.CommandHistoryPos--
	m.Command = m.History[m.CommandHistoryPos]
}

func (m *Model) resetCommandHistoryNavigation() {
	m.CommandHistoryPos = -1
	m.CommandDraft = ""
}

func (m *Model) toggleFocused() {
	if len(m.Targets) == 0 {
		return
	}
	m.ensureCursorVisible()
	indexes := m.targetSubtreeIndexes(m.Cursor)
	selected := !m.Targets[m.Cursor].Selected
	for _, index := range indexes {
		m.Targets[index].Selected = selected
	}
	state := "deselected"
	if selected {
		state = "selected"
	}
	m.Notice = state + " " + m.Targets[m.Cursor].RelPath
	m.RunError = ""
}

func (m *Model) cycleFocus(_ int) {
	switch m.Focus {
	case FocusTargets:
		m.Focus = FocusLogs
	case FocusLogs:
		m.Focus = FocusTargets
	default:
		m.Focus = FocusTargets
	}
}

func (m *Model) toggleVisibleSelected() {
	indexes := m.visibleTargetIndexes()
	scope := "visible"
	if m.Filter != "" {
		indexes = m.matchingTargetIndexes()
		scope = "matching"
	}
	selected := false
	for _, i := range indexes {
		if !m.Targets[i].Selected {
			selected = true
			break
		}
	}
	m.setTargetIndexesSelected(indexes, scope, selected)
}

func (m *Model) setTargetIndexesSelected(indexes []int, scope string, selected bool) {
	count := 0
	for _, i := range indexes {
		m.Targets[i].Selected = selected
		count++
	}
	action := "selected"
	if !selected {
		action = "deselected"
	}
	m.Notice = fmt.Sprintf("%s %d %s target(s)", action, count, scope)
	m.RunError = ""
}

func (m Model) bulkSelectionLabel() string {
	if m.Filter != "" {
		return "toggle matches"
	}
	return "toggle visible"
}

func (m *Model) selectFailedTargets() {
	count := 0
	for i := range m.Targets {
		m.Targets[i].Selected = m.Status[m.Targets[i].ID] == core.StatusFailed
		if m.Targets[i].Selected {
			count++
		}
	}
	m.ensureCursorVisible()
	if count == 0 {
		m.Notice = "no failed targets to select"
	} else {
		m.Notice = fmt.Sprintf("selected %d failed target(s)", count)
	}
	m.RunError = ""
}

func (m *Model) setFolded(folded bool) {
	if len(m.Targets) == 0 {
		return
	}
	m.ensureCursorVisible()
	if len(m.Targets[m.Cursor].Children) == 0 {
		m.RunError = ""
		return
	}
	m.Targets[m.Cursor].Folded = folded
	action := "unfolded"
	if folded {
		action = "folded"
	}
	m.Notice = action + " " + m.Targets[m.Cursor].RelPath
	m.RunError = ""
}

func (m *Model) cancelSelectedOrFocused() {
	cancelled := 0
	for _, target := range m.Targets {
		if target.Selected {
			if m.cancelTarget(target) {
				cancelled++
			}
		}
	}
	if cancelled == 0 && len(m.Targets) > 0 {
		m.ensureCursorVisible()
		if m.cancelTarget(m.Targets[m.Cursor]) {
			cancelled++
		}
	}
	if cancelled > 0 {
		m.Notice = fmt.Sprintf("cancelled %d target(s)", cancelled)
		m.RunError = ""
	} else {
		m.Notice = "nothing running or queued to cancel"
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
		m.recordCompletedResult(core.RunResult{Target: target, Status: core.StatusCancelled})
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
	for _, cancel := range m.targetCancels {
		if cancel != nil {
			cancel()
		}
	}
	cancelled := 0
	for _, target := range m.Targets {
		switch m.Status[target.ID] {
		case core.StatusRunning:
			m.Status[target.ID] = core.StatusCancelled
			cancelled++
		case core.StatusQueued:
			m.Status[target.ID] = core.StatusCancelled
			if m.PendingRuns > 0 {
				m.PendingRuns--
			}
			m.recordCompletedResult(core.RunResult{Target: target, Status: core.StatusCancelled})
			cancelled++
		}
	}
	m.runQueue = nil
	if m.PendingRuns == 0 {
		m.appendRunHistory()
		m.Running = false
		m.runCtx = nil
		m.cancelRun = nil
		m.targetCancels = map[string]context.CancelFunc{}
	}
	if cancelled > 0 {
		m.Notice = fmt.Sprintf("cancelled %d active target(s)", cancelled)
	}
}

func (m Model) completionNotice() string {
	stats := m.statusCounts()
	notice := fmt.Sprintf("run complete: %d ok, %d failed, %d cancelled", stats[core.StatusSucceeded], stats[core.StatusFailed], stats[core.StatusCancelled])
	if stats[core.StatusFailed] > 0 {
		notice += " · press R to rerun failed"
	}
	return notice
}

func (m Model) hasActiveRuns() bool {
	if m.Running {
		return true
	}
	for _, status := range m.Status {
		if status == core.StatusRunning || status == core.StatusQueued {
			return true
		}
	}
	return false
}

func (m Model) activeCount() int {
	count := 0
	for _, status := range m.Status {
		if status == core.StatusRunning || status == core.StatusQueued {
			count++
		}
	}
	return count
}

func (m Model) statusCount(want core.Status) int {
	count := 0
	for _, status := range m.Status {
		if status == want {
			count++
		}
	}
	return count
}

func (m Model) activeTargetSummary(width int) string {
	return m.statusTargetSummary(core.StatusRunning, width, core.StatusQueued)
}

func (m Model) statusTargetSummary(status core.Status, width int, extra ...core.Status) string {
	names := make([]string, 0, len(m.Targets))
	statuses := append([]core.Status{status}, extra...)
	for _, target := range m.Targets {
		current := m.Status[target.ID]
		for _, candidate := range statuses {
			if current == candidate {
				names = append(names, target.RelPath)
				break
			}
		}
	}
	if len(names) == 0 {
		return "none"
	}
	summary := strings.Join(names, ", ")
	if len(names) > 3 {
		summary = strings.Join(names[:3], ", ") + fmt.Sprintf(", +%d more", len(names)-3)
	}
	return truncateVisible(summary, width)
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

func (m Model) statusCounts() map[core.Status]int {
	counts := map[core.Status]int{
		core.StatusIdle:      0,
		core.StatusQueued:    0,
		core.StatusRunning:   0,
		core.StatusSucceeded: 0,
		core.StatusFailed:    0,
		core.StatusCancelled: 0,
		core.StatusSkipped:   0,
	}
	for _, target := range m.Targets {
		status := m.Status[target.ID]
		if status == "" {
			status = core.StatusIdle
		}
		counts[status]++
	}
	return counts
}

func (m Model) visible(target core.Target) bool {
	if m.Filter == "" || filterMatches(target.RelPath, m.Filter) {
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

func (m *Model) moveFilterMatch(delta int) {
	indexes := m.matchingTargetIndexes()
	if len(indexes) == 0 {
		return
	}
	position := 0
	for i, targetIndex := range indexes {
		if targetIndex == m.Cursor {
			position = i
			break
		}
	}
	position += delta
	for position < 0 {
		position += len(indexes)
	}
	position %= len(indexes)
	m.Cursor = indexes[position]
	m.ensureDirectoryOffset()
}

func (m *Model) moveCursorToEdge(last bool) {
	indexes := m.visibleTargetIndexes()
	if len(indexes) == 0 {
		return
	}
	if last {
		m.Cursor = indexes[len(indexes)-1]
	} else {
		m.Cursor = indexes[0]
	}
	m.ensureDirectoryOffset()
}

func (m *Model) scrollPreview(delta int) {
	m.PreviewOffset += delta
	if m.PreviewOffset < 0 {
		m.PreviewOffset = 0
	}
	m.LogFollow = false
}

func (m *Model) ensureCursorVisible() {
	if len(m.Targets) == 0 {
		m.Cursor = 0
		return
	}
	if m.Filter != "" && (m.Cursor < 0 || m.Cursor >= len(m.Targets) || !filterMatches(m.Targets[m.Cursor].RelPath, m.Filter) || m.hiddenByFold(m.Targets[m.Cursor])) {
		if indexes := m.matchingTargetIndexes(); len(indexes) > 0 {
			m.Cursor = indexes[0]
			m.ensureDirectoryOffset()
			return
		}
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
	panelHeight := max(10, height-9)
	return max(1, panelHeight-5)
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

func (m Model) matchingTargetIndexes() []int {
	if m.Filter == "" {
		return nil
	}
	indexes := make([]int, 0, len(m.Targets))
	for i, target := range m.Targets {
		if filterMatches(target.RelPath, m.Filter) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (m Model) filterMatchLabel() string {
	if m.Filter == "" {
		return "match -"
	}
	indexes := m.matchingTargetIndexes()
	if len(indexes) == 0 {
		return "match 0/0"
	}
	for i, targetIndex := range indexes {
		if targetIndex == m.Cursor {
			return fmt.Sprintf("match %d/%d", i+1, len(indexes))
		}
	}
	return fmt.Sprintf("match -/%d", len(indexes))
}

func (m Model) isVisibleTarget(target core.Target) bool {
	if m.Filter != "" {
		return m.visible(target)
	}
	return m.visible(target) && !m.hiddenByFold(target)
}
