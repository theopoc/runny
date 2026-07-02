package tui

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07]*(\x07|\x1b\\)`)

func stripANSI(value string) string {
	clean := ansiPattern.ReplaceAllString(value, "")
	lines := strings.Split(clean, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func maxLineWidth(value string) int {
	maxWidth := 0
	for _, line := range strings.Split(stripANSI(value), "\n") {
		if width := lipgloss.Width(line); width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
