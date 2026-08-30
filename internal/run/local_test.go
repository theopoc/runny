package run

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
)

func TestLocalRuntimePreservesLegacyHistoryAndLogLayout(t *testing.T) {
	root := t.TempDir()
	commandHistory := filepath.Join(root, "command-history.jsonl")
	runHistory := filepath.Join(root, "run-history.jsonl")
	logRoot := filepath.Join(root, "runs")
	runtime := NewLocal(LocalOptions{
		CommandHistoryPath: commandHistory,
		RunHistoryPath:     runHistory,
		LogRoot:            logRoot,
	})
	t.Cleanup(runtime.CloseAndWait)
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir()}
	r, err := runtime.Start(context.Background(), Spec{
		Command:  "printf 'hello from api\\n'",
		Targets:  []core.Target{target},
		Mode:     core.ModeSerial,
		SaveLogs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	events := collectEvents(t, r)
	completed := events[len(events)-1]
	if completed.Kind != EventCompleted || completed.Run.Succeeded != 1 {
		t.Fatalf("completed = %#v", completed)
	}

	commands, err := history.ReadCommands(commandHistory)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 1 || commands[0].Command != "printf 'hello from api\\n'" {
		t.Fatalf("commands = %#v", commands)
	}
	runs, err := history.ReadRuns(runHistory)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].LogID != string(completed.Run.ID) || runs[0].Succeeded != 1 {
		t.Fatalf("runs = %#v", runs)
	}
	if len(runs[0].Targets) != 1 || runs[0].Targets[0].ID != "api" {
		t.Fatalf("targets = %#v", runs[0].Targets)
	}
	data, err := os.ReadFile(filepath.Join(logRoot, string(completed.Run.ID), "api.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello from api\n" {
		t.Fatalf("log = %q", data)
	}
}

func TestLocalRuntimeSerializesConcurrentHistoryWrites(t *testing.T) {
	root := t.TempDir()
	commandHistory := filepath.Join(root, "commands.jsonl")
	runHistory := filepath.Join(root, "runs.jsonl")
	runtime := NewLocal(LocalOptions{
		CommandHistoryPath: commandHistory,
		RunHistoryPath:     runHistory,
	})
	t.Cleanup(runtime.CloseAndWait)

	runs := make([]*Run, 0, 2)
	for _, id := range []string{"api", "web"} {
		r, err := runtime.Start(context.Background(), Spec{
			Command: "printf ok",
			Targets: []core.Target{{ID: id, AbsPath: t.TempDir()}},
		})
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, r)
	}
	var wg sync.WaitGroup
	ids := make(chan ID, len(runs))
	for _, r := range runs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				event, err := r.Next(context.Background())
				if errors.Is(err, io.EOF) {
					return
				}
				if err != nil {
					t.Errorf("Next() error = %v", err)
					return
				}
				if event.Kind == EventCompleted {
					ids <- event.Run.ID
				}
			}
		}()
	}
	wg.Wait()
	close(ids)
	seenIDs := map[ID]bool{}
	for id := range ids {
		seenIDs[id] = true
	}
	if len(seenIDs) != 2 {
		t.Fatalf("run ids = %v", seenIDs)
	}
	commands, err := history.ReadCommands(commandHistory)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := history.ReadRuns(runHistory)
	if err != nil {
		t.Fatal(err)
	}
	if len(commands) != 2 || len(entries) != 2 {
		t.Fatalf("history sizes = commands:%d runs:%d", len(commands), len(entries))
	}
}

func TestLocalArchiveFailureLeavesSuccessfulOutcome(t *testing.T) {
	rootFile := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(rootFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	deps := testDependencies(func(context.Context, executionRequest) executionOutcome {
		return executionOutcome{Status: core.StatusSucceeded}
	})
	deps.archive = localArchive(LocalOptions{LogRoot: rootFile})
	r := startTestRun(t, Spec{
		Command:  "test",
		Targets:  testTargets("api"),
		SaveLogs: true,
	}, deps)
	events := collectEvents(t, r)
	if events[len(events)-1].Run.Succeeded != 1 {
		t.Fatalf("completed = %#v", events[len(events)-1])
	}
	foundArchiveFailure := false
	for _, event := range events {
		foundArchiveFailure = foundArchiveFailure || event.Kind == EventArchiveFailed
	}
	if !foundArchiveFailure {
		t.Fatalf("events = %v", eventKinds(events))
	}
}
