package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

func (m *Model) ensureCommandCursor() {
	if !m.commandCursorValid {
		m.commandCursor = len([]rune(m.Command))
		m.commandCursorValid = true
	}
	length := len([]rune(m.Command))
	m.commandCursor = min(max(m.commandCursor, 0), length)
}

func (m *Model) moveCommandCursor(delta int, selecting bool) {
	m.ensureCommandCursor()
	if selecting && !m.commandSelecting {
		m.commandSelection = m.commandCursor
		m.commandSelecting = true
	}
	m.commandCursor = min(max(m.commandCursor+delta, 0), len([]rune(m.Command)))
	if !selecting || m.commandCursor == m.commandSelection {
		m.commandSelecting = false
	}
}

func (m *Model) moveCommandCursorByWord(direction int, selecting bool) {
	m.ensureCommandCursor()
	runes := []rune(m.Command)
	position := m.commandCursor
	if direction < 0 {
		for position > 0 && unicode.IsSpace(runes[position-1]) {
			position--
		}
		for position > 0 && !unicode.IsSpace(runes[position-1]) {
			position--
		}
	} else if direction > 0 {
		for position < len(runes) && !unicode.IsSpace(runes[position]) {
			position++
		}
		for position < len(runes) && unicode.IsSpace(runes[position]) {
			position++
		}
	}
	m.setCommandCursor(position, selecting)
}

func (m *Model) setCommandCursor(position int, selecting bool) {
	m.ensureCommandCursor()
	if selecting && !m.commandSelecting {
		m.commandSelection = m.commandCursor
		m.commandSelecting = true
	}
	m.commandCursor = min(max(position, 0), len([]rune(m.Command)))
	if !selecting || m.commandCursor == m.commandSelection {
		m.commandSelecting = false
	}
}

func (m *Model) moveCommandCursorToEnd() {
	m.commandCursor = len([]rune(m.Command))
	m.commandCursorValid = true
	m.commandSelecting = false
}

func (m Model) commandSelectionRange() (int, int, bool) {
	if !m.commandSelecting || m.commandCursor == m.commandSelection {
		return 0, 0, false
	}
	start, end := m.commandSelection, m.commandCursor
	if start > end {
		start, end = end, start
	}
	return start, end, true
}

func (m Model) hasCommandSelection() bool {
	_, _, ok := m.commandSelectionRange()
	return ok
}

func (m Model) selectedCommandText() string {
	start, end, ok := m.commandSelectionRange()
	if !ok {
		return ""
	}
	runes := []rune(m.Command)
	start = min(max(start, 0), len(runes))
	end = min(max(end, start), len(runes))
	return string(runes[start:end])
}

func (m *Model) deleteCommandSelection() bool {
	start, end, ok := m.commandSelectionRange()
	if !ok {
		return false
	}
	runes := []rune(m.Command)
	m.Command = string(append(runes[:start], runes[end:]...))
	m.commandCursor = start
	m.commandCursorValid = true
	m.commandSelecting = false
	return true
}

func (m *Model) insertCommandText(value string) {
	m.ensureCommandCursor()
	m.deleteCommandSelection()
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	runes := []rune(m.Command)
	inserted := []rune(value)
	m.Command = string(append(append(append([]rune(nil), runes[:m.commandCursor]...), inserted...), runes[m.commandCursor:]...))
	m.commandCursor += len(inserted)
	m.commandCursorValid = true
	m.commandSelecting = false
	m.resetCommandHistoryNavigation()
}

func (m *Model) deleteCommandBackward() {
	m.ensureCommandCursor()
	if !m.deleteCommandSelection() && m.commandCursor > 0 {
		runes := []rune(m.Command)
		m.Command = string(append(runes[:m.commandCursor-1], runes[m.commandCursor:]...))
		m.commandCursor--
	}
	m.resetCommandHistoryNavigation()
}

func (m *Model) deleteCommandForward() {
	m.ensureCommandCursor()
	if !m.deleteCommandSelection() {
		runes := []rune(m.Command)
		if m.commandCursor < len(runes) {
			m.Command = string(append(runes[:m.commandCursor], runes[m.commandCursor+1:]...))
		}
	}
	m.resetCommandHistoryNavigation()
}

func (m *Model) deleteCommandWordBackward() {
	m.ensureCommandCursor()
	if m.deleteCommandSelection() {
		m.resetCommandHistoryNavigation()
		return
	}
	runes := []rune(m.Command)
	start := m.commandCursor
	for start > 0 && unicode.IsSpace(runes[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	for start > 0 && unicode.IsSpace(runes[start-1]) {
		start--
	}
	m.Command = string(append(runes[:start], runes[m.commandCursor:]...))
	m.commandCursor = start
	m.resetCommandHistoryNavigation()
}

func (m Model) renderCommandInputValue(width int) string {
	runes := []rune(m.Command)
	cursor := m.commandCursor
	if !m.commandCursorValid {
		cursor = len(runes)
	}
	cursor = min(max(cursor, 0), len(runes))
	viewportStart, viewportEnd := commandInputViewport(runes, cursor, width)
	runes = runes[viewportStart:viewportEnd]
	cursor -= viewportStart
	selectionStart, selectionEnd, selected := m.commandSelectionRange()
	selectionStart -= viewportStart
	selectionEnd -= viewportStart
	var value strings.Builder
	for i, r := range runes {
		style := commandInputStyle
		if selected && i >= selectionStart && i < selectionEnd {
			style = commandSelectionStyle
		}
		isSelected := selected && i >= selectionStart && i < selectionEnd
		if i == cursor && !isSelected {
			style = style.Reverse(true)
		}
		value.WriteString(style.Render(string(r)))
	}
	if cursor == len(runes) {
		value.WriteString(commandInputStyle.Reverse(true).Render(" "))
	}
	return value.String()
}

func commandInputViewport(runes []rune, cursor int, width int) (int, int) {
	if width <= 0 {
		return cursor, cursor
	}
	remaining := width
	if cursor == len(runes) {
		remaining--
	}
	start, end := cursor, cursor
	rightBudget := remaining / 2
	for end < len(runes) {
		charWidth := ansi.StringWidth(string(runes[end]))
		if charWidth > rightBudget {
			break
		}
		rightBudget -= charWidth
		remaining -= charWidth
		end++
	}
	for start > 0 {
		charWidth := ansi.StringWidth(string(runes[start-1]))
		if charWidth > remaining {
			break
		}
		remaining -= charWidth
		start--
	}
	for end < len(runes) {
		charWidth := ansi.StringWidth(string(runes[end]))
		if charWidth > remaining {
			break
		}
		remaining -= charWidth
		end++
	}
	return start, end
}
