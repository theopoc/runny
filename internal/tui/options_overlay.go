package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/core"
)

type sessionOption int

type sessionOptionCategory struct {
	name    string
	narrow  string
	options []sessionOption
}

const (
	optionSerial sessionOption = iota
	optionFailFast
	optionCaptureOutput
	optionSaveLogs
	optionFollowOutput
	optionMaximizePane
)

var sessionOptions = []sessionOption{
	optionSerial,
	optionFailFast,
	optionCaptureOutput,
	optionSaveLogs,
	optionFollowOutput,
	optionMaximizePane,
}

var sessionOptionCategories = []sessionOptionCategory{
	{name: "Execution", narrow: "Exec", options: []sessionOption{optionSerial, optionFailFast}},
	{name: "Logging", narrow: "Logs", options: []sessionOption{optionCaptureOutput, optionSaveLogs}},
	{name: "Display", narrow: "View", options: []sessionOption{optionFollowOutput, optionMaximizePane}},
}

func (m Model) handleOptionsKey(keyName string) (tea.Model, tea.Cmd) {
	m.normalizeOptionsSelection()
	switch keyName {
	case "esc", "q", "o":
		m.ShowOptions = false
	case "up", "k":
		m.moveOptionsSelection(-1)
	case "down", "j":
		m.moveOptionsSelection(1)
	case "left", "h":
		m.moveOptionsTab(-1)
	case "right", "l":
		m.moveOptionsTab(1)
	case "1", "2", "3":
		m.selectOptionsTab(int(keyName[0] - '1'))
	case " ", "space", "enter":
		m.toggleSessionOption(sessionOptions[m.OptionsPos])
	}
	return m, nil
}

func (m *Model) normalizeOptionsSelection() {
	m.OptionsTab = min(max(0, m.OptionsTab), len(sessionOptionCategories)-1)
	category := sessionOptionCategories[m.OptionsTab]
	for _, option := range category.options {
		if int(option) == m.OptionsPos {
			return
		}
	}
	m.OptionsPos = int(category.options[0])
}

func (m *Model) moveOptionsSelection(delta int) {
	category := sessionOptionCategories[m.OptionsTab]
	position := 0
	for index, option := range category.options {
		if int(option) == m.OptionsPos {
			position = index
			break
		}
	}
	position = min(max(0, position+delta), len(category.options)-1)
	m.OptionsPos = int(category.options[position])
}

func (m *Model) moveOptionsTab(delta int) {
	m.selectOptionsTab((m.OptionsTab + delta + len(sessionOptionCategories)) % len(sessionOptionCategories))
}

func (m *Model) selectOptionsTab(tab int) {
	if tab < 0 || tab >= len(sessionOptionCategories) {
		return
	}
	m.OptionsTab = tab
	m.OptionsPos = int(sessionOptionCategories[tab].options[0])
}

func (m *Model) toggleSessionOption(option sessionOption) {
	if option <= optionSaveLogs && m.hasActiveRuns() {
		m.Notice = "execution options locked while runs are active"
		return
	}
	switch option {
	case optionSerial:
		if m.mode() == core.ModeSerial {
			m.Mode = core.ModeParallel
		} else {
			m.Mode = core.ModeSerial
			m.Workers = 0
		}
	case optionFailFast:
		m.FailFast = !m.FailFast
	case optionCaptureOutput:
		m.DisableLogging = !m.DisableLogging
		if m.DisableLogging {
			m.SaveLogs = false
		}
	case optionSaveLogs:
		if m.DisableLogging {
			m.Notice = "save logs requires capture output"
			return
		}
		m.SaveLogs = !m.SaveLogs
	case optionFollowOutput:
		m.LogFollow = !m.LogFollow
	case optionMaximizePane:
		m.Zoom = !m.Zoom
	}
	m.Notice = m.sessionOptionLabel(option) + " " + ternary(m.sessionOptionEnabled(option), "enabled", "disabled")
}

