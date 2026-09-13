package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/theopoc/runny/internal/core"
)

func changeText(summary core.ChangeSummary, status core.Status) string {
	if status != core.StatusSucceeded {
		return ""
	}
	if !summary.Known || summary.Phase != "plan" && summary.Phase != "apply" {
		return "—"
	}
	phase := "P"
	if summary.Phase == "apply" {
		phase = "A"
	}
	return fmt.Sprintf("%s +%d ~%d -%d", phase, summary.Add, summary.Change, summary.Destroy)
}

func styleChanges(text string) string {
	parts := strings.Fields(text)
	for i, part := range parts {
		switch part[0] {
		case '+':
			parts[i] = metricSuccessStyle.Render(part)
		case '~':
			parts[i] = metricRunningStyle.Render(part)
		case '-':
			parts[i] = metricFailedStyle.Render(part)
		default:
			parts[i] = subtleStyle.Render(part)
		}
	}
	return strings.Join(parts, " ")
}

func compactStatus(status core.Status) string {
	switch status {
	case core.StatusSucceeded:
		return "✓"
	case core.StatusFailed:
		return "×"
	case core.StatusCancelled:
		return "!"
	case core.StatusRunning:
		return "●"
	case core.StatusQueued:
		return "◌"
	default:
		return "·"
	}
}

func (m Model) hasChanges() bool {
	for _, c := range m.Changes {
		if c.Detected {
			return true
		}
	}
	return false
}

func (m Model) changeWidth() int {
	width := 10
	for id, c := range m.Changes {
		width = max(width, len(changeText(c, m.Status[id])))
	}
	return width
}

type changeColumns struct {
	left, status, time, changes int
	continued                   bool
}

func changesLayout(width, changesWidth int) changeColumns {
	c := changeColumns{status: targetStatusWidth, time: targetTimeWidth, changes: changesWidth}
	c.left = width - c.status - c.time - c.changes - 5
	if c.left < 14 {
		c.time = 0
		c.left = width - c.status - c.changes - 4
	}
	if c.left < 14 {
		c.status = 1
		c.left = width - c.status - c.changes - 4
	}
	if c.left < 10 {
		c.continued = true
		c.status, c.time = targetStatusWidth, 0
		c.left = max(1, width-c.status-2)
	}
	return c
}

// Continuations contain counters only; they do not introduce selectable units.
func wrappedChanges(text string, width int) []string {
	var rows []string
	line := "  "
	for _, token := range strings.Fields(text) {
		if len(line) > 2 && lipgloss.Width(line)+1+len(token) > width {
			rows = append(rows, line)
			line = "  "
		}
		if len(line) > 2 {
			line += " "
		}
		line += token
	}
	if len(line) > 2 {
		rows = append(rows, line)
	}
	return rows
}

func (m Model) changeRowHeight(width int) int {
	if !m.hasChanges() || !changesLayout(width, m.changeWidth()).continued {
		return 1
	}
	height := 2
	for id, c := range m.Changes {
		height = max(height, 1+len(wrappedChanges(changeText(c, m.Status[id]), width)))
	}
	return height
}

func (m Model) directoryContentWidth() int {
	width := max(60, m.Width)
	_, left, _ := m.panelDimensions(width, m.Height)
	if m.Zoom || m.compactMode(width) {
		left = m.singlePanelWidth(width)
	}
	return max(1, left-4)
}

func (m Model) changeTaskHeader(width int) string {
	c := changesLayout(width, m.changeWidth())
	if c.continued {
		return padRightVisible(truncateVisible("DIRECTORY / CHANGES", c.left), c.left) + "  " + padRightVisible("STATUS", c.status)
	}
	line := padRightVisible(truncateVisible("DIRECTORY", c.left), c.left) + "  "
	status := "STATUS"
	if c.status == 1 {
		status = "S"
	}
	line += padRightVisible(status, c.status)
	if c.time > 0 {
		line += " " + padLeftVisible("TIME", c.time)
	}
	return line + "  " + padLeftVisible("CHANGES", c.changes)
}

func (m Model) renderChangeTargetRow(left string, target core.Target, status core.Status, active, partial bool, width int) string {
	c := changesLayout(width, m.changeWidth())
	left = padRightVisible(truncateVisible(left, c.left), c.left)
	if active {
		left = rowActiveStyle.Render(left + "  ")
	} else {
		left += "  "
	}
	statusText := m.renderRowStatus(status)
	if c.status == 1 {
		statusText = compactStatus(status)
		if style, ok := statusStyles[status]; ok {
			statusText = style.Render(statusText)
		}
	}
	line := left + statusText
	text := changeText(m.Changes[target.ID], status)
	if c.time > 0 {
		line += " " + m.renderRowTime(target.ID, status)
	}
	if !c.continued {
		line += "  " + padLeftVisible(styleChanges(text), c.changes)
	}
	rows := []string{line}
	if c.continued {
		for _, row := range wrappedChanges(text, width) {
			rows = append(rows, "  "+styleChanges(strings.TrimSpace(row)))
		}
		for len(rows) < m.changeRowHeight(width) {
			rows = append(rows, "")
		}
	}
	if !active {
		for i, row := range rows {
			switch {
			case target.Selected:
				rows[i] = rowSelectedStyle.Render(padRightVisible(row, width))
			case partial:
				rows[i] = rowPartialStyle.Render(padRightVisible(row, width))
			case status == core.StatusRunning:
				rows[i] = rowRunningStyle.Render(padRightVisible(row, width))
			}
		}
	}
	return strings.Join(rows, "\n")
}
