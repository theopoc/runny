package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	defaultPanelSplitNumerator   = 42
	defaultPanelSplitDenominator = 100
	minimumTasksPanelWidth       = 36
	minimumOutputPanelWidth      = 32
	panelLayoutOverhead          = 4
)

func panelWidths(width, splitWidth, splitBasis int) (leftWidth, rightWidth int) {
	leftWidth = width * defaultPanelSplitNumerator / defaultPanelSplitDenominator
	if splitBasis > 0 {
		leftWidth = width * splitWidth / splitBasis
	}
	leftWidth = max(minimumTasksPanelWidth, leftWidth)
	rightWidth = width - leftWidth - panelLayoutOverhead
	if rightWidth < minimumOutputPanelWidth {
		rightWidth = minimumOutputPanelWidth
		leftWidth = width - rightWidth - panelLayoutOverhead
	}
	return leftWidth, rightWidth
}

func (m Model) canResizePanes() bool {
	return m.Width >= 100 && m.Height >= 20 && !m.Zoom && !m.hasOverlay()
}

func (m Model) paneDividerAt(x, y int) bool {
	if !m.canResizePanes() {
		return false
	}
	panelHeight, leftWidth, _ := m.panelDimensions(m.Width, m.Height)
	panelTop := strings.Count(m.renderPanelPrefix(m.Width), "\n")
	separatorWidth := lipgloss.Width(panelSeparator)
	return x >= leftWidth && x < leftWidth+separatorWidth && y >= panelTop && y < panelTop+panelHeight
}

func (m *Model) startPaneResize(x, y int) bool {
	if !m.paneDividerAt(x, y) {
		return false
	}
	_, leftWidth, _ := m.panelDimensions(m.Width, m.Height)
	m.paneResizeActive = true
	m.paneResizeGrabOffset = x - leftWidth
	m.paneResizeStartX = x
	m.paneResizeMoved = false
	return true
}

func (m *Model) resizePanesAt(x int) {
	if !m.paneResizeActive || !m.canResizePanes() {
		return
	}
	if x == m.paneResizeStartX && !m.paneResizeMoved {
		return
	}
	m.paneResizeMoved = true
	desiredLeft := x - m.paneResizeGrabOffset
	leftWidth, _ := panelWidths(m.Width, desiredLeft, m.Width)
	m.panelSplitWidth = leftWidth
	m.panelSplitBasis = m.Width
	m.syncOutputViewport()
	m.refreshOutputSelection()
}
