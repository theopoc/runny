package tui

const filterHistoryLimit = 50

func (m *Model) openFilterEditor() {
	m.Focus = FocusFilter
	m.filterLineEditor().moveToEnd()
	m.resetFilterHistoryNavigation()
}

func (m Model) hasFilterSelection() bool {
	return m.filterLineEditor().hasSelection()
}

func (m Model) selectedFilterText() string {
	return m.filterLineEditor().selectedText()
}

func (m *Model) moveFilterCursor(delta int, selecting bool) {
	m.filterLineEditor().move(delta, selecting)
}

func (m *Model) moveFilterCursorByWord(direction int, selecting bool) {
	m.filterLineEditor().moveByWord(direction, selecting)
}

func (m *Model) setFilterCursor(position int, selecting bool) {
	m.filterLineEditor().setCursor(position, selecting)
}

func (m *Model) insertFilterText(value string) {
	m.filterLineEditor().insert(value)
	m.afterFilterEdit()
}

func (m *Model) deleteFilterBackward() {
	m.filterLineEditor().deleteBackward()
	m.afterFilterEdit()
}

func (m *Model) deleteFilterForward() {
	m.filterLineEditor().deleteForward()
	m.afterFilterEdit()
}

func (m *Model) deleteFilterWordBackward() {
	m.filterLineEditor().deleteWordBackward()
	m.afterFilterEdit()
}

func (m *Model) deleteFilterSelection() bool {
	deleted := m.filterLineEditor().deleteSelection()
	if deleted {
		m.afterFilterEdit()
	}
	return deleted
}

func (m *Model) clearFilterInput() {
	m.filterLineEditor().clear()
	m.resetFilterHistoryNavigation()
	m.ensureCursorVisible()
	m.Notice = "filter cleared"
	m.RunError = ""
}

func (m *Model) afterFilterEdit() {
	m.resetFilterHistoryNavigation()
	m.syncTargetFilterError()
	m.ensureCursorVisible()
}

func (m *Model) commitFilterHistory() {
	filter := parseTargetFilter(m.Filter)
	if filter.err != nil || !filter.active() {
		m.resetFilterHistoryNavigation()
		return
	}

	updated := make([]string, 0, min(filterHistoryLimit, len(m.filterHistory)+1))
	updated = append(updated, m.Filter)
	for _, value := range m.filterHistory {
		if value != m.Filter {
			updated = append(updated, value)
		}
		if len(updated) == filterHistoryLimit {
			break
		}
	}
	m.filterHistory = updated
	m.resetFilterHistoryNavigation()
}

func (m *Model) previousFilterHistory() {
	if len(m.filterHistory) == 0 {
		return
	}
	if m.filterHistoryPos < 0 {
		m.filterDraft = m.Filter
		m.filterHistoryPos = 0
	} else if m.filterHistoryPos < len(m.filterHistory)-1 {
		m.filterHistoryPos++
	}
	m.Filter = m.filterHistory[m.filterHistoryPos]
	m.filterLineEditor().moveToEnd()
	m.syncTargetFilterError()
	m.ensureCursorVisible()
}

func (m *Model) nextFilterHistory() {
	if m.filterHistoryPos < 0 {
		return
	}
	if m.filterHistoryPos == 0 {
		m.filterHistoryPos = -1
		m.Filter = m.filterDraft
		m.filterDraft = ""
		m.filterLineEditor().moveToEnd()
		m.syncTargetFilterError()
		m.ensureCursorVisible()
		return
	}
	m.filterHistoryPos--
	m.Filter = m.filterHistory[m.filterHistoryPos]
	m.filterLineEditor().moveToEnd()
	m.syncTargetFilterError()
	m.ensureCursorVisible()
}

func (m *Model) resetFilterHistoryNavigation() {
	m.filterHistoryPos = -1
	m.filterDraft = ""
}
