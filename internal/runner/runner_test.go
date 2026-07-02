package runner

import (
	"context"
	"testing"

	"github.com/saewyn/runny/internal/core"
)

func TestRunnerRunsCommandInTargetDirectory(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command: "pwd",
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Status != core.StatusSucceeded {
		t.Fatalf("results = %#v", results)
	}
}

func TestRunnerMarksFailure(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(context.Background(), core.RunRequest{
		Command: "exit 7",
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusFailed || results[0].ExitCode != 7 {
		t.Fatalf("result = %#v", results[0])
	}
}

func TestRunnerCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir(), Selected: true}
	results, err := Run(ctx, core.RunRequest{
		Command: "sleep 1",
		Targets: []core.Target{target},
		Mode:    core.ModeSerial,
	})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != core.StatusCancelled {
		t.Fatalf("result = %#v", results[0])
	}
}
