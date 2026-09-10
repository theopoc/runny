package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"
)

type outputSelection struct {
	targetID      string
	snapshot      string
	truncated     bool
	rows          []outputVisualRow
	width         int
	offset        int
	restoreOffset int
	anchor        outputPoint
	head          outputPoint
	dragging      bool
	moved         bool
	follow        bool
	pendingLive   int
}

func (s outputSelection) active() bool {
	return s.targetID != "" && (s.dragging || s.moved)
}

func (s outputSelection) exists() bool {
	return s.targetID != ""
}

func (s outputSelection) selectedRange() (int, int, bool) {
	if !s.moved {
		return 0, 0, false
	}
	start := min(s.anchor.start, s.head.start)
	end := max(s.anchor.end, s.head.end)
	if start < 0 || end > len(s.snapshot) || start >= end {
		return 0, 0, false
	}
	return start, end, true
}

func (s outputSelection) text() string {
	start, end, ok := s.selectedRange()
	if !ok {
		return ""
	}
	return s.snapshot[start:end]
}

func (s *outputSelection) rebuild(width int) {
	width = max(1, width)
	if s.width == width && s.rows != nil {
		return
	}
	top := -1
	if s.offset >= 0 && s.offset < len(s.rows) {
		top = s.rows[s.offset].start
	}
	s.rows = projectOutputRows(s.snapshot, width, s.truncated)
	s.width = width
	if top >= 0 {
		for i, row := range s.rows {
			if (row.start < row.end && row.start <= top && top < row.end) || (row.start == row.end && row.start == top) {
				s.offset = i
				break
			}
		}
	}
}

type outputRect struct {
	x      int
	y      int
	width  int
	height int
}

func (m *Model) setCursor(cursor int) {
	if cursor != m.Cursor {
		m.clearOutputSelection()
	}
	m.Cursor = cursor
}

func (m Model) outputContentRect() (outputRect, bool) {
	if m.Width < 60 || m.Height < 20 || m.hasOverlay() {
		return outputRect{}, false
	}
	panelHeight, leftWidth, rightWidth := m.panelDimensions(m.Width, m.Height)
	panelX := leftWidth + lipgloss.Width(panelSeparator)
	panelWidth := rightWidth
	if m.Zoom || m.compactMode(m.Width) {
		if m.Focus != FocusLogs {
			return outputRect{}, false
		}
		panelX = 0
		panelWidth = m.singlePanelWidth(m.Width)
	}
	panelTop := strings.Count(m.renderPanelPrefix(m.Width), "\n")
	return outputRect{x: panelX + 2, y: panelTop + 1, width: max(1, panelWidth-4), height: max(1, panelHeight-2)}, true
}

func (m Model) newOutputSelection() (outputSelection, bool) {
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) {
		return outputSelection{}, false
	}
	rect, ok := m.outputContentRect()
	if !ok {
		return outputSelection{}, false
	}
	targetID := m.Targets[m.Cursor].ID
	truncated := m.liveLogTruncated[targetID]
	selection := outputSelection{
		targetID:  targetID,
		snapshot:  normalizeOutputText(m.Logs[targetID], truncated),
		truncated: truncated,
		width:     rect.width,
		follow:    m.LogFollow,
	}
	selection.rows = projectOutputRows(selection.snapshot, rect.width, truncated)
	viewport := m.configuredOutputViewport(targetID, rect.width, rect.height)
	selection.offset = viewport.YOffset()
	selection.restoreOffset = selection.offset
	selection.offset = min(selection.offset, max(0, len(selection.rows)-rect.height))
	return selection, true
}

func (m *Model) startOutputSelection(x, y int) bool {
	restoreFollow := m.LogFollow
	restoreOffset := m.outputViewport.YOffset()
	if m.outputSelection.exists() {
		restoreFollow = m.outputSelection.follow
		restoreOffset = m.outputSelection.restoreOffset
	}
	selection, ok := m.newOutputSelection()
	if !ok {
		return false
	}
	selection.follow = restoreFollow
	selection.restoreOffset = restoreOffset
	point, ok := m.outputPointAtScreen(selection, x, y)
	if !ok {
		m.clearOutputSelection()
		return false
	}
	selection.anchor = point
	selection.head = point
	selection.dragging = true
	m.outputSelection = selection
	m.LogFollow = false
	m.outputViewport.SetYOffset(selection.offset)
	return true
}

