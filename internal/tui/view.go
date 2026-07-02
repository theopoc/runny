package tui

import tea "charm.land/bubbletea/v2"

func Run(opts Options) error {
	_, err := tea.NewProgram(NewModel(opts)).Run()
	return err
}