func (m Model) renderOptionsOverlay(width, _ int) string {
	m.normalizeOptionsSelection()
	boxWidth := min(68, max(52, width*2/3))
	contentWidth := max(1, boxWidth-4)
	rows := []string{m.renderOptionsTabs(contentWidth), ""}
	category := sessionOptionCategories[m.OptionsTab]
	for _, option := range category.options {
		rows = append(rows, m.renderSessionOptionRow(option, contentWidth))
	}
	rows = append(rows, "", subtleStyle.Render(strings.Repeat("─", contentWidth)))
	rows = append(rows, m.renderSessionOptionDetail(sessionOptions[m.OptionsPos], contentWidth)...)
	return strings.Join(boxLines(boxWidth, len(rows)+2, "Options · session", rows, false), "\n")
}

func (m Model) renderOptionsTabs(width int) string {
	labels := make([]string, 0, len(sessionOptionCategories))
	for index, category := range sessionOptionCategories {
		name := category.name
		if width < 40 {
			name = category.narrow
		}
		label := fmt.Sprintf("%d %s", index+1, name)
		if index == m.OptionsTab {
			label = sectionStyle.Render("[" + label + "]")
		} else {
			label = subtleStyle.Render(label)
		}
		labels = append(labels, label)
	}
	return " " + strings.Join(labels, "   ")
}

func (m Model) renderSessionOptionRow(option sessionOption, width int) string {
	selected := int(option) == m.OptionsPos
	label := m.sessionOptionLabel(option)
	if selected {
		label = sectionStyle.Render(label)
	}
	prefix := "  "
	if selected {
		prefix = sectionStyle.Render("› ")
	}
	return fixedStatusJoin(prefix+label, m.renderSessionOptionState(option), width)
}

func (m Model) renderSessionOptionState(option sessionOption) string {
	if m.sessionOptionLocked(option) {
		return subtleStyle.Render("◇ LOCKED")
	}
	if m.sessionOptionEnabled(option) {
		return sectionStyle.Render("● ON")
	}
	return subtleStyle.Render("○ OFF")
}

func (m Model) renderSessionOptionDetail(option sessionOption, width int) []string {
	detail := m.sessionOptionDescription(option)
	if option <= optionSaveLogs && m.hasActiveRuns() {
		detail = "Locked until active runs finish or are cancelled."
	} else if option == optionSaveLogs && m.DisableLogging {
		detail = "Unavailable while Capture output is off."
	}
	return []string{truncateVisible(detail, width)}
}

func (m Model) sessionOptionLocked(option sessionOption) bool {
	return option <= optionSaveLogs && m.hasActiveRuns() || option == optionSaveLogs && m.DisableLogging
}

func (m Model) sessionOptionEnabled(option sessionOption) bool {
	switch option {
	case optionSerial:
		return m.mode() == core.ModeSerial
	case optionFailFast:
		return m.FailFast
	case optionCaptureOutput:
		return !m.DisableLogging
	case optionSaveLogs:
		return m.SaveLogs
	case optionFollowOutput:
		return m.LogFollow
	case optionMaximizePane:
		return m.Zoom
	default:
		return false
	}
}

func (m Model) sessionOptionLabel(option sessionOption) string {
	switch option {
	case optionSerial:
		return "Serial execution"
	case optionFailFast:
		return "Stop on first failure"
	case optionCaptureOutput:
		return "Capture output"
	case optionSaveLogs:
		return "Save logs"
	case optionFollowOutput:
		return "Follow output"
	case optionMaximizePane:
		return "Maximize pane"
	default:
		return "Option"
	}
}

func (m Model) sessionOptionDescription(option sessionOption) string {
	switch option {
	case optionSerial:
		return "Runs targets one by one. Workers becomes auto."
	case optionFailFast:
		return "Stops queued targets after first failure."
	case optionCaptureOutput:
		return "Keeps live output in memory for this session."
	case optionSaveLogs:
		return "Persists captured output after each run."
	case optionFollowOutput:
		return "Keeps output pinned to newest line."
	case optionMaximizePane:
		return "Shows focused pane only."
	default:
		return ""
	}
}
