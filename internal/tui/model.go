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
	var b strings.Builder
	b.WriteString("runny  ")
	b.WriteString(m.Command)
	b.WriteByte('\n')
	if m.Filter != "" {
		b.WriteString("filter: ")
		b.WriteString(m.Filter)
		b.WriteByte('\n')
	}
	b.WriteString("sel fold directory status\n")
	for i, target := range m.Targets {
		if !m.visible(target) {
			continue
		}
		cursor := " "
		if i == m.Cursor {
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
		b.WriteString(cursor + " [" + selected + "] " + fold + " ")
		b.WriteString(strings.Repeat("  ", max(0, target.Depth-1)))
		b.WriteString(target.RelPath)
		b.WriteString(" ")
		b.WriteString(string(m.Status[target.ID]))
		b.WriteByte('\n')
	}
	if m.ShowHelp {
		b.WriteString("\n?: help  space: toggle  a/A: all/none  /: filter  H: history  del: cancel  R: rerun failed  ctrl+c: cancel+quit\n")
	}
	if m.ShowHistory {
		b.WriteString("\nhistory\n")
	}
	return tea.NewView(b.String())
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
