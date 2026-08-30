package tui

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	runpkg "github.com/theopoc/runny/internal/run"
)

func Run(opts Options) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runProgram(ctx, opts)
}

func runProgram(ctx context.Context, opts Options) error {
	lifecycleCtx, cancelLifecycle := context.WithCancel(ctx)
	defer cancelLifecycle()
	opts.lifecycleCtx = lifecycleCtx
	var runtime *runpkg.Runtime
	if opts.startRun == nil {
		runtime = runpkg.NewLocal(runpkg.LocalOptions{
			CommandHistoryPath: opts.CommandHistoryPath,
			RunHistoryPath:     opts.RunHistoryPath,
			LogRoot:            opts.LogRoot,
		})
		opts.startRun = func(ctx context.Context, spec runpkg.Spec) (activeRun, error) {
			return runtime.Start(ctx, spec)
		}
	}
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
	cancelLifecycle()
	close(programDone)
	<-shutdownDone
	if runtime != nil {
		runtime.CloseAndWait()
	}
	return err
}
