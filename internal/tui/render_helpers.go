package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

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
	boxWidth := width - 4
	if boxWidth > 92 {
		boxWidth = 92
	}
	if boxWidth < 52 {
		boxWidth = width
	}
	boxHeight := len(rows) + 2
	box := strings.Join(boxLines(boxWidth, boxHeight, title, rows, false), "\n")
	if boxWidth >= width {
		return box
	}
	padding := strings.Repeat(" ", max(0, (width-boxWidth)/2))
	lines := strings.Split(box, "\n")
	for i := range lines {
		lines[i] = padding + lines[i]
	}
	return strings.Join(lines, "\n")
}

func boxLines(width int, height int, title string, rows []string, active bool) []string {
	width = max(width, len(title)+6)
	height = max(height, 3)
	lines := make([]string, 0, height)
	titleStyle := panelInactiveTitle
	if active {
		titleStyle = panelTitleStyle
	}
	if !active && title != "Tasks" && title != "Preview" {
		titleStyle = overlayTitleStyle
	}
	titleText := " " + titleStyle.Render(title) + " "
	topFill := max(0, width-lipgloss.Width(titleText)-2)
	borderStyle := panelStyle
	if active {
		borderStyle = panelActiveStyle
	}
	lines = append(lines, borderStyle.Render("╭─")+titleText+borderStyle.Render(strings.Repeat("─", topFill)+"╮"))
	contentWidth := width - 4
	for i := 0; i < height-2; i++ {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		lines = append(lines, borderStyle.Render("│")+" "+padRightVisible(truncateVisible(row, contentWidth), contentWidth)+" "+borderStyle.Render("│"))
	}
	lines = append(lines, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return lines
}

func clipOverlayRows(rows []string, boxHeight int) []string {
	contentHeight := max(1, boxHeight-2)
	if len(rows) <= contentHeight {
		return rows
	}
	clipped := append([]string(nil), rows[:contentHeight]...)
	if contentHeight > 0 {
		clipped[contentHeight-1] = subtleStyle.Render("... more, resize terminal for full view")
	}
	return clipped
}

func visibleJoin(left string, right string, width int) string {
	space := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return truncateVisible(left+strings.Repeat(" ", space)+right, width)
}

func fixedStatusJoin(left string, status string, width int) string {
	statusWidth := 12
	gap := 2
	if width < statusWidth+gap+8 {
		return truncateVisible(left, width)
	}
	leftWidth := width - statusWidth - gap
	left = truncateVisible(left, leftWidth)
	return padRightVisible(left, leftWidth) + strings.Repeat(" ", gap) + truncateVisible(status, statusWidth)
}

func truncateVisible(value string, width int) string {
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return ""
	}
	var b strings.Builder
	for _, r := range value {
		if lipgloss.Width(b.String()+string(r)+"~") > width {
			break
		}
		b.WriteRune(r)
	}
	b.WriteString("~")
	return b.String()
}

func padRightVisible(value string, width int) string {
	current := lipgloss.Width(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

func containsFold(value string, query string) bool {
	return indexFold(value, query) >= 0
}

func indexFold(value string, query string) int {
	if query == "" {
		return 0
	}
	return strings.Index(strings.ToLower(value), strings.ToLower(query))
}

func filterQuery(query string) (string, bool) {
	if strings.HasPrefix(query, "'") {
		return strings.TrimPrefix(query, "'"), true
	}
	return query, false
}

func filterMatches(value string, query string) bool {
	query, exact := filterQuery(query)
	if query == "" {
		return true
	}
	if containsFold(value, query) {
		return true
	}
	return !exact && len(fuzzyIndexesFold(value, query)) > 0
}

func fuzzyIndexesFold(value string, query string) []int {
	if query == "" {
		return nil
	}
	valueRunes := []rune(strings.ToLower(value))
	queryRunes := []rune(strings.ToLower(query))
	byteIndexes := make([]int, 0, len(valueRunes))
	for index := range value {
		byteIndexes = append(byteIndexes, index)
	}
	matches := make([]int, 0, len(queryRunes))
	queryPos := 0
	for valuePos, valueRune := range valueRunes {
		if queryPos >= len(queryRunes) {
			break
		}
		if valueRune == queryRunes[queryPos] {
			matches = append(matches, byteIndexes[valuePos])
			queryPos++
		}
	}
	if queryPos != len(queryRunes) {
		return nil
	}
	return matches
}

func ternary(condition bool, yes string, no string) string {
	if condition {
		return yes
	}
	return no
}

func trimLastWord(value string) string {
	value = strings.TrimRight(value, " ")
	index := strings.LastIndex(value, " ")
	if index < 0 {
		return ""
	}
	return strings.TrimRight(value[:index], " ")
}
