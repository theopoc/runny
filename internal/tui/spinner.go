package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const spinnerInterval = 100 * time.Millisecond

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinnerTickMsg struct{}

func spinnerTick() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg {
		return spinnerTickMsg{}
	})
}

func spinnerFrame(frame int) string {
	return spinnerFrames[frame%len(spinnerFrames)]
}
