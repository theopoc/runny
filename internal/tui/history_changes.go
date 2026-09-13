package tui

import (
	"fmt"
	"strings"

	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
)

func historyHasChanges(targets []history.TargetEntry) bool {
	for _, target := range targets {
		if target.Changes.Detected {
			return true
		}
	}
	return false
}

func (m Model) appendHistoryChanges(rows []string, targets []history.TargetEntry, width, height int) []string {
	layoutModel := Model{Changes: map[string]core.ChangeSummary{}, Status: map[string]core.Status{}}
	for _, t := range targets {
		layoutModel.Changes[t.ID] = t.Changes
		layoutModel.Status[t.ID] = t.Status
	}
	c := changesLayout(width, layoutModel.changeWidth())
	header := strings.Replace(layoutModel.changeTaskHeader(width), "DIRECTORY", "TARGET   ", 1)
	if c.status > 1 {
		header = strings.Replace(header, "STATUS     ", "STATUS/EXIT", 1)
	}
	rows = append(rows, subtleStyle.Render(header))
	rowHeight := layoutModel.changeRowHeight(width)
	remaining := max(1, (height-len(rows)-1)/rowHeight)
	position := clampHistoryIndex(m.HistoryTargetPos, len(targets))
	start := historyWindowStart(position, remaining, len(targets))
	for i := start; i < min(len(targets), start+remaining); i++ {
		target := targets[i]
		prefix := "  "
		if i == position {
			prefix = "› "
		}
		left := padRightVisible(truncateVisible(prefix+m.highlightHistoryMatch(target.RelPath), c.left), c.left)
		status := historyTargetStatus(target.Status)
		if target.Status == core.StatusSucceeded || target.Status == core.StatusFailed {
			status = fmt.Sprintf("%s/%d", status, target.ExitCode)
		}
		if c.status == 1 {
			status = compactStatus(target.Status)
		}
		line := left + "  " + padRightVisible(truncateVisible(status, c.status), c.status)
		if c.time > 0 {
			line += " " + padLeftVisible(truncateVisible(formatHistoryDuration(target.Started, target.Ended), c.time), c.time)
		}
		text := changeText(target.Changes, target.Status)
		if !c.continued {
			line += "  " + padLeftVisible(styleChanges(text), c.changes)
		}
		targetRows := []string{line}
		if c.continued {
			for _, row := range wrappedChanges(text, width) {
				targetRows = append(targetRows, "  "+styleChanges(strings.TrimSpace(row)))
			}
			for len(targetRows) < rowHeight {
				targetRows = append(targetRows, "")
			}
		}
		for _, row := range targetRows {
			if i == position {
				row = paletteActiveStyle.Render(padRightVisible(row, width))
			}
			rows = append(rows, row)
		}
	}
	if len(targets) > 0 && targets[position].Error != "" {
		rows = append(rows, metricFailedStyle.Render("error")+"  "+truncateVisible(targets[position].Error, max(1, width-7)))
	}
	return rows
}
