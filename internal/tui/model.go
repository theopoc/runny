package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/saewyn/runny/internal/core"
)

type Focus int

const (
	FocusTargets Focus = iota
	FocusFilter
	FocusLogs
)

type Options struct {
	Command string
	Targets []core.Target
}

type Model struct {
	Command     string
	Targets     []core.Target
	Status      map[string]core.Status
	Focus       Focus
	Cursor      int
	Filter      string
	ShowHelp    bool
	ShowHistory bool
	ConfirmRun  bool
	Width       int
	Height      int
}

func NewModel(opts Options) Model {
	status := map[string]core.Status{}
	for _, target := range opts.Targets {
		status[target.ID] = core.StatusQueued
	}
	return Model{Command: opts.Command, Targets: opts.Targets, Status: status}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.Width = size.Width
		m.Height = size.Height
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
	switch keyName {
	case "ctrl+c":
		m.cancelAll()
		return m, tea.Quit
	case "esc":
		if m.Focus == FocusFilter {
			m.Focus = FocusTargets
			return m, nil
		}
	case "q":
		if !m.hasActiveRuns() {
			return m, tea.Quit
		}
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "tab":
		m.Focus = (m.Focus + 1) % 3
	case "/":
		m.Focus = FocusFilter
	case "backspace":
		if m.Focus == FocusFilter && len(m.Filter) > 0 {
			m.Filter = m.Filter[:len(m.Filter)-1]
			m.ensureCursorVisible()
		}
	case "?":
		m.ShowHelp = !m.ShowHelp
	case "H":
		m.ShowHistory = !m.ShowHistory
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
		m.ConfirmRun = true
	default:
		if m.Focus == FocusFilter && key.Key().Text != "" {
			m.Filter += key.Key().Text
			m.ensureCursorVisible()
		}
	}
	return m, nil
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
	if width < 72 {
		width = 72
	}
	height := m.Height
	if height < 18 {
		height = 18
	}
	mainHeight := max(8, height-7)
	leftWidth := width * 62 / 100
	if leftWidth < 42 {
		leftWidth = 42
	}
	rightWidth := width - leftWidth - 3
	if rightWidth < 24 {
		rightWidth = 24
		leftWidth = width - rightWidth - 3
	}

	b.WriteString(renderHeader(width, m.Command, m.Filter, m.Focus))
	b.WriteByte('\n')
	left := m.renderDirectoryPanel(leftWidth, mainHeight)
	right := m.renderLogPanel(rightWidth, mainHeight)
	b.WriteString(joinPanels(left, right))
	if m.Filter != "" {
		b.WriteString("\nFilter active: ")
		b.WriteString(m.Filter)
	}
	if m.ShowHelp {
		b.WriteString("\n\n")
		b.WriteString(renderBox(width, "Shortcuts", []string{
			"space toggle   a select all   A deselect all   / filter",
			"up/down or j/k move   left fold   right/l unfold",
			"H history   del cancel selected running   R rerun failed",
			"ctrl+c cancel active runs and quit   q quit when idle",
		}))
	}
	if m.ShowHistory {
		b.WriteString("\n\n")
		b.WriteString(renderBox(width, "History", []string{"No command history loaded yet."}))
	}
	b.WriteByte('\n')
	b.WriteString(renderFooter(width))
	return b.String()
}

func renderHeader(width int, command string, filter string, focus Focus) string {
	if command == "" {
		command = "<enter command>"
	}
	filterText := filter
	if filterText == "" {
		filterText = "<none>"
	}
	focusText := "directories"
	if focus == FocusFilter {
		focusText = "filter"
	}
	line := " runny  command: " + command + "  filter: " + filterText + "  focus: " + focusText
	return padRight(truncate(line, width), width)
}

