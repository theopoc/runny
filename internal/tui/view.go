package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func Run(opts Options) error {
	_, err := tea.NewProgram(NewModel(opts), tea.WithColorProfile(colorprofile.TrueColor)).Run()
	return err
}