func (m *Model) extendOutputSelection(x, y int) {
	if !m.outputSelection.dragging {
		return
	}
	point, ok := m.outputPointAtScreen(m.outputSelection, x, y)
	if !ok {
		return
	}
	m.outputSelection.head = point
	m.outputSelection.moved = point != m.outputSelection.anchor
}

func (m *Model) finishOutputSelection() {
	if !m.outputSelection.dragging {
		return
	}
	m.outputSelection.dragging = false
	if !m.outputSelection.moved {
		m.clearOutputSelection()
	}
}

func (m Model) outputPointAtScreen(selection outputSelection, x, y int) (outputPoint, bool) {
	rect, ok := m.outputContentRect()
	if !ok || x < rect.x || x >= rect.x+rect.width || y < rect.y || y >= rect.y+rect.height {
		return outputPoint{}, false
	}
	if m.copyToastContains(x, y) {
		return outputPoint{}, false
	}
	rowIndex := selection.offset + y - rect.y
	if rowIndex < 0 || rowIndex >= len(selection.rows) {
		return outputPoint{}, false
	}
	return outputPointAt(selection.snapshot, selection.rows[rowIndex], x-rect.x)
}

func (m *Model) clearOutputSelection() {
	if !m.outputSelection.exists() {
		m.outputSelection = outputSelection{}
		return
	}
	follow := m.outputSelection.follow
	offset := m.outputSelection.restoreOffset
	m.outputSelection = outputSelection{}
	m.LogFollow = follow
	m.syncOutputViewport()
	if follow {
		m.outputViewport.GotoBottom()
	} else {
		m.outputViewport.SetYOffset(offset)
	}
}

func (m *Model) refreshOutputSelection() {
	if !m.outputSelection.active() {
		return
	}
	width, height := m.outputViewportDimensions()
	m.outputSelection.rebuild(width)
	m.outputSelection.offset = min(m.outputSelection.offset, max(0, len(m.outputSelection.rows)-height))
}

func (m *Model) scrollOutputSelection(delta int) {
	if !m.outputSelection.active() {
		return
	}
	_, height := m.outputViewportDimensions()
	maxOffset := max(0, len(m.outputSelection.rows)-height)
	m.outputSelection.offset = max(0, min(maxOffset, m.outputSelection.offset+delta))
}

func (m Model) renderSelectedOutputRows(width, height int) []string {
	selection := m.outputSelection
	selection.rebuild(width)
	start, end, selected := selection.selectedRange()
	offset := min(selection.offset, max(0, len(selection.rows)-height))
	rows := make([]string, 0, height)
	for _, row := range selection.rows[offset:min(len(selection.rows), offset+height)] {
		if row.synthetic != "" {
			rows = append(rows, subtleStyle.Render(row.synthetic))
			continue
		}
		rows = append(rows, m.renderSelectedOutputRow(selection.snapshot, row, start, end, selected))
	}
	return rows
}

func (m Model) renderSelectedOutputRow(text string, row outputVisualRow, selectionStart, selectionEnd int, selected bool) string {
	if row.start == row.end {
		return ""
	}
	line := text[row.start:row.end]
	base := m.logStyle(line)
	if !selected || selectionEnd <= row.start || selectionStart >= row.end {
		return base.Render(line)
	}
	var rendered strings.Builder
	graphemes := uniseg.NewGraphemes(line)
	for graphemes.Next() {
		from, to := graphemes.Positions()
		absoluteStart := row.start + from
		absoluteEnd := row.start + to
		style := base
		if absoluteStart < selectionEnd && absoluteEnd > selectionStart {
			style = outputSelectionStyle
		}
		rendered.WriteString(style.Render(graphemes.Str()))
	}
	return rendered.String()
}
