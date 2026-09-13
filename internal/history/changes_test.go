package history

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/theopoc/runny/internal/core"
)

func TestChangeSummaryLargeRunAndLegacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	if err := os.WriteFile(path, []byte("{\"command\":\"old\",\"targets\":[{\"id\":\"old\",\"status\":\"succeeded\"}]}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := RunEntry{Command: "terragrunt plan", Total: 600}
	summary := core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: 1}
	for i := 0; i < 600; i++ {
		entry.Targets = append(entry.Targets, TargetEntry{ID: fmt.Sprint(i), RelPath: fmt.Sprintf("production/region-%d/database", i), Status: core.StatusSucceeded, Changes: summary})
	}
	if err := AppendRun(path, entry); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRuns(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Targets[0].Changes.Known || len(got[1].Targets) != 600 || got[1].Targets[599].Changes != summary {
		t.Fatal("history lost legacy or large results")
	}
}
