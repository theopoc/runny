package tui

import (
	"fmt"
	"slices"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/theopoc/runny/internal/history"
)

// Commands can finish out of order. Share a writer across model copies so an
// older snapshot can never replace a newer one, including during shutdown.
type filterHistoryWriter struct {
	mu       sync.Mutex
	path     string
	revision uint64
}

func (w *filterHistoryWriter) save(revision uint64, filters []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if revision <= w.revision {
		return nil
	}
	if err := history.SaveFilters(w.path, filters); err != nil {
		return fmt.Errorf("save filter history: %w", err)
	}
	w.revision = revision
	return nil
}

type filterHistorySavedMsg struct{ err error }

func (m Model) saveFilterHistory() tea.Cmd {
	if m.filterHistoryWriter == nil || m.filterHistoryRevision == 0 {
		return nil
	}
	writer, revision, filters := m.filterHistoryWriter, m.filterHistoryRevision, slices.Clone(m.filterHistory)
	return func() tea.Msg {
		return filterHistorySavedMsg{err: writer.save(revision, filters)}
	}
}
