# Archive failures do not change Target outcomes

A Target outcome records command execution or accepted cancellation. Failure to persist Command history, Target diagnostics, or a completed Run is reported as a Run archive failure while execution outcomes remain unchanged; conflating archival success with command success would make rerun and fail-fast semantics misrepresent what happened.
