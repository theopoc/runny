package tui

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
)

func Run(opts Options) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runProgram(ctx, opts)
}

func runProgram(ctx context.Context, opts Options) error {
	runTracker := newRunTracker()
	opts.runTracker = runTracker
	programOptions := []tea.ProgramOption{
		tea.WithColorProfile(colorprofile.TrueColor),
		tea.WithoutSignals(),
	}
	programOptions = append(programOptions, opts.programOptions...)
	program := tea.NewProgram(NewModel(opts), programOptions...)
	programDone := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			program.Send(shutdownMsg{})
		case <-programDone:
		}
	}()

	_, err := program.Run()
	close(programDone)
	<-shutdownDone
	runTracker.CloseAndWait()
	return err
}
