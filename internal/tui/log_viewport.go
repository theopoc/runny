package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
)

func newLogViewport() viewport.Model {
	model := viewport.New()
	model.MouseWheelEnabled = false
	model.FillHeight = false
	model.SoftWrap = true
	return model
}

func (m Model) configuredOutputViewport(targetID string, width, height int) viewport.Model {
	model := m.outputViewport
	model.SetWidth(max(1, width))
	model.SetHeight(max(1, height))

	lines := outputLines(m.Logs[targetID])
	styled := make([]string, 0, len(lines))
	for _, line := range lines {
		styled = append(styled, m.styleLogLine(line))
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

	width, height := m.outputViewportDimensions()
	m.outputViewport.SetWidth(width)
	m.outputViewport.SetHeight(height)
	m.outputViewport.SetContentLines(outputLines(m.Logs[m.Targets[m.Cursor].ID]))
}

func (m Model) outputViewportDimensions() (width, height int) {
	windowWidth := m.Width
	if windowWidth == 0 {
		windowWidth = 80
	}
	windowHeight := m.Height
	if windowHeight == 0 {
		windowHeight = 20
	}
	panelHeight, _, rightWidth := m.panelDimensions(windowWidth, windowHeight)
	panelWidth := rightWidth
	if m.Zoom || m.compactMode(windowWidth) {
		panelWidth = m.singlePanelWidth(windowWidth)
	}
	return max(1, panelWidth-4), max(1, panelHeight-2)
}

func (m Model) configuredHistoryLogViewport(width, height int) viewport.Model {
	model := m.historyLogViewport
	model.SetWidth(max(1, width))
	model.SetHeight(max(1, height))

	model.SetContentLines(outputLines(m.HistoryLog))
	return model
}

func (m *Model) syncHistoryLogViewport() {
	width, height := m.historyLogViewportDimensions()
	m.historyLogViewport.SetWidth(width)
	m.historyLogViewport.SetHeight(height)
	m.historyLogViewport.SetContentLines(outputLines(m.HistoryLog))
}

func (m Model) historyLogViewportDimensions() (width, height int) {
	windowWidth := m.Width
	if windowWidth == 0 {
		windowWidth = 80
	}
	windowHeight := m.Height
	if windowHeight == 0 {
		windowHeight = 20
	}
	panelHeight, _, _ := m.panelDimensions(windowWidth, windowHeight)
	boxWidth := windowWidth - 4
	if windowWidth >= 120 {
		boxWidth = min(boxWidth, 136)
	}
	boxWidth = max(52, min(windowWidth, boxWidth))
	contentWidth := max(1, boxWidth-4)
	logWidth := contentWidth
	if contentWidth >= 112 {
		dividerWidth := lipgloss.Width(" │ ")
		leftWidth := (contentWidth - dividerWidth) * 46 / 100
		logWidth = contentWidth - dividerWidth - leftWidth
	}
	return max(1, logWidth), max(1, panelHeight-4)
}

func viewportRows(model viewport.Model) []string {
	view := model.View()
	if view == "" {
		return nil
	}
	return strings.Split(view, "\n")
}
