# Live Command Output Design

## Goal

Display command output in the TUI Output panel while each command is still
running. Both stdout and stderr must appear through the same stream, preserving
their observed arrival order.

## Current Behavior

The runner connects stdout and stderr to one bounded in-memory buffer. The TUI
receives that buffer only inside `RunResult`, after `cmd.Wait()` returns. As a
result, the Output panel remains empty until the command exits.

## Design

### Runner event seam

Extend `core.RunRequest` with an optional output event sink. The runner writes
stdout and stderr through one concurrency-safe writer that performs two jobs:

1. retain the bounded tail used by the final `RunResult`;
2. emit each received output chunk immediately as a `core.Event` with type
   `EventOutput` and the target ID.

The sink is optional so existing runner callers keep working. When
`DisableLogging` is enabled, output remains discarded and no live output event
is emitted, preserving current behavior.

The existing 4 MiB final-output limit remains unchanged. Live rendering does
not introduce an unbounded second buffer: the TUI stores the same bounded tail
per target.

### Bubble Tea event flow

Each target run owns a channel carrying runner output events and its final
result. A Bubble Tea command starts the runner and waits for the first channel
item. Each received output item becomes a `runOutputMsg`; `Update` appends its
chunk to the selected target's log and returns another command waiting for the
next item. The final item becomes the existing `runDoneMsg` flow.

No subprocess work blocks `Update`. No goroutine calls `View` or mutates the
model directly.

### Rendering

The existing Output panel renders `Model.Logs[targetID]`, so no layout change is
needed. Tail mode remains enabled by default and follows newly appended output.
Manual-scroll behavior remains unchanged.

When a command finishes, final result handling must not append captured output
a second time. It adds only any final error text not already emitted by the
subprocess stream.

### Concurrency and lifecycle

- Parallel targets keep independent event channels and logs.
- Cancellation closes each subprocess through the existing target context.
- A target completion event always terminates its stream-wait loop.
- Program shutdown still waits for tracked runner goroutines.
- Fail-fast behavior remains based on final target status.

## Error Handling

Command start, exit, cancellation, and log-persistence errors retain existing
status and `RunResult` behavior. Subprocess stderr is command output, not a
runner error, and is streamed exactly like stdout.

## Test Seams

Tests cover behavior through two agreed seams:

1. `runner.Run`: proves the first chunk is delivered before command completion,
   proves stdout and stderr are both emitted, and preserves bounded final
   output.
2. Bubble Tea `Model.Update`: proves a running target accepts output messages,
   updates its Output panel immediately, follows tail mode, and avoids duplicate
   output when the final result arrives.

One program-level test exercises the asynchronous channel flow from a running
target to rendered TUI state. Existing runner, model, golden, cancellation, and
shutdown tests remain green.

## Out of Scope

- Separate stdout and stderr styling or labels.
- Persisting unlimited live output in memory.
- Changing `--disable-logging` semantics.
- Changing panel layout, shortcuts, or command scheduling.
