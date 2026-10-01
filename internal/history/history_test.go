package history

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/theopoc/runny/internal/core"
)

func TestCommandHistoryRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	for i := 0; i < 55; i++ {
		if err := AppendCommand(path, CommandEntry{Command: "cmd-" + strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadCommands(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 50 {
		t.Fatalf("entries = %d, want 50", len(entries))
	}
	if entries[0].Command != "cmd-5" || entries[len(entries)-1].Command != "cmd-54" {
		t.Fatalf("retained commands = %q ... %q, want cmd-5 ... cmd-54", entries[0].Command, entries[len(entries)-1].Command)
	}
}

func TestCommandHistoryKeepsOnlyMostRecentIdenticalCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	first := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)
	latest := first.Add(time.Minute)
	for _, entry := range []CommandEntry{
		{Command: "go test ./...", Time: first},
		{Command: "pnpm test", Time: first.Add(30 * time.Second)},
		{Command: "go test ./...", Time: latest},
	} {
		if err := AppendCommand(path, entry); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ReadCommands(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %#v, want two unique commands", entries)
	}
	if entries[0].Command != "pnpm test" || entries[1].Command != "go test ./..." || !entries[1].Time.Equal(latest) {
		t.Fatalf("entries = %#v, want latest duplicate moved to the end", entries)
	}
}

func TestCommandHistoryKeepsNewestAcceptedDuplicateWhenWritesArriveOutOfOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	older := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)
	newer := older.Add(time.Minute)
	for _, entry := range []CommandEntry{
		{Command: "go test ./...", Time: newer},
		{Command: "go test ./...", Time: older},
	} {
		if err := AppendCommand(path, entry); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ReadCommands(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Time.Equal(newer) {
		t.Fatalf("entries = %#v, want newest accepted duplicate", entries)
	}
}

func TestReadCommandsDeduplicatesLegacyHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	legacy := []byte("{\"command\":\"go test ./...\",\"time\":\"2026-09-01T08:00:00Z\"}\n" +
		"{\"command\":\"pnpm test\",\"time\":\"2026-09-01T08:00:30Z\"}\n" +
		"{\"command\":\"go test ./...\",\"time\":\"2026-09-01T08:01:00Z\"}\n")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadCommands(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Command != "pnpm test" || entries[1].Command != "go test ./..." {
		t.Fatalf("legacy entries = %#v, want unique commands ordered by their latest occurrence", entries)
	}
}

func TestRunHistoryRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	for i := 0; i < 105; i++ {
		if err := AppendRun(path, RunEntry{Command: "cmd", Total: 1}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := ReadRuns(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 100 {
		t.Fatalf("entries = %d, want 100", len(entries))
	}
}

func TestRunHistoryRoundTripsDiagnosticMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	started := time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC)
	ended := started.Add(3 * time.Second)
	want := RunEntry{
		Command:   "go test ./...",
		Total:     2,
		Succeeded: 1,
		Failed:    1,
		Time:      ended,
		Started:   started,
		Ended:     ended,
		LogID:     "20260815T080000.000000000Z",
		Targets: []TargetEntry{
			{ID: "api", RelPath: "api", Status: core.StatusSucceeded, ExitCode: 0, Started: started, Ended: started.Add(time.Second)},
			{ID: "web", RelPath: "services/web", Status: core.StatusFailed, ExitCode: 1, Error: "exit status 1", Started: started, Ended: ended},
		},
	}

	if err := AppendRun(path, want); err != nil {
		t.Fatal(err)
	}
	entries, err := ReadRuns(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	got := entries[0]
	if got.Command != want.Command || got.Started != want.Started || got.Ended != want.Ended || got.LogID != want.LogID {
		t.Fatalf("run metadata = %#v, want %#v", got, want)
	}
	if len(got.Targets) != 2 || got.Targets[1].RelPath != "services/web" || got.Targets[1].Status != core.StatusFailed || got.Targets[1].Error != "exit status 1" {
		t.Fatalf("targets = %#v", got.Targets)
	}
}

func TestRunHistoryReadsLegacySummary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	legacy := []byte("{\"command\":\"go test\",\"total\":2,\"succeeded\":1,\"failed\":1,\"cancelled\":0,\"time\":\"2026-08-15T08:00:00Z\"}\n")
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadRuns(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Command != "go test" || len(entries[0].Targets) != 0 {
		t.Fatalf("legacy entries = %#v", entries)
	}
}