func (m Model) renderDirectoryPanel(width int, height int) []string {
	rows := []string{"sel fold directory" + strings.Repeat(" ", max(1, width-30)) + "status"}
	count := 0
	limit := max(1, height-4)
	for i, target := range m.Targets {
		if count >= limit {
			break
		}
		if !m.visible(target) || m.hiddenByFold(target) {
			continue
		}
		rows = append(rows, m.renderTargetRow(i, target, width-4))
		count++
	}
	if count == 0 {
		rows = append(rows, "  no directories")
	}
	return boxLines(width, height, "Directories", rows)
}

func (m Model) renderTargetRow(index int, target core.Target, width int) string {
	cursor := " "
	if index == m.Cursor {
		cursor = ">"
	}
	selected := " "
	if target.Selected {
		selected = "x"
	}
	fold := " "
	if target.Folded {
		fold = "+"
	} else if len(target.Children) > 0 {
		fold = "-"
	}
	name := strings.Repeat("  ", max(0, target.Depth-1)) + target.RelPath
	status := string(m.Status[target.ID])
	nameWidth := max(10, width-17)
	return cursor + " [" + selected + "] " + fold + " " + padRight(truncate(name, nameWidth), nameWidth) + " " + padRight(status, 10)
}

func (m Model) renderLogPanel(width int, height int) []string {
	lines := []string{"focused target"}
	if len(m.Targets) > 0 && m.Cursor >= 0 && m.Cursor < len(m.Targets) {
		target := m.Targets[m.Cursor]
		lines = append(lines, target.RelPath+"  "+string(m.Status[target.ID]))
	} else {
		lines = append(lines, "none")
	}
	lines = append(lines, "", "Logs", "No output yet.")
	return boxLines(width, height, "Logs", lines)
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
	return strings.Join(boxLines(width, len(rows)+2, title, rows), "\n")
}

func boxLines(width int, height int, title string, rows []string) []string {
	width = max(width, len(title)+6)
	height = max(height, 3)
	lines := make([]string, 0, height)
	titleText := " " + title + " "
	topFill := max(0, width-len(titleText)-2)
	lines = append(lines, "+"+titleText+strings.Repeat("-", topFill)+"+")
	contentWidth := width - 4
	for i := 0; i < height-2; i++ {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		lines = append(lines, "| "+padRight(truncate(row, contentWidth), contentWidth)+" |")
	}
	lines = append(lines, "+"+strings.Repeat("-", width-2)+"+")
	return lines
}

func renderFooter(width int) string {
	return padRight(truncate(" Shortcuts  space toggle  / filter  enter run  ? help  H history  ctrl+c cancel+quit", width), width)
}

func truncate(value string, width int) string {
	if len(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:max(0, width)]
	}
	return value[:width-1] + "~"
}

func padRight(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
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
		if m.visible(m.Targets[i]) {
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
		if target.Selected && m.Status[target.ID] == core.StatusRunning {
			m.Status[target.ID] = core.StatusCancelled
			cancelled = true
		}
	}
	if !cancelled && len(m.Targets) > 0 && m.Status[m.Targets[m.Cursor].ID] == core.StatusRunning {
		m.Status[m.Targets[m.Cursor].ID] = core.StatusCancelled
	}
}

func (m *Model) cancelAll() {
	for id, status := range m.Status {
		if status == core.StatusRunning || status == core.StatusQueued {
			m.Status[id] = core.StatusCancelled
		}
	}
}

func (m Model) hasActiveRuns() bool {
	for _, status := range m.Status {
		if status == core.StatusRunning {
			return true
		}
	}
	return false
}

func (m Model) visible(target core.Target) bool {
	return m.Filter == "" || strings.Contains(target.RelPath, m.Filter)
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
		if m.visible(m.Targets[next]) {
			m.Cursor = next
			return
		}
	}
}

func (m *Model) ensureCursorVisible() {
	if len(m.Targets) == 0 {
		m.Cursor = 0
		return
	}
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) || !m.visible(m.Targets[m.Cursor]) {
		for i, target := range m.Targets {
			if m.visible(target) {
				m.Cursor = i
				return
			}
		}
		m.Cursor = 0
	}
}
