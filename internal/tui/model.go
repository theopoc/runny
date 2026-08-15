package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	runFunc            func(context.Context, core.RunRequest) ([]core.RunResult, error)
	runTracker         *runTracker
	lifecycleCtx       context.Context
	programOptions     []tea.ProgramOption
}

type Model struct {
	Command               string
	commandCursor         int
	commandCursorValid    bool
	commandSelection      int
	commandSelecting      bool
	Targets               []core.Target
	Status                map[string]core.Status
	Logs                  map[string]string
	liveLogTruncated      map[string]bool
	TargetStarted         map[string]time.Time
	History               []string
	RunHistory            []history.RunEntry
	Focus                 Focus
	Cursor                int
	DirectoryOffset       int
	HistoryPos            int
	HistoryTab            historyTab
	HistoryDepth          historyDepth
	HistoryCommandPos     int
	HistoryTargetPos      int
	HistoryShowAll        bool
	HistoryLog            string
	HistoryLogError       string
	HistoryLogLoading     bool
	HistoryLogOffset      int
	HistoryDetailOffset   int
	historyLogRunID       string
	historyLogTargetID    string
	CommandHistoryPos     int
	CommandDraft          string
	Filter                string
	ShowHelp              bool
	ShowHistory           bool
	ShowPalette           bool
	ConfirmRun            bool
	confirmRunCommand     string
	confirmRunTargets     []core.Target
	ConfirmCancelSelected bool
	ConfirmCancelAll      bool
	ConfirmQuit           bool
	ConfirmQuitYes        bool
	Zoom                  bool
	Palette               string
	PalettePos            int
	HistoryFilter         string
	HistorySearching      bool
	PreviewOffset         int
	LogFollow             bool
	Running               bool
	PendingRuns           int
	spinnerFrame          int
	runCtx                context.Context
	runQueue              []core.Target
	runLogRoot            string
	completedResults      []core.RunResult
	RunError              string
	Notice                string
	Width                 int
	Height                int
	Mode                  core.ExecutionMode
	Workers               int
	FailFast              bool
	SaveLogs              bool
	DisableLogging        bool
	LogRoot               string
	CommandHistoryPath    string
	RunHistoryPath        string
	cancelRun             context.CancelFunc
	targetCancels         map[string]context.CancelFunc
	runFunc               func(context.Context, core.RunRequest) ([]core.RunResult, error)
	runTracker            *runTracker
	lifecycleCtx          context.Context
}

type runTracker struct {
	mu     sync.Mutex
	cond   *sync.Cond
	closed bool
	active int
}

func newRunTracker() *runTracker {
	tracker := &runTracker{}
	tracker.cond = sync.NewCond(&tracker.mu)
	return tracker
}

func (t *runTracker) TryStart() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	t.active++
	return true
}

func (t *runTracker) Done() {
	t.mu.Lock()
	t.active--
	if t.active == 0 {
		t.cond.Broadcast()
	}
	t.mu.Unlock()
}

func (t *runTracker) CloseAndWait() {
	t.mu.Lock()
	t.closed = true
	for t.active > 0 {
		t.cond.Wait()
	}
	t.mu.Unlock()
}

