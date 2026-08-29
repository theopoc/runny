package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
)

func newLogViewport() viewport.Model {
	model := viewport.New()
	model.MouseWheelEnabled = false
	model.FillHeight = false
	return model
}

func (m Model) configuredOutputViewport(targetID string, width, height int) viewport.Model {
	model := m.outputViewport
	model.SetWidth(max(1, width))
	model.SetHeight(max(1, height))

	lines := outputLines(m.Logs[targetID])
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		styled = append(styled, formatBoxRow(m.styleLogLine(line), max(1, width)))
	}
	model.SetContentLines(styled)
	if m.LogFollow {
		model.GotoBottom()
	}
	return model
}

func (m *Model) syncOutputViewport() {
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) {
		m.outputViewport.SetContentLines(nil)
		m.outputViewport.SetYOffset(0)
		return
	}

	visible := max(1, panelHeightForWindow(m.Height)-2)
	m.outputViewport.SetHeight(visible)
	m.outputViewport.SetContentLines(outputLines(m.Logs[m.Targets[m.Cursor].ID]))
}

func (m Model) configuredHistoryLogViewport(width, height int) viewport.Model {
	model := m.historyLogViewport
	model.SetWidth(max(1, width))
	model.SetHeight(max(1, height))

	lines := outputLines(m.HistoryLog)
	truncated := make([]string, 0, len(lines))
	for _, line := range lines {
		truncated = append(truncated, truncateVisible(line, width))
	}
	model.SetContentLines(truncated)
	return model
}

func (m *Model) syncHistoryLogViewport() {
	panelHeight, _, _ := m.panelDimensions(m.Width, m.Height)
	bodyHeight := max(1, panelHeight-3)
	m.historyLogViewport.SetHeight(max(1, bodyHeight-1))
	m.historyLogViewport.SetContentLines(outputLines(m.HistoryLog))
}

func viewportRows(model viewport.Model) []string {
	view := model.View()
	if view == "" {
		return nil
	}
	return strings.Split(view, "\n")
}
