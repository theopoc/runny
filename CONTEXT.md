# Runny

Runny executes one shell command across selected child directories while keeping each outcome independently observable and controllable.

## Language

**Run**:
A user-initiated attempt to execute one command across a fixed snapshot of selected Targets. It ends when every Target execution reaches a terminal outcome.
_Avoid_: Session, batch, job

**Target**:
A discovered child directory eligible for inclusion in a Run.
_Avoid_: Task, project, folder

**Target execution**:
One Target's participation in a Run, with its own lifecycle, output, and outcome.
_Avoid_: Task, worker, process

**Target outcome**:
The terminal status and diagnostics of a Target execution, determined by command execution or accepted cancellation rather than archival success.
_Avoid_: Archive status, log status

**Run lifecycle**:
The progression of a Run and its Target executions from acceptance through terminal completion, including output and cancellation.
_Avoid_: Scheduler, session lifecycle

**Run event**:
A factual change within a Run lifecycle that observers can react to without owning the Run's state.
_Avoid_: TUI message, callback

**Run archive**:
The retained record of completed Runs and their available Target diagnostics.
_Avoid_: History file, log store

**Run ID**:
The stable identity assigned when a Run is accepted, shared by its live events and archived diagnostics.
_Avoid_: Log ID, timestamp path

**Command history**:
The retained commands accepted for Runs and available for starting a later Run.
_Avoid_: Editor history, input history

**Cancellation scope**:
The fixed Target executions, or entire Run, for which cancellation has been requested.
_Avoid_: Current selection, cursor scope