func NewModel(opts Options) Model {
	status := map[string]core.Status{}
	logs := map[string]string{}
	started := map[string]time.Time{}
	for _, target := range opts.Targets {
		status[target.ID] = core.StatusIdle
		logs[target.ID] = ""
	}
	runFunc := opts.runFunc
	if runFunc == nil {
		runFunc = runner.Run
	}
	lifecycleCtx := opts.lifecycleCtx
	if lifecycleCtx == nil {
		lifecycleCtx = context.Background()
	}
	model := Model{
		Command:            opts.Command,
		Targets:            opts.Targets,
		Status:             status,
		Logs:               logs,
		liveLogTruncated:   map[string]bool{},
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
		runFunc:            runFunc,
		runTracker:         opts.runTracker,
		lifecycleCtx:       lifecycleCtx,
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

type runOutputMsg struct {
	targetID string
	chunk    string
	stream   <-chan tea.Msg
}

type shutdownMsg struct{}

type historyLogLoadedMsg struct {
	runID    string
	targetID string
	content  string
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
	if paste, ok := msg.(tea.PasteMsg); ok {
		if m.Focus == FocusCommand {
			m.insertCommandText(paste.Content)
			return m, nil
		}
		return m, nil
	}
	if clipboard, ok := msg.(tea.ClipboardMsg); ok {
		if m.Focus == FocusCommand {
			m.insertCommandText(clipboard.Content)
			return m, nil
		}
		return m, nil
	}
	if _, ok := msg.(spinnerTickMsg); ok {
		if !m.Running {
			return m, nil
		}
		m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
		return m, spinnerTick()
	}
	if _, ok := msg.(shutdownMsg); ok {
		m.cancelAll()
		return m, tea.Quit
	}
	if loaded, ok := msg.(historyLogLoadedMsg); ok {
		m.applyHistoryLogLoaded(loaded)
		return m, nil
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width = size.Width
		m.Height = size.Height
		if m.ShowHistory && m.HistoryDepth == historyDepthLogs {
			m.HistoryLogOffset = min(m.HistoryLogOffset, m.maxHistoryLogOffset())
		}
		if m.ShowHistory && m.HistoryDepth == historyDepthTargets {
			m.HistoryDetailOffset = min(m.HistoryDetailOffset, m.maxHistoryDetailOffset())
		}
		return m, nil
	}
	if done, ok := msg.(runDoneMsg); ok {
		cmd := m.applyRunDone(done)
		return m, cmd
	}
	if output, ok := msg.(runOutputMsg); ok {
		m.Logs[output.targetID], m.liveLogTruncated[output.targetID] = runner.AppendOutputTail(
			m.Logs[output.targetID],
			output.chunk,
			m.liveLogTruncated[output.targetID],
		)
		return m, waitForRunStream(output.stream)
	}
	if click, ok := msg.(tea.MouseClickMsg); ok {
		if click.Button == tea.MouseLeft {
			if focus, hit := m.paneFocusAt(click.X, click.Y); hit {
				m.Focus = focus
			}
		}
		return m, nil
	}
	if wheel, ok := msg.(tea.MouseWheelMsg); ok {
		m.handleMouseWheel(wheel)
		return m, nil
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	keyName := key.String()
	if key.Key().Text != "" {
		keyName = key.Key().Text
	}
	if keyName == "ctrl+c" && !(m.Focus == FocusCommand && m.hasCommandSelection()) {
		m.ShowHelp = false
		m.ShowHistory = false
		m.ShowPalette = false
		m.ConfirmRun = false
		m.clearConfirmedRun()
		m.ConfirmCancelSelected = false
		m.ConfirmCancelAll = false
		m.ConfirmQuit = true
		m.ConfirmQuitYes = true
		return m, nil
	}
	if keyName == "?" {
		m.ShowHelp = !m.ShowHelp
		return m, nil
	}
	if m.ShowHelp || m.ShowHistory || m.ConfirmRun || m.ConfirmCancelSelected || m.ConfirmCancelAll || m.ConfirmQuit {
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
		if m.hasActiveRuns() {
			m.ConfirmQuit = true
			m.ConfirmQuitYes = true
			return m, nil
		}
		return m, tea.Quit
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
		if m.ShowHistory {
			m.ShowHistory = false
		} else {
			m.openHistory()
		}
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

func (m *Model) handleMouseWheel(wheel tea.MouseWheelMsg) {
	if m.ShowHelp || m.ShowHistory || m.ShowPalette || m.ConfirmRun || m.ConfirmCancelSelected || m.ConfirmCancelAll || m.ConfirmQuit {
		return
	}

	direction := 0
	switch wheel.Button {
	case tea.MouseWheelUp:
		direction = -1
	case tea.MouseWheelDown:
		direction = 1
	default:
		return
	}

	switch m.Focus {
	case FocusTargets:
		m.moveCursor(direction)
	case FocusLogs:
		maxOffset := m.outputMaxOffset()
		if m.LogFollow {
			m.PreviewOffset = maxOffset
		}
		m.scrollPreview(direction * 3)
		m.PreviewOffset = min(m.PreviewOffset, maxOffset)
	}
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
	case "left":
		m.moveCommandCursor(-1, false)
	case "right":
		m.moveCommandCursor(1, false)
	case "shift+left":
		m.moveCommandCursor(-1, true)
	case "shift+right":
		m.moveCommandCursor(1, true)
	case "alt+left", "alt+b":
		m.moveCommandCursorByWord(-1, false)
	case "alt+right", "alt+f":
		m.moveCommandCursorByWord(1, false)
	case "alt+shift+left", "alt+shift+b":
		m.moveCommandCursorByWord(-1, true)
	case "alt+shift+right", "alt+shift+f":
		m.moveCommandCursorByWord(1, true)
	case "home", "ctrl+a":
		m.setCommandCursor(0, false)
	case "end", "ctrl+e":
		m.setCommandCursor(len([]rune(m.Command)), false)
	case "backspace":
		m.deleteCommandBackward()
	case "delete":
		m.deleteCommandForward()
	case "ctrl+u":
		m.Command = ""
		m.setCommandCursor(0, false)
		m.Notice = "command cleared"
		m.RunError = ""
		m.resetCommandHistoryNavigation()
	case "ctrl+w":
		m.deleteCommandWordBackward()
	case "ctrl+c", "super+c":
		return m, tea.SetClipboard(m.selectedCommandText())
	case "ctrl+x", "super+x":
		selected := m.selectedCommandText()
		if selected != "" {
			m.deleteCommandSelection()
			m.resetCommandHistoryNavigation()
			return m, tea.SetClipboard(selected)
		}
	case "ctrl+v", "super+v":
		return m, func() tea.Msg { return tea.ReadClipboard() }
	case " ", "space":
		m.insertCommandText(" ")
	default:
		if key.Key().Text != "" {
			m.insertCommandText(key.Key().Text)
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
		if m.Filter != "" {
			m.Filter = deleteLastRune(m.Filter)
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
		if m.Palette != "" {
			m.Palette = deleteLastRune(m.Palette)
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
		m.openHistory()
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
			m.openHistory()
		}
		return m, nil
	}
	if m.ShowHistory {
		return m.handleHistoryKey(keyName, key)
	}
	switch keyName {
	case "esc":
		if m.ConfirmRun {
			m.clearConfirmedRun()
		}
		m.ConfirmRun = false
		m.ConfirmCancelSelected = false
		m.ConfirmCancelAll = false
		m.ConfirmQuit = false
		m.ConfirmQuitYes = false
	case "n":
		if m.ConfirmRun {
			m.clearConfirmedRun()
		}
		m.ConfirmRun = false
		m.ConfirmCancelSelected = false
		m.ConfirmCancelAll = false
		m.ConfirmQuit = false
		m.ConfirmQuitYes = false
		m.Notice = "confirmation cancelled"
	case "tab":
		if m.ConfirmQuit {
			m.ConfirmQuitYes = !m.ConfirmQuitYes
		}
	case "enter", "y":
		if m.ConfirmRun {
			m.ConfirmRun = false
			if len(m.confirmRunTargets) > 0 {
				command := m.confirmRunCommand
				targets := append([]core.Target(nil), m.confirmRunTargets...)
				m.clearConfirmedRun()
				return m.beginRun(command, targets)
			}
			return m.startRun(true)
		}
		if m.ConfirmCancelSelected {
			m.ConfirmCancelSelected = false
			m.cancelSelectedImmediate()
			return m, nil
		}
		if m.ConfirmCancelAll {
			m.ConfirmCancelAll = false
			m.cancelAll()
			return m, nil
		}
		if m.ConfirmQuit {
			if keyName == "enter" && !m.ConfirmQuitYes {
				m.ConfirmQuit = false
				m.Notice = "confirmation cancelled"
				return m, nil
			}
			m.ConfirmQuit = false
			m.cancelAll()
			return m, tea.Quit
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
			if !m.hasHistorySelection() {
				if m.HistoryTab == historyTabCommands {
					m.RunError = "no history command matches"
				} else {
					m.RunError = "no project run matches"
				}
				return m, nil
			}
			m.HistorySearching = false
			return m.activateHistorySelection()
		case "ctrl+u":
			m.HistoryFilter = ""
			m.resetHistoryPositions()
			m.RunError = ""
		case "backspace":
			if m.HistoryFilter != "" {
				m.HistoryFilter = deleteLastRune(m.HistoryFilter)
				m.resetHistoryPositions()
				m.RunError = ""
			}
		case "up", "k":
			m.moveHistorySelection(-1)
		case "down", "j":
			m.moveHistorySelection(1)
		default:
			if key.Key().Text != "" {
				m.HistoryFilter += key.Key().Text
				m.resetHistoryPositions()
				m.RunError = ""
			}
		}
		return m, nil
	}
	switch keyName {
	case "esc":
		if m.HistoryDepth == historyDepthLogs {
			m.HistoryDepth = historyDepthTargets
			m.HistoryDetailOffset = 0
		} else if m.HistoryDepth == historyDepthTargets {
			m.HistoryDepth = historyDepthRuns
			m.HistoryDetailOffset = 0
		} else {
			m.ShowHistory = false
		}
	case "/":
		m.HistorySearching = true
	case "ctrl+u":
		m.HistoryFilter = ""
		m.resetHistoryPositions()
		m.RunError = ""
	case "[":
		m.HistoryTab = historyTabRuns
		m.HistoryDepth = historyDepthRuns
		m.HistoryDetailOffset = 0
	case "]":
		m.HistoryTab = historyTabCommands
		m.HistoryDepth = historyDepthRuns
		m.HistoryDetailOffset = 0
	case "up", "k":
		m.moveHistorySelection(-1)
	case "down", "j":
		m.moveHistorySelection(1)
	case "pageup", "pgup":
		if m.HistoryTab == historyTabRuns && m.HistoryDepth == historyDepthTargets {
			m.HistoryDetailOffset = min(m.maxHistoryDetailOffset(), m.HistoryDetailOffset+max(1, m.Height/2))
		}
	case "pagedown", "pgdown":
		if m.HistoryTab == historyTabRuns && m.HistoryDepth == historyDepthTargets {
			m.HistoryDetailOffset = max(0, m.HistoryDetailOffset-max(1, m.Height/2))
		}
	case "a":
		if m.HistoryTab == historyTabRuns && m.HistoryDepth == historyDepthTargets {
			m.HistoryShowAll = !m.HistoryShowAll
			m.HistoryTargetPos = 0
			m.HistoryDetailOffset = 0
		}
	case "enter":
		return m.activateHistorySelection()
	case "r":
		if m.HistoryTab == historyTabRuns {
			return m.reuseSelectedHistoryRun()
		}
	case "R":
		if m.HistoryTab == historyTabRuns {
			m.prepareHistoricalRerun()
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
		m.Focus = FocusTargets
		m.Notice = ""
		return m, nil
	}
	return m.beginRun(strings.TrimSpace(m.Command), targets)
}

func (m Model) beginRun(command string, targets []core.Target) (tea.Model, tea.Cmd) {
	if m.Running {
		m.Notice = "run already in progress"
		return m, nil
	}
	command = strings.TrimSpace(command)
	if command == "" {
		m.Focus = FocusCommand
		m.RunError = "command is empty; press c to edit"
		return m, nil
	}
	if len(targets) == 0 {
		m.RunError = "no selected targets"
		m.Focus = FocusTargets
		return m, nil
	}
	m.Command = command
	m.Focus = FocusTargets
	reqTargets := append([]core.Target(nil), targets...)
	ctx, cancel := context.WithCancel(m.lifecycleCtx)
	if m.targetCancels == nil {
		m.targetCancels = map[string]context.CancelFunc{}
	}
	m.cancelRun = cancel
	m.runCtx = ctx
	m.targetCancels = map[string]context.CancelFunc{}
	m.Running = true
	m.PendingRuns = len(reqTargets)
	m.runQueue = append([]core.Target(nil), reqTargets...)
	m.runLogRoot = ""
	if m.SaveLogs && !m.DisableLogging {
		m.runLogRoot = filepath.Join(m.LogRoot, time.Now().UTC().Format("20060102T150405.000000000Z"))
	}
	m.completedResults = nil
	m.RunError = ""
	m.Notice = fmt.Sprintf("started %d target(s)", len(reqTargets))
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
		m.liveLogTruncated[target.ID] = false
		delete(m.TargetStarted, target.ID)
	}
	next, cmd := m.startQueuedRuns()
	return next, tea.Batch(spinnerTick(), cmd)
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
		if len(done.results) == 0 {
			target, _ := m.targetByID(done.targetID)
			done.results = []core.RunResult{{
				Target:  target,
				Status:  core.StatusFailed,
				Error:   done.err.Error(),
				Started: m.TargetStarted[done.targetID],
				Ended:   time.Now(),
			}}
		}
	}
	failedFast := false
	for _, result := range done.results {
		if m.Status[result.Target.ID] == core.StatusCancelled && result.Status != core.StatusCancelled {
			result.Status = core.StatusCancelled
		}
		if result.Started.IsZero() {
			result.Started = m.TargetStarted[result.Target.ID]
		}
		if result.Ended.IsZero() {
			result.Ended = time.Now()
		}
		m.Status[result.Target.ID] = result.Status
		m.recordCompletedResult(result)
		log := m.Logs[result.Target.ID]
		if log == "" {
			log = result.Output
		}
		if result.Error != "" && !strings.Contains(log, result.Error) {
			if log != "" && !strings.HasSuffix(log, "\n") {
				log += "\n"
			}
			log += result.Error
		}
		m.Logs[result.Target.ID] = log
		if m.FailFast && result.Status == core.StatusFailed {
			failedFast = true
		}
	}
	if failedFast {
		m.cancelAll()
	}
	if m.PendingRuns == 0 {
		if m.Running {
			m.appendRunHistory()
		}
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
		LogRoot:        m.runLogRoot,
	}
	stream := make(chan tea.Msg)
	req.OnEvent = func(event core.Event) {
		if event.Type != core.EventOutput || event.TargetID != target.ID || event.Output == "" {
			return
		}
		select {
		case stream <- runOutputMsg{targetID: target.ID, chunk: event.Output, stream: stream}:
		case <-targetCtx.Done():
		}
	}
	runTracker := m.runTracker
	lifecycleCtx := m.lifecycleCtx
	if lifecycleCtx == nil {
		lifecycleCtx = context.Background()
	}
	return func() tea.Msg {
		if runTracker != nil {
			if !runTracker.TryStart() {
				targetCancel()
				now := time.Now()
				return runDoneMsg{targetID: target.ID, results: []core.RunResult{{
					Target:  target,
					Status:  core.StatusCancelled,
					Error:   context.Canceled.Error(),
					Started: now,
					Ended:   now,
				}}}
			}
		}
		go func() {
			defer close(stream)
			if runTracker != nil {
				defer runTracker.Done()
			}
			results, err := runFunc(targetCtx, req)
			done := runDoneMsg{targetID: target.ID, results: results, err: err}
			select {
			case stream <- done:
			case <-lifecycleCtx.Done():
			}
		}()
		select {
		case msg := <-stream:
			return msg
		case <-lifecycleCtx.Done():
			return nil
		}
	}
}

func waitForRunStream(stream <-chan tea.Msg) tea.Cmd {
	if stream == nil {
		return nil
	}
	return func() tea.Msg {
		return <-stream
	}
}

func deleteLastRune(value string) string {
	_, size := utf8.DecodeLastRuneInString(value)
	if size == 0 {
		return value
	}
	return value[:len(value)-size]
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
	if m.runLogRoot != "" {
		summary.LogID = filepath.Base(m.runLogRoot)
	}
	results := make(map[string]core.RunResult, len(m.completedResults))
	for _, result := range m.completedResults {
		results[result.Target.ID] = result
	}
	for _, target := range m.Targets {
		result, ok := results[target.ID]
		if !ok {
			continue
		}
		summary.Total++
		summary.Targets = append(summary.Targets, history.TargetEntry{
			ID:       target.ID,
			RelPath:  target.RelPath,
			Status:   result.Status,
			ExitCode: result.ExitCode,
			Error:    result.Error,
			Started:  result.Started,
			Ended:    result.Ended,
		})
		if !result.Started.IsZero() && (summary.Started.IsZero() || result.Started.Before(summary.Started)) {
			summary.Started = result.Started
		}
		if !result.Ended.IsZero() && (summary.Ended.IsZero() || result.Ended.After(summary.Ended)) {
			summary.Ended = result.Ended
		}
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
	if !summary.Ended.IsZero() {
		summary.Time = summary.Ended
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
	view.MouseMode = tea.MouseModeCellMotion
	view.WindowTitle = "runny"
	return view
}

func (m Model) render() string {
	var b strings.Builder
	width := m.Width
	if width > 0 && width < 60 {
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
	panelHeight, leftWidth, rightWidth := m.panelDimensions(width, height)

	b.WriteString(m.renderPanelPrefix(width))
	panels := m.renderPanelArea(width, panelHeight, leftWidth, rightWidth)
	if m.ShowHistory && !m.ShowHelp {
		panels = subtleStyle.Render(ansi.Strip(panels))
	}
	if overlay := m.renderOverlay(width, panelHeight); overlay != "" {
		b.WriteString(placeOverlay(panels, overlay, width))
	} else {
		b.WriteString(panels)
	}
	b.WriteString("\n")
	b.WriteString(m.renderDashboard(width))
	b.WriteByte('\n')
	b.WriteString(m.renderFooter(width))
	rendered := b.String()
	if m.ShowHistory && !m.ShowHelp {
		return padRenderedHeight(rendered, width, height)
	}
	return rendered
}

func padRenderedHeight(rendered string, width, height int) string {
	for strings.Count(rendered, "\n")+1 < height {
		rendered += "\n" + strings.Repeat(" ", max(0, width))
	}
	return rendered
}

func panelDimensions(width int, height int) (panelHeight int, leftWidth int, rightWidth int) {
	return panelDimensionsForInput(width, height, 1)
}

func panelDimensionsForInput(width int, height int, inputRows int) (panelHeight int, leftWidth int, rightWidth int) {
	panelHeight = panelHeightForInput(height, inputRows)
	leftWidth = width * 42 / 100
	if leftWidth < 36 {
		leftWidth = 36
	}
	rightWidth = width - leftWidth - 4
	if rightWidth < 32 {
		rightWidth = 32
		leftWidth = width - rightWidth - 4
	}
	return panelHeight, leftWidth, rightWidth
}

func (m Model) panelDimensions(width int, height int) (panelHeight int, leftWidth int, rightWidth int) {
	inputRows := m.commandInputVisibleRows(width)
	panelHeight, leftWidth, rightWidth = panelDimensionsForInput(width, height, inputRows)
	if m.Focus != FocusCommand && m.Focus != FocusFilter && !m.ShowPalette {
		panelHeight = max(10, height-4)
	}
	return panelHeight, leftWidth, rightWidth
}

func (m Model) paneFocusAt(x int, y int) (Focus, bool) {
	if m.Width < 60 || m.Height < 20 || m.hasOverlay() {
		return 0, false
	}
	panelHeight, leftWidth, rightWidth := m.panelDimensions(m.Width, m.Height)
	panelTop := strings.Count(m.renderPanelPrefix(m.Width), "\n")
	if y < panelTop || y >= panelTop+panelHeight {
		return 0, false
	}
	if m.Zoom || m.compactMode(m.Width) {
		if x < 0 || x >= m.singlePanelWidth(m.Width) {
			return 0, false
		}
		if m.Focus == FocusLogs {
			return FocusLogs, true
		}
		return FocusTargets, true
	}
	if x >= 0 && x < leftWidth {
		return FocusTargets, true
	}
	rightStart := leftWidth + lipgloss.Width(panelSeparator)
	if x >= rightStart && x < rightStart+rightWidth {
		return FocusLogs, true
	}
	return 0, false
}

func (m Model) renderPanelPrefix(width int) string {
	return strings.Join([]string{
		m.renderHeader(width),
		m.renderSubHeader(width),
	}, "\n") + "\n"
}

func (m Model) hasOverlay() bool {
	return m.ShowHelp || m.ShowHistory || m.ShowPalette || m.ConfirmRun || m.ConfirmCancelSelected || m.ConfirmCancelAll || m.ConfirmQuit
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
		fmt.Sprintf("need at least 60x20, got %dx%d", width, height),
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
	if m.ShowHistory && !m.ShowHelp {
		return m.renderHistoryOverlay(width, height)
	}
	title := ""
	var rows []string
	switch {
	case m.ShowHelp:
		title = "Keymap"
		rows = m.helpRows(width)
	case m.ShowPalette:
		title = "Command palette"
		rows = m.paletteRows()
	case m.ConfirmRun:
		title = "Rerun failed"
		rows = []string{
			fmt.Sprintf("%d failed target(s) will run again.", m.confirmedRunTargetCount()),
			"command: " + truncateVisible(m.confirmedRunCommand(), 64),
			"targets: " + m.confirmedRunTargetSummary(64),
			"y/enter confirm   n/esc cancel",
		}
	case m.ConfirmCancelSelected:
		title = "Cancel selected"
		rows = []string{
			fmt.Sprintf("%d selected active target(s) will be cancelled.", m.selectedActiveCount()),
			"scope: selected running and queued targets only",
			"targets: " + m.selectedActiveTargetSummary(64),
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
	case m.ConfirmQuit:
		rows = []string{
			centerANSI(dangerTitleStyle.Render("Quit runny?"), ansi.StringWidth(m.renderQuitChoices())),
			m.renderQuitChoices(),
		}
		return renderDangerFittedFloatingBox(width, "", rows)
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

func (m Model) renderQuitChoices() string {
	yes := " Yes "
	no := " No "
	if m.ConfirmQuitYes {
		yes = dangerChoiceStyle.Render(yes)
		no = helpDescStyle.Render(no)
	} else {
		yes = helpDescStyle.Render(yes)
		no = dangerChoiceStyle.Render(no)
	}
	return "[" + yes + "]  [" + no + "]"
}

func (m Model) renderHeader(width int) string {
	stats := m.statusCounts()
	mode := string(m.mode())
	if m.mode() == core.ModeParallel {
		mode += "×" + m.workersLabel()
	}
	segments := []string{
		runnyBadgeStyle.Render("runny"),
		subtleStyle.Render(mode),
		fmt.Sprintf("%d selected", m.selectedCount()),
		metricRunningStyle.Render(fmt.Sprintf("%d run", stats[core.StatusRunning])),
		metricQueuedStyle.Render(fmt.Sprintf("%d queue", stats[core.StatusQueued])),
		metricSuccessStyle.Render(fmt.Sprintf("%d ok", stats[core.StatusSucceeded])),
		metricFailedStyle.Render(fmt.Sprintf("%d fail", stats[core.StatusFailed])),
	}
	line := " " + strings.Join(segments, subtleStyle.Render(" · "))
	return headerStyle.Render(padRightVisible(truncateVisible(line, width), width))
}

func (m Model) renderDashboard(width int) string {
	stats := m.statusCounts()
	compact := width < 100 || m.RunError != "" || m.Notice != ""
	segments := []string{}
	if compact {
		segments = []string{
			metricRunningStyle.Render(fmt.Sprintf("●%d", stats[core.StatusRunning])),
			metricQueuedStyle.Render(fmt.Sprintf("◌%d", stats[core.StatusQueued])),
			metricSuccessStyle.Render(fmt.Sprintf("✓%d", stats[core.StatusSucceeded])),
			metricFailedStyle.Render(fmt.Sprintf("✕%d", stats[core.StatusFailed])),
		}
	} else {
		segments = []string{
			metricRunningStyle.Render(fmt.Sprintf("● %d running", stats[core.StatusRunning])),
			metricQueuedStyle.Render(fmt.Sprintf("◌ %d queued", stats[core.StatusQueued])),
			metricSuccessStyle.Render(fmt.Sprintf("✓ %d ok", stats[core.StatusSucceeded])),
			metricFailedStyle.Render(fmt.Sprintf("✕ %d failed", stats[core.StatusFailed])),
		}
	}
	if m.RunError != "" {
		segments = append(segments, errorBarStyle.Render("ERROR "+m.RunError))
	} else if m.Notice != "" {
		label := "INFO"
		style := noticeBarStyle
		if strings.HasPrefix(m.Notice, "run complete:") && m.completionNeedsAttention() {
			label = "WARN"
			style = warningBarStyle
		}
		segments = append(segments, style.Render(label+" "+m.Notice))
	}
	left := " " + strings.Join(segments, subtleStyle.Render(" · "))
	right := subtleStyle.Render("follow:" + ternary(m.LogFollow, "on", "off"))
	rightWidth := lipgloss.Width(right)
	leftWidth := max(1, width-rightWidth-2)
	return padRightVisible(truncateVisible(left, leftWidth), leftWidth) + "  " + right
}

func (m Model) renderSubHeader(width int) string {
	if m.Focus != FocusCommand && m.Focus != FocusFilter && !m.ShowPalette {
		command := strings.TrimSpace(m.Command)
		if command == "" {
			command = "(not set)"
		}
		left := commandPromptStyle.Render("command ›") + " " + command
		right := subtleStyle.Render(fmt.Sprintf("scope %d targets · workers %s", m.selectedCount(), m.workersLabel()))
		rightWidth := min(lipgloss.Width(right), max(0, width/2))
		right = truncateVisible(right, rightWidth)
		leftWidth := max(1, width-rightWidth-2)
		return padRightVisible(truncateVisible(left, leftWidth), leftWidth) + "  " + right
	}
	return strings.Join(m.commandInputBoxLines(width), "\n")
}

func (m Model) commandInputBoxLines(width int) []string {
	titleText := m.commandInputTitle()
	title := " " + commandInputTitleStyle.Render(titleText) + " "
	width = max(width, lipgloss.Width(title)+2)
	contentWidth := max(0, width-4)
	topFill := max(0, width-lipgloss.Width(title)-2)
	values := []string{}
	if m.Focus == FocusCommand && !m.ShowPalette {
		var hiddenAbove, hiddenBelow bool
		values, hiddenAbove, hiddenBelow = m.renderWrappedCommandInput(contentWidth, m.commandInputMaxRows())
		indicator := ""
		switch {
		case hiddenAbove && hiddenBelow:
			indicator = " ↕"
		case hiddenAbove:
			indicator = " ↑"
		case hiddenBelow:
			indicator = " ↓"
		}
		if indicator != "" {
			title = " " + commandInputTitleStyle.Render(titleText+indicator) + " "
			topFill = max(0, width-lipgloss.Width(title)-2)
		}
	} else {
		values = []string{commandInputStyle.Render(m.commandInputValue())}
	}
	lines := []string{
		commandInputBorderStyle.Render("┌") + title + commandInputBorderStyle.Render(strings.Repeat("─", topFill)+"┐"),
	}
	for _, value := range values {
		value = padRightVisible(truncateVisible(value, contentWidth), contentWidth)
		lines = append(lines, commandInputBorderStyle.Render("│")+" "+value+" "+commandInputBorderStyle.Render("│"))
	}
	lines = append(lines, commandInputBorderStyle.Render("└"+strings.Repeat("─", max(0, width-2))+"┘"))
	return lines
}

func (m Model) commandInputMaxRows() int {
	height := m.Height
	if height <= 0 {
		height = 20
	}
	return max(1, height-19)
}

func (m Model) commandInputVisibleRows(width int) int {
	if m.Focus != FocusCommand || m.ShowPalette {
		return 1
	}
	contentWidth := max(1, width-4)
	rows, _, _ := m.renderWrappedCommandInput(contentWidth, m.commandInputMaxRows())
	return max(1, len(rows))
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
		return m.Filter + "▌"
	}
	if m.Focus == FocusCommand {
		return m.renderCommandInputValue(len([]rune(m.Command)) + 1)
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
	cursor := " "
	if active {
		cursor = "›"
	}
	selection := unselectedStyle.Render("[ ]")
	if target.Selected {
		selection = selectedStyle.Render("[x]")
	} else if partial {
		selection = sectionStyle.Render("[-]")
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
		fold = m.foldSymbol(target)
		name = m.renderTargetNamePlain(target)
		statusText = padRightVisible(m.statusLabel(status), 12)
	}
	left := cursor + " " + selection + " " + fold + " " + name
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
	title := "Output"
	if len(m.Targets) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Targets) {
		target := m.Targets[m.Cursor]
		lines = append(lines, m.renderOutputLines(target.ID, height)...)
		title = fmt.Sprintf(
			"Output — %s [%s] · follow:%s · %d lines",
			target.RelPath,
			outputStatusLabel(m.Status[target.ID]),
			ternary(m.LogFollow, "on", "off"),
			outputLineCount(m.Logs[target.ID]),
		)
	}
	return boxLines(width, height, title, lines, m.Focus == FocusLogs)
}

func outputStatusLabel(status core.Status) string {
	switch status {
	case core.StatusQueued:
		return "QUEUE"
	case core.StatusRunning:
		return "RUN"
	case core.StatusSucceeded:
		return "OK"
	case core.StatusFailed:
		return "FAIL"
	case core.StatusCancelled:
		return "CANCEL"
	case core.StatusSkipped:
		return "SKIP"
	default:
		return "IDLE"
	}
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
			return "del/x target  ctrl+c quit"
		}
		return "del/x target   ctrl+c confirm quit"
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
	output := outputLines(m.Logs[targetID])
	if len(output) == 0 {
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
	hints := []string{}
	if m.ShowHelp {
		hints = []string{"? close", "q close", "esc close", "H history"}
	} else if m.ShowHistory {
		_, hints = m.historyFooterKeys()
	} else if m.ShowPalette {
		hints = []string{"enter choose", "up/down choose", "ctrl+u clear", "esc close", "? help"}
	} else if m.ConfirmRun {
		hints = []string{"y confirm", "enter confirm", "n cancel", "esc cancel"}
	} else if m.ConfirmCancelSelected {
		hints = []string{"y confirm", "enter confirm", "n cancel", "esc cancel"}
	} else if m.ConfirmCancelAll {
		hints = []string{"y confirm", "enter confirm", "n cancel", "esc cancel"}
	} else if m.ConfirmQuit {
		hints = []string{"tab switch", "enter choose", "y yes", "n no", "esc cancel"}
	} else {
		switch m.Focus {
		case FocusCommand:
			hints = []string{"enter run", "esc tasks", "up/down history", "ctrl+u clear", "? help", "ctrl+c quit"}
		case FocusFilter:
			hints = []string{"type fuzzy", "' exact", "n/N matches", "ctrl+u clear", "enter/esc tasks", "? help"}
		case FocusLogs:
			hints = []string{"pgup/pgdn scroll", "f follow", "tab tasks", "? help", "q quit"}
		default:
			hints = []string{"space select", "/ filter", "x cancel", "tab output", "? help", "q quit"}
			if width < 100 {
				hints = []string{"space sel", "x stop", "tab pane", "? help", "q quit"}
			}
			if !m.hasActiveRuns() && m.failedCount() > 0 {
				hints = append([]string{"R failed"}, hints...)
			}
		}
	}
	return renderFooterHintGrid(hints, width, 1)
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
	if key == "no" {
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
	return width < 100
}

func (m Model) singlePanelWidth(width int) int {
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
		return spinnerFrame(m.spinnerFrame) + " running"
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
				{"?", "keymap"}, {"/", "filter"}, {":", "palette"}, {"H", "history"}, {"q", "quit"},
				{"tab", "tasks/output"}, {"z", "maximize panel / split"}, {"esc", "close mode"},
				{"ctrl+c", "confirm quit"},
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
				{"R", "rerun failed"}, {"spinner", "running"}, {"◌", "queued"}, {"✓", "ok"},
				{"✕", "failed"}, {"–", "cancelled"},
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
				{"H", "open history"}, {"[/]", "switch runs/commands"},
				{"/", "search active tab"}, {"'", "exact match"}, {"up/down", "choose"},
				{"enter", "inspect run or logs"}, {"r", "reuse run command"},
				{"R", "rerun historical failures"}, {"a", "failures/all targets"},
				{"ctrl+u", "clear search"}, {"esc", "back/close"},
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
	case m.ConfirmRun || m.ConfirmCancelSelected || m.ConfirmCancelAll || m.hasActiveRuns():
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
	rows := []string{commandPromptStyle.Render(" : " + input + " ")}
	panelHeight, _, _ := m.panelDimensions(max(60, m.Width), m.Height)
	compact := m.Height > 0 && panelHeight-2 < len(matches)+3
	if !compact {
		rows = append(rows, subtleStyle.Render(helpText), "")
	}
	if len(matches) == 0 {
		return append(rows, "  no commands; backspace or ctrl+u to edit")
	}
	for i, command := range matches {
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
	m.moveCommandCursorToEnd()
}

func (m *Model) nextCommandHistory() {
	if m.CommandHistoryPos < 0 {
		return
	}
	if m.CommandHistoryPos == 0 {
		m.CommandHistoryPos = -1
		m.Command = m.CommandDraft
		m.moveCommandCursorToEnd()
		m.CommandDraft = ""
		return
	}
	m.CommandHistoryPos--
	m.Command = m.History[m.CommandHistoryPos]
	m.moveCommandCursorToEnd()
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
	if m.selectedActiveCount() > 1 {
		m.ConfirmCancelSelected = true
		m.Notice = ""
		m.RunError = ""
		return
	}
	m.cancelSelectedOrFocusedImmediate()
}

func (m *Model) cancelSelectedOrFocusedImmediate() {
	cancelled := m.cancelSelectedActive()
	if cancelled == 0 && len(m.Targets) > 0 {
		m.ensureCursorVisible()
		if m.cancelTarget(m.Targets[m.Cursor]) {
			cancelled++
		}
	}
	m.setCancellationNotice(cancelled)
}

func (m *Model) cancelSelectedImmediate() {
	m.setCancellationNotice(m.cancelSelectedActive())
}

func (m *Model) cancelSelectedActive() int {
	cancelled := 0
	for _, target := range m.Targets {
		if target.Selected {
			if m.cancelTarget(target) {
				cancelled++
			}
		}
	}
	return cancelled
}

func (m *Model) setCancellationNotice(cancelled int) {
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

func (m Model) selectedActiveCount() int {
	count := 0
	for _, target := range m.Targets {
		status := m.Status[target.ID]
		if target.Selected && (status == core.StatusRunning || status == core.StatusQueued) {
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

func (m Model) selectedActiveTargetSummary(width int) string {
	names := make([]string, 0, len(m.Targets))
	for _, target := range m.Targets {
		status := m.Status[target.ID]
		if target.Selected && (status == core.StatusRunning || status == core.StatusQueued) {
			names = append(names, target.RelPath)
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

func (m Model) outputMaxOffset() int {
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) {
		return 0
	}
	output := outputLines(m.Logs[m.Targets[m.Cursor].ID])
	if len(output) == 0 {
		return 0
	}
	visible := max(1, panelHeightForWindow(m.Height)-2)
	return max(0, len(output)-visible)
}

func outputLines(output string) []string {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

func outputLineCount(output string) int {
	output = strings.TrimRight(output, "\n")
	if output == "" {
		return 0
	}
	return strings.Count(output, "\n") + 1
}

func panelHeightForWindow(height int) int {
	if height < 20 {
		height = 20
	}
	return max(10, height-4)
}

func panelHeightForInput(height int, inputRows int) int {
	if height < 20 {
		height = 20
	}
	return max(10, height-5-max(1, inputRows))
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
	panelHeight, _, _ := m.panelDimensions(max(60, m.Width), m.Height)
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
