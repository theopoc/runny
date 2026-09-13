package run

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theopoc/runny/internal/core"
	"github.com/theopoc/runny/internal/history"
)

func TestChangeSummaryLifecycleAndArchive(t *testing.T) {
	for _, disable := range []bool{false, true} {
		t.Run(map[bool]string{true: "logging disabled", false: "logs evicted"}[disable], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "runs.jsonl")
			deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
				req.OnOutput([]byte("Plan: 3 to add, 2 to change, 1 to destroy.\n"))
				// Retention is byte-based. Newline padding crosses the real limit
				// without making this lifecycle check a parser throughput benchmark.
				filler := []byte(strings.Repeat("\n", MaxOutputBytes))
				copy(filler, "ordinary output\n")
				req.OnOutput(filler[:len(filler)/2])
				req.OnOutput(filler[len(filler)/2:])
				return executionOutcome{Status: core.StatusSucceeded}
			})
			deps.archive = localArchive(LocalOptions{RunHistoryPath: path})
			r := startTestRun(t, Spec{Command: "terraform plan", Targets: testTargets("stack"), DisableLogging: disable}, deps)
			events := collectEvents(t, r)
			for _, e := range events {
				if e.Target != nil && e.Kind != EventTargetFinished && e.Target.Changes.Known {
					t.Fatal("published counts before finish")
				}
			}
			s := events[len(events)-1].Run.Targets[0]
			want := core.ChangeSummary{Detected: true, Known: true, Phase: "plan", Add: 3, Change: 2, Destroy: 1}
			if s.Changes != want {
				t.Fatalf("summary %+v", s.Changes)
			}
			if !disable && !s.OutputTruncated {
				t.Fatal("fixture did not cross the retention limit")
			}
			if strings.Contains(s.OutputTail, "Plan:") {
				t.Fatal("fixture did not evict the plan")
			}
			if disable && s.OutputTail != "" {
				t.Fatal("logs captured despite disabled option")
			}
			entries, err := history.ReadRuns(path)
			if err != nil {
				t.Fatal(err)
			}
			if entries[0].Targets[0].Changes != want {
				t.Fatal("summary not archived independently of logs")
			}
		})
	}
}

func TestChangeSummaryFailuresAndDetailedExit(t *testing.T) {
	for _, tt := range []struct {
		name, command, output string
		status                core.Status
		code                  int
		known                 bool
		final                 core.Status
	}{
		{"success", "tofu plan", "Plan: 1 to add, 0 to change, 0 to destroy.", core.StatusSucceeded, 0, true, core.StatusSucceeded},
		{"detailed", "tofu plan -detailed-exitcode", "Plan: 1 to add, 0 to change, 0 to destroy.", core.StatusFailed, 2, true, core.StatusSucceeded},
		{"shell two", "tofu plan; exit 2", "Plan: 1 to add, 0 to change, 0 to destroy.", core.StatusFailed, 2, false, core.StatusFailed},
		{"failed", "terraform plan", "Plan: 1 to add, 0 to change, 0 to destroy.", core.StatusFailed, 1, false, core.StatusFailed},
		{"cancelled", "terraform plan", "Plan: 1 to add, 0 to change, 0 to destroy.", core.StatusCancelled, 0, false, core.StatusCancelled},
		{"terragrunt detailed", "terragrunt plan -detailed-exitcode", "12:00:00 STDOUT tofu: Plan: 1 to add, 0 to change, 0 to destroy.", core.StatusFailed, 2, true, core.StatusSucceeded},
		{"terragrunt hook two", "terragrunt plan -detailed-exitcode", "12:00:00 STDOUT tofu: Plan: 1 to add, 0 to change, 0 to destroy.\n12:00:01 ERROR after hook failed: exit status 2", core.StatusFailed, 2, false, core.StatusFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deps := testDependencies(func(_ context.Context, req executionRequest) executionOutcome {
				req.OnOutput([]byte(tt.output))
				return executionOutcome{Status: tt.status, ExitCode: tt.code, Error: map[bool]string{true: "exit status 2"}[tt.code == 2]}
			})
			r := startTestRun(t, Spec{Command: tt.command, Targets: testTargets("one", "two"), Mode: core.ModeSerial, FailFast: true}, deps)
			events := collectEvents(t, r)
			final := events[len(events)-1].Run
			got := final.Targets[0]
			if got.Changes.Known != tt.known || got.Status != tt.final || got.ExitCode != tt.code {
				t.Fatalf("%+v", got)
			}
			if tt.final == core.StatusSucceeded && (final.Succeeded != 2 || got.Error != "") {
				t.Fatal("valid code 2 triggered fail-fast or retained failure")
			}
		})
	}
}
