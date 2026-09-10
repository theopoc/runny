package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type commandVisualCell struct {
	text     string
	width    int
	cursor   bool
	selected bool
}

func (m *Model) ensureCommandCursor() {
	m.commandLineEditor().ensure()
}

func (m *Model) moveCommandCursor(delta int, selecting bool) {
	m.commandLineEditor().move(delta, selecting)
}

func (m *Model) moveCommandCursorByWord(direction int, selecting bool) {
	m.commandLineEditor().moveByWord(direction, selecting)
}

func (m *Model) setCommandCursor(position int, selecting bool) {
	m.commandLineEditor().setCursor(position, selecting)
}

func (m *Model) moveCommandCursorToEnd() {
	m.commandLineEditor().moveToEnd()
}

func (m Model) commandSelectionRange() (int, int, bool) {
	return m.commandLineEditor().selectionRange()
}

func (m Model) hasCommandSelection() bool {
	_, _, ok := m.commandSelectionRange()
	return ok
}

func (m Model) selectedCommandText() string {
	return m.commandLineEditor().selectedText()
}

func (m *Model) deleteCommandSelection() bool {
	return m.commandLineEditor().deleteSelection()
}

func (m *Model) insertCommandText(value string) {
	m.commandLineEditor().insert(value)
	m.resetCommandHistoryNavigation()
}

func (m *Model) deleteCommandBackward() {
	m.commandLineEditor().deleteBackward()
	m.resetCommandHistoryNavigation()
}

func (m *Model) deleteCommandForward() {
	m.commandLineEditor().deleteForward()
	m.resetCommandHistoryNavigation()
}

func (m *Model) deleteCommandWordBackward() {
	m.commandLineEditor().deleteWordBackward()
	m.resetCommandHistoryNavigation()
}

func (m *Model) commandLineEditor() lineEditor {
	return newLineEditor(&m.Command, &m.commandCursor, &m.commandCursorValid, &m.commandSelection, &m.commandSelecting)
}

func (m *Model) filterLineEditor() lineEditor {
	return newLineEditor(&m.Filter, &m.filterCursor, &m.filterCursorValid, &m.filterSelection, &m.filterSelecting)
}

func (m Model) renderCommandInputValue(width int) string {
	editor := m.commandLineEditor()
	cursor := editor.cursorPosition()
	graphemes := splitGraphemes(m.Command)
	viewportStart, viewportEnd := lineEditorViewport(graphemes, cursor, width, cursor == len(graphemes))
	selectionStart, selectionEnd, selected := m.commandSelectionRange()
	var value strings.Builder
	for i := viewportStart; i < viewportEnd; i++ {
		style := commandInputStyle
		if selected && i >= selectionStart && i < selectionEnd {
			style = commandSelectionStyle
		}
		isSelected := selected && i >= selectionStart && i < selectionEnd
		if i == cursor && !isSelected {
			style = style.Reverse(true)
		}
		value.WriteString(style.Render(graphemes[i]))
	}
	if cursor == len(graphemes) {
		value.WriteString(commandInputStyle.Reverse(true).Render(" "))
	}
	return value.String()
}

func (m Model) renderFilterInputValue(width int) string {
	editor := m.filterLineEditor()
	cursor := editor.cursorPosition()
	graphemes := splitGraphemes(m.Filter)
	reserveCursor := cursor == len(graphemes)
	contentWidth := max(1, width)
	start, end := 0, 0
	leftHidden, rightHidden := false, false
	for range 3 {
		start, end = lineEditorViewport(graphemes, cursor, contentWidth, reserveCursor)
		leftHidden = start > 0
		rightHidden = end < len(graphemes)
		markerWidth := 0
		if leftHidden {
			markerWidth++
		}
		if rightHidden {
			markerWidth++
		}
		nextWidth := max(1, width-markerWidth)
		if nextWidth == contentWidth {
			break
		}
		contentWidth = nextWidth
	}

	selectionStart, selectionEnd, selected := editor.selectionRange()
	var value strings.Builder
	if leftHidden {
		value.WriteString(commandInputBorderStyle.Render("‹"))
	}
	for i := start; i < end; i++ {
		style := commandInputStyle
		if selected && i >= selectionStart && i < selectionEnd {
			style = commandSelectionStyle
		}
		isSelected := selected && i >= selectionStart && i < selectionEnd
		if i == cursor && !isSelected {
			style = style.Reverse(true)
		}
		value.WriteString(style.Render(graphemes[i]))
	}
	if cursor == len(graphemes) {
		value.WriteString(commandInputStyle.Reverse(true).Render(" "))
	}
	if rightHidden {
		value.WriteString(commandInputBorderStyle.Render("›"))
	}
	return value.String()
}

