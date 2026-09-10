package tui

import (
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
)

type lineEditor struct {
	value       *string
	cursor      *int
	cursorValid *bool
	selection   *int
	selecting   *bool
}

func newLineEditor(value *string, cursor *int, cursorValid *bool, selection *int, selecting *bool) lineEditor {
	return lineEditor{
		value:       value,
		cursor:      cursor,
		cursorValid: cursorValid,
		selection:   selection,
		selecting:   selecting,
	}
}

func splitGraphemes(value string) []string {
	graphemes := uniseg.NewGraphemes(value)
	result := make([]string, 0, len([]rune(value)))
	for graphemes.Next() {
		result = append(result, graphemes.Str())
	}
	return result
}

func (e lineEditor) ensure() {
	length := len(splitGraphemes(*e.value))
	if !*e.cursorValid {
		*e.cursor = length
		*e.cursorValid = true
	}
	*e.cursor = min(max(*e.cursor, 0), length)
	*e.selection = min(max(*e.selection, 0), length)
}

func (e lineEditor) cursorPosition() int {
	e.ensure()
	return *e.cursor
}

func (e lineEditor) move(delta int, selecting bool) {
	e.ensure()
	e.prepareSelection(selecting)
	*e.cursor = min(max(*e.cursor+delta, 0), len(splitGraphemes(*e.value)))
	e.finishSelection(selecting)
}

func (e lineEditor) moveByWord(direction int, selecting bool) {
	e.ensure()
	graphemes := splitGraphemes(*e.value)
	position := *e.cursor
	if direction < 0 {
		for position > 0 && graphemeIsSpace(graphemes[position-1]) {
			position--
		}
		for position > 0 && !graphemeIsSpace(graphemes[position-1]) {
			position--
		}
	} else if direction > 0 {
		for position < len(graphemes) && !graphemeIsSpace(graphemes[position]) {
			position++
		}
		for position < len(graphemes) && graphemeIsSpace(graphemes[position]) {
			position++
		}
	}
	e.setCursor(position, selecting)
}

func graphemeIsSpace(value string) bool {
	for _, r := range value {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return value != ""
}

func (e lineEditor) setCursor(position int, selecting bool) {
	e.ensure()
	e.prepareSelection(selecting)
	*e.cursor = min(max(position, 0), len(splitGraphemes(*e.value)))
	e.finishSelection(selecting)
}

func (e lineEditor) moveToEnd() {
	*e.cursor = len(splitGraphemes(*e.value))
	*e.cursorValid = true
	*e.selecting = false
}

func (e lineEditor) selectionRange() (int, int, bool) {
	e.ensure()
	if !*e.selecting || *e.cursor == *e.selection {
		return 0, 0, false
	}
	start, end := *e.selection, *e.cursor
	if start > end {
		start, end = end, start
	}
	return start, end, true
}

func (e lineEditor) hasSelection() bool {
	_, _, ok := e.selectionRange()
	return ok
}

func (e lineEditor) selectedText() string {
	start, end, ok := e.selectionRange()
	if !ok {
		return ""
	}
	graphemes := splitGraphemes(*e.value)
	return strings.Join(graphemes[start:end], "")
}

func (e lineEditor) deleteSelection() bool {
	start, end, ok := e.selectionRange()
	if !ok {
		return false
	}
	graphemes := splitGraphemes(*e.value)
	*e.value = strings.Join(append(graphemes[:start], graphemes[end:]...), "")
	*e.cursor = start
	*e.cursorValid = true
	*e.selecting = false
	return true
}

func (e lineEditor) insert(value string) {
	e.ensure()
	e.deleteSelection()
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	graphemes := splitGraphemes(*e.value)
	inserted := splitGraphemes(value)
	result := make([]string, 0, len(graphemes)+len(inserted))
	result = append(result, graphemes[:*e.cursor]...)
	result = append(result, inserted...)
	result = append(result, graphemes[*e.cursor:]...)
	*e.value = strings.Join(result, "")
	*e.cursor += len(inserted)
	*e.cursorValid = true
	*e.selecting = false
}

func (e lineEditor) deleteBackward() {
	e.ensure()
	if e.deleteSelection() || *e.cursor == 0 {
		return
	}
	graphemes := splitGraphemes(*e.value)
	*e.value = strings.Join(append(graphemes[:*e.cursor-1], graphemes[*e.cursor:]...), "")
	*e.cursor--
}

func (e lineEditor) deleteForward() {
	e.ensure()
	if e.deleteSelection() {
		return
	}
	graphemes := splitGraphemes(*e.value)
	if *e.cursor < len(graphemes) {
		*e.value = strings.Join(append(graphemes[:*e.cursor], graphemes[*e.cursor+1:]...), "")
	}
}

func (e lineEditor) deleteWordBackward() {
	e.ensure()
	if e.deleteSelection() {
		return
	}
	graphemes := splitGraphemes(*e.value)
	start := *e.cursor
	for start > 0 && graphemeIsSpace(graphemes[start-1]) {
		start--
	}
	for start > 0 && !graphemeIsSpace(graphemes[start-1]) {
		start--
	}
	for start > 0 && graphemeIsSpace(graphemes[start-1]) {
		start--
	}
	*e.value = strings.Join(append(graphemes[:start], graphemes[*e.cursor:]...), "")
	*e.cursor = start
}

func (e lineEditor) clear() {
	*e.value = ""
	*e.cursor = 0
	*e.cursorValid = true
	*e.selecting = false
}

func (e lineEditor) prepareSelection(selecting bool) {
	if selecting && !*e.selecting {
		*e.selection = *e.cursor
		*e.selecting = true
	}
}

func (e lineEditor) finishSelection(selecting bool) {
	if !selecting || *e.cursor == *e.selection {
		*e.selecting = false
	}
}
