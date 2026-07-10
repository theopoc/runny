package tui

import (
	"context"
	"os"
	"os/signal"
	"sync"
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
	var runWait sync.WaitGroup
	opts.runWait = &runWait
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
	runWait.Wait()
	return err
}