func (m Model) renderWrappedCommandInput(width int, maxRows int) (rows []string, hiddenAbove bool, hiddenBelow bool) {
	width = max(1, width)
	maxRows = max(1, maxRows)
	editor := m.commandLineEditor()
	cursor := editor.cursorPosition()
	selectionStart, selectionEnd, selected := editor.selectionRange()

	graphemes := splitGraphemes(m.Command)
	cells := make([]commandVisualCell, 0, len(graphemes)+1)
	for index, text := range graphemes {
		cell := commandVisualCell{
			text:  text,
			width: max(0, ansi.StringWidth(text)),
		}
		cell.cursor = cursor == index
		cell.selected = selected && index >= selectionStart && index < selectionEnd
		cells = append(cells, cell)
	}
	if cursor == len(graphemes) {
		cells = append(cells, commandVisualCell{
			text:   " ",
			width:  1,
			cursor: true,
		})
	}

	visualRows := make([][]commandVisualCell, 1)
	rowWidths := []int{0}
	cursorRow := 0
	for _, cell := range cells {
		row := len(visualRows) - 1
		if rowWidths[row] > 0 && rowWidths[row]+cell.width > width {
			visualRows = append(visualRows, nil)
			rowWidths = append(rowWidths, 0)
			row++
		}
		visualRows[row] = append(visualRows[row], cell)
		rowWidths[row] += cell.width
		if cell.cursor {
			cursorRow = row
		}
	}

	start := 0
	if len(visualRows) > maxRows {
		start = max(0, cursorRow-maxRows/2)
		start = min(start, len(visualRows)-maxRows)
	}
	end := min(len(visualRows), start+maxRows)
	hiddenAbove = start > 0
	hiddenBelow = end < len(visualRows)
	rows = make([]string, 0, end-start)
	for _, visualRow := range visualRows[start:end] {
		var rendered strings.Builder
		for _, cell := range visualRow {
			style := commandInputStyle
			if cell.selected {
				style = commandSelectionStyle
			}
			if cell.cursor && !cell.selected {
				style = style.Reverse(true)
			}
			rendered.WriteString(style.Render(cell.text))
		}
		rows = append(rows, rendered.String())
	}
	return rows, hiddenAbove, hiddenBelow
}

func lineEditorViewport(graphemes []string, cursor int, width int, reserveCursor bool) (int, int) {
	cursor = min(max(cursor, 0), len(graphemes))
	if width <= 0 {
		return cursor, cursor
	}
	remaining := width
	if reserveCursor {
		remaining--
	}
	remaining = max(0, remaining)
	start, end := cursor, cursor
	rightBudget := remaining / 2
	for end < len(graphemes) {
		cellWidth := max(0, ansi.StringWidth(graphemes[end]))
		if cellWidth > rightBudget {
			break
		}
		rightBudget -= cellWidth
		remaining -= cellWidth
		end++
	}
	for start > 0 {
		cellWidth := max(0, ansi.StringWidth(graphemes[start-1]))
		if cellWidth > remaining {
			break
		}
		remaining -= cellWidth
		start--
	}
	for end < len(graphemes) {
		cellWidth := max(0, ansi.StringWidth(graphemes[end]))
		if cellWidth > remaining {
			break
		}
		remaining -= cellWidth
		end++
	}
	return start, end
}
