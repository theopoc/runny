package history

import (
	"path/filepath"
	"testing"
)

func TestCommandHistoryRetention(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	for i := 0; i < 55; i++ {
		if err := AppendCommand(path, CommandEntry{Command: "cmd"}); err != nil {
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
