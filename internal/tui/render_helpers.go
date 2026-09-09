package tui

import (
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const panelSeparator = "  "

const targetRegexFilterPrefix = "re:"

type targetFilterMode uint8

const (
	targetFilterModeFuzzy targetFilterMode = iota
	targetFilterModeExact
	targetFilterModeRegex
)

type filterMatchRange struct {
	start int
	end   int
}

type targetFilter struct {
	mode  targetFilterMode
	query string
	regex *regexp.Regexp
	err   error
}

func joinPanels(left []string, right []string) string {
	var b strings.Builder
	height := max(len(left), len(right))
	for i := range height {
		if i < len(left) {
			b.WriteString(left[i])
		} else {
			b.WriteString(strings.Repeat(" ", len(left[0])))
		}
		b.WriteString(panelSeparator)
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

func renderFloatingBox(width int, title string, rows []string) string {
	boxWidth := width * 80 / 100
	if boxWidth > 92 {
		boxWidth = 92
	}
	if boxWidth < 52 {
		boxWidth = min(width, 52)
	}
	return strings.Join(boxLines(boxWidth, len(rows)+2, title, rows, false), "\n")
}

func renderFittedFloatingBox(width int, title string, rows []string) string {
	return renderFittedFloatingBoxWithStyle(width, title, rows, panelStyle)
}

func renderDangerFittedFloatingBox(width int, title string, rows []string) string {
	return renderFittedFloatingBoxWithStyle(width, title, rows, dangerBorderStyle)
}

func renderFittedFloatingBoxWithStyle(width int, title string, rows []string, borderStyle lipgloss.Style) string {
	boxWidth := lipgloss.Width(title) + 6
	for _, row := range rows {
		boxWidth = max(boxWidth, ansi.StringWidth(row)+4)
	}
	boxWidth = min(width, boxWidth)
	return strings.Join(boxLinesWithTitle(boxWidth, len(rows)+2, title, rows, false, panelInactiveTitle, borderStyle), "\n")
}

func placeOverlay(background string, overlay string, width int) string {
	backgroundLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")
	overlayWidth := 0
	for _, line := range overlayLines {
		overlayWidth = max(overlayWidth, ansi.StringWidth(line))
	}
	if overlayWidth <= 0 || len(overlayLines) == 0 {
		return background
	}
	left := max(0, (width-overlayWidth)/2)
	top := max(0, (len(backgroundLines)-len(overlayLines))/2)
	for i, overlayLine := range overlayLines {
		row := top + i
		if row >= len(backgroundLines) {
			break
		}
		line := padRightANSI(backgroundLines[row], width)
		backgroundLines[row] = ansi.Cut(line, 0, left) + overlayLine + ansi.Cut(line, left+overlayWidth, width)
	}
	return strings.Join(backgroundLines, "\n")
}

func padRightANSI(value string, width int) string {
	current := ansi.StringWidth(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

func padLeftVisible(value string, width int) string {
	current := ansi.StringWidth(value)
	if current >= width {
		return value
	}
	return strings.Repeat(" ", width-current) + value
}

func centerANSI(value string, width int) string {
	current := ansi.StringWidth(value)
	if current >= width {
		return value
	}
	left := (width - current) / 2
	right := width - current - left
	return strings.Repeat(" ", left) + value + strings.Repeat(" ", right)
}

func boxLines(width int, height int, title string, rows []string, active bool) []string {
	titleStyle := panelInactiveTitle
	if active {
		titleStyle = panelTitleStyle
	}
	if !active && title != "Tasks" && title != "Output" {
		titleStyle = overlayTitleStyle
	}
	borderStyle := panelStyle
	if active {
		borderStyle = panelActiveStyle
	}
	return boxLinesWithTitle(width, height, title, rows, active, titleStyle, borderStyle)
}

func boxLinesWithTitle(width int, height int, title string, rows []string, active bool, titleStyle lipgloss.Style, borderStyle lipgloss.Style) []string {
	width = max(width, 5)
	height = max(height, 3)
	lines := make([]string, 0, height)
	border := panelBorder(active)
	if title == "" {
		lines = append(lines, borderStyle.Render(border.topLeft+strings.Repeat(border.horizontal, width-2)+border.topRight))
	} else {
		title = truncateVisible(title, width-5)
		titleText := " " + titleStyle.Render(title) + " "
		topFill := max(0, width-lipgloss.Width(titleText)-3)
		lines = append(lines, borderStyle.Render(border.topLeft+border.horizontal)+titleText+borderStyle.Render(strings.Repeat(border.horizontal, topFill)+border.topRight))
	}
	contentWidth := width - 4
	for i := 0; i < height-2; i++ {
		row := ""
		if i < len(rows) {
			row = rows[i]
		}
		lines = append(lines, borderStyle.Render(border.vertical)+" "+formatBoxRow(row, contentWidth)+" "+borderStyle.Render(border.vertical))
	}
	lines = append(lines, borderStyle.Render(border.bottomLeft+strings.Repeat(border.horizontal, width-2)+border.bottomRight))
	return lines
}

func formatBoxRow(row string, width int) string {
	if ansi.StringWidth(row) > width {
		row = ansi.Cut(row, 0, max(0, width-1)) + "~"
	}
	return padRightANSI(row, width)
}

type panelBorderGlyphs struct {
	topLeft     string
	topRight    string
	bottomLeft  string
	bottomRight string
	horizontal  string
	vertical    string
}

func panelBorder(active bool) panelBorderGlyphs {
	if active {
		return panelBorderGlyphs{
			topLeft:     "╔",
			topRight:    "╗",
			bottomLeft:  "╚",
			bottomRight: "╝",
			horizontal:  "═",
			vertical:    "║",
		}
	}
	return panelBorderGlyphs{
		topLeft:     "╭",
		topRight:    "╮",
		bottomLeft:  "╰",
		bottomRight: "╯",
		horizontal:  "─",
		vertical:    "│",
	}
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
	const statusWidth = 12
	const gap = 2
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

func parseTargetFilter(query string) targetFilter {
	if strings.HasPrefix(query, targetRegexFilterPrefix) {
		pattern := strings.TrimPrefix(query, targetRegexFilterPrefix)
		filter := targetFilter{mode: targetFilterModeRegex, query: pattern}
		if pattern != "" {
			filter.regex, filter.err = regexp.Compile(pattern)
		}
		return filter
	}
	query, exact := filterQuery(query)
	mode := targetFilterModeFuzzy
	if exact {
		mode = targetFilterModeExact
	}
	return targetFilter{mode: mode, query: query}
}

func (f targetFilter) active() bool {
	return f.query != "" || f.mode == targetFilterModeExact
}

func (f targetFilter) matches(value string) bool {
	if !f.active() {
		return true
	}
	if f.err != nil {
		return false
	}
	if f.mode == targetFilterModeRegex {
		return f.regex.MatchString(value)
	}
	if containsFold(value, f.query) {
		return true
	}
	return f.mode == targetFilterModeFuzzy && len(fuzzyIndexesFold(value, f.query)) > 0
}

func (f targetFilter) matchRanges(value string) []filterMatchRange {
	if !f.active() || f.err != nil || f.mode != targetFilterModeRegex {
		return nil
	}
	indexes := f.regex.FindAllStringIndex(value, -1)
	ranges := make([]filterMatchRange, 0, len(indexes))
	for _, index := range indexes {
		if index[0] == index[1] {
			continue
		}
		ranges = append(ranges, filterMatchRange{start: index[0], end: index[1]})
	}
	return ranges
}

func (f targetFilter) modeLabel() string {
	switch f.mode {
	case targetFilterModeExact:
		return "exact"
	case targetFilterModeRegex:
		return "regex"
	default:
		return "fuzzy"
	}
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
