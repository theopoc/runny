<div align="center">

# Runny

Run one shell command across selected child directories from a terminal UI.

[![GitHub](https://img.shields.io/badge/GitHub-theopoc%2Frunny-181717?style=for-the-badge&logo=github)](https://github.com/theopoc/runny)
[![Releases](https://img.shields.io/badge/Releases-view-2ea44f?style=for-the-badge)](https://github.com/theopoc/runny/releases)
[![Go 1.26.x](https://img.shields.io/badge/Go-1.26.x-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/doc/)
[![MIT License](https://img.shields.io/badge/License-MIT-yellow?style=for-the-badge)](LICENSE)

</div>

## Overview

`Runny` discovers child directories, lets you select targets in a terminal UI, and runs the same command across them. It supports parallel or serial execution, filtering, cancellation, run history, and optional persisted logs.

Runny is 100% vibe coded.

![Runny TUI demo](demo/runny.gif)

## Features

- Select and filter directories before running a command.
- Run targets in parallel with a worker limit, or one at a time.
- Cancel queued or active runs and rerun failures.
- Review live output and command history in the TUI.
- Configure discovery, execution, and logging through YAML files or CLI flags.

By default, `Runny` discovers directories up to depth 3, excludes hidden directories, skips symlinked directories, and runs in parallel.

## Installation

Install with Homebrew:

```bash
brew install theopoc/tap/runny
```

Homebrew builds Runny from source, so macOS does not treat the installed binary
as an unidentified downloaded executable. Go is installed as a build dependency.

If you installed Runny as a cask before v0.6.0, migrate once:

```bash
brew uninstall --cask runny
brew install theopoc/tap/runny
```

Alternatively, install directly with Go:

```bash
go install github.com/theopoc/runny/cmd/runny@latest
```

## Quick Start

Launch the TUI, optionally with an initial command and execution settings:

```bash
runny
runny -- pnpm test
runny --depth 2 --workers 4 -- pnpm test
runny -- printf '%s\n' 'hello world'
runny -- sh -c 'pnpm test && pnpm lint'
```

`Runny` always opens the TUI. Flags and config files prepare the initial discovery, command, execution mode, and logging options.

Each task runs through the interactive shell named by `$SHELL`. When that variable is unset, Runny uses the current user's login shell from `/etc/passwd`, then falls back to `/bin/sh`. This loads shell aliases and functions. When `direnv` is available, Runny automatically allows and loads the `.envrc` found for each target directory. Because `.envrc` files execute shell code, only run Runny in trusted directory trees. Commands receive a pseudo-terminal for shell compatibility, but no interactive input.

Arguments after `--` keep their original boundaries. Shell metacharacters inside an argument are treated as data. Use `sh -c` explicitly when the command needs shell composition such as `&&`, pipes, or redirections.

## Resource change summaries

Tasks shows a **CHANGES** column when Runny recognizes Terraform, OpenTofu or
Terragrunt output. Each Target keeps its existing row:

```text
production   ✓ ok   P +3 ~2 -1
staging      ✓ ok   A +0 ~1 -0
```

`P` means plan, `A` means apply; `+` counts additions, `~` updates, and `-`
destructions. A replacement contributes to both additions and destructions.
Imports and resources forgotten from state are not added to these counters.
`+0 ~0 -0` means an explicit zero-resource result; `—` means no reliable summary.
Output-only changes may have no recognizable resource summary.

Counts appear after the entire Target command succeeds. Failed, cancelled,
queued and running Targets show no counts. Rerunning a Target clears its old
counts immediately. History retains the displayed summary with that execution,
even without saved logs or with `--disable-logging`. Runs with summaries open
their History target list showing all Targets; `a` still toggles the filter.

For `terragrunt run --all` (or legacy `run-all`), Runny totals the final results
of the participating units **inside each Target**. Unit names and counts are not
added to Tasks or History. Recognized unit identities prevent duplicate summaries
from being counted twice; plan and apply counts are never added together.
Unidentifiable units, missing summaries, mixed phases, oversized records or
reported errors make the result unavailable rather than presenting a partial total.

Recognition is passive: commands, flags and original Output are unchanged.
Supported inputs include engine text, version-1 JSON UI `change_summary` events,
and standard Terragrunt pretty/JSON log envelopes. Custom log formats, stripped
unit prefixes in concurrent output and arbitrary wrappers cannot guarantee a total.
Native regression captures cover Terraform 1.14.3, OpenTofu 1.12.6 and Terragrunt
1.1.4; older Terragrunt envelopes are covered separately by synthetic fixtures.

A literal `terraform plan -detailed-exitcode` or `tofu plan -detailed-exitcode`
can finish successfully with exit code `2` when recognizable engine output is
present. The raw code remains in History and does not trigger fail-fast.
Terragrunt additionally requires a complete framed plan result with no reported
error, including hook errors. Shell compositions and ambiguous results keep the
normal nonzero-exit handling; Runny cannot recover exit statuses hidden by scripts.

At narrow widths CHANGES takes priority over TIME, then STATUS becomes a symbol.
Large counts wrap between counters instead of truncating digits. This does not
add selectable rows or change Target actions.

## Docker

Run the published image:

```bash
docker run --rm ghcr.io/theopoc/runny:latest --version
docker run --rm -it -v "$PWD:/workspace" -w /workspace ghcr.io/theopoc/runny:latest
```

Use a versioned release tag instead of `latest` when reproducibility matters,
for example `ghcr.io/theopoc/runny:v0.5.0`.

Build an image from the current checkout:

```bash
docker build \
  --build-arg VERSION="$(git describe --tags --always --dirty --match 'v[0-9]*' | sed 's/^v//')" \
  -t runny .
```

Run the locally built image:

```bash
docker run --rm -it -v "$PWD:/workspace" -w /workspace runny
docker run --rm -it -v "$PWD:/workspace" -w /workspace runny --depth 2 -- pnpm test
```

The image includes `sh`, `bash`, `zsh`, and `direnv`. Its non-root `runny` user has `/bin/zsh` as the login shell, so no shell environment variable is required.

Docker remaps the workspace path, so a `.envrc` allowed on the host is not trusted at its container path. Runny automatically allows the file before executing a target command:

```bash
docker run --rm -it \
  -v "$PWD:/workspace" \
  -v "$HOME/.zshrc:/home/runny/.zshrc:ro" \
  -w /workspace \
  runny
```

Mounting `.zshrc` makes its aliases and functions available to commands launched by Runny. Shell startup files must only reference tools and paths available inside the container; use a container-specific `.zshrc` when the host file depends on host-only plugins. The TUI forces a truecolor render profile, so Docker runs do not need extra terminal color environment flags. Other tools installed only on the host, such as `pnpm` or project-specific CLIs, must also exist in the image or be provided by a custom image.

## Flags

| Flag | Short | Description |
| --- | --- | --- |
| `--config FILE` | `-c` | Explicit config file |
| `--recursive` | `-r` | Discover recursively |
| `--depth N` | `-d` | Discovery depth, default `3`, `1` direct children, `0` unlimited |
| `--include-hidden` | `-H` | Include hidden directories |
| `--include PATTERN` | `-i` | Include matching directories, repeatable |
| `--exclude PATTERN` | `-e` | Exclude matching directories, repeatable |
| `--serial` | `-s` | Run one target at a time |
| `--workers N` | `-w` | Max parallel target runs |
| `--fail-fast` | `-f` | Cancel queued work after first failure |
| `--save-logs` | `-L` | Persist logs under `.runny/runs/` |
| `--disable-logging` | `-N` | Disable log capture |
| `--version` | `-v` | Print version |

`--include` and `--exclude` are mutually exclusive. `--serial` and `--workers` are mutually exclusive. `--disable-logging` and `--save-logs` are mutually exclusive.

## Configuration

Configuration loads in this order, with later sources taking precedence:

1. `~/.runny.yaml`
2. `./.runny.yaml`
3. `--config FILE`
4. CLI flags

Example:

```yaml
recursive: true
depth: 2
include_hidden: false
workers: 4
fail_fast: false
save_logs: false
disable_logging: false
exclude:
  - node_modules
```

## Shortcuts

| Key | Action |
| --- | --- |
| `space` | Select/deselect focused directory |
| `a` | Select/unselect all directories; with an active filter, exclusively select matching directories or deselect all |
| `/` | Focus Target filter (`fuzzy` by default, `'` exact, `re:` regex) |
| `:` | Open command overlay |
| `o` | Open session options; `left`/`right` changes category, `up`/`down` selects, `space`/`enter` toggles, `+`/`-` adjusts workers, `a` resets workers to auto, `esc` closes |
| `ctrl+p` | Open command palette |
| `enter` | Run or confirm |
| `tab` | Change focus |
| `up`, `k` | Move cursor up |
| `down`, `j` | Move cursor down |
| `n`, `N` | Move to next/previous direct Target-filter match while Tasks is focused |
| `g` | Move to first visible directory |
| `G` | Move to last visible directory |
| `right`, `l` | Unfold focused directory |
| `left`, `h` | Fold focused directory |
| `esc` | Clear active Target filter and return to Tasks |
| `H` | Show command/run history |
| `?` | Show shortcuts |
| `del`, `x` | Cancel selected running or queued runs, or focused run; multiple selected active runs require confirmation |
| `q` | Confirm quit with Yes/No |
| `R` | Rerun all failed with confirmation |
| `pageup`, `pagedown` | Scroll output |
| Mouse wheel | Move one task when Tasks is focused; scroll three lines when Output is focused |
| `f` | Toggle output tail mode |
| `y` | With Output focused, copy its active mouse selection or all retained output for the current Target |
| Left-button drag in Output | Select text to copy; a simple click or `esc` clears the selection |
| `ctrl+c` | Confirm quit with Yes/No; `tab` switches choice, Yes cancels active runs cleanly |

Target filters match discovered relative paths. Prefix a query with `re:` to use
Go regular-expression syntax, for example `re:^services/(api|web)$`. Regex
matching is case-sensitive by default; use an inline flag such as
`re:(?i)^api` for case-insensitive matching. Invalid expressions match no
Targets, remain in the filter editor after `enter`, and show the parse error.
Regex mode applies only to the Target filter; command palette and history
search keep their existing fuzzy and exact modes.

Output copy is limited to the current Target and excludes History. Local sessions
use the platform clipboard tool when available (`pbcopy`, `clip.exe`, `wl-copy`,
`xclip`, or `xsel`). tmux and remote SSH sessions use terminal clipboard
forwarding; Runny reports the request as sent because those transports provide no
clipboard acknowledgement.

While the Target-filter editor is focused, `left`/`right`, `home`/`end`, and
word motions edit at the cursor; hold `shift` to select text. Copy, cut, and
paste use the same bindings as the command editor. `up`/`down` browse up to 50
unique filters accepted by the current Runny process, newest first; moving
back down restores the draft. `enter`, `tab`, or clicking Tasks accepts a valid,
non-empty filter into this in-memory history. Long filters remain one line and
scroll horizontally with `‹`/`›` markers.

Palette commands include `run`, `options`, `workers N|auto`, `serial`, `parallel`, `failed`, `rerun-failed`, `cancel`, `cancel-all`, `logs`, `history`, and `clear-filter`.

Session options include serial execution, worker count, fail-fast behavior, output capture, persisted logs, and output following. Execution and logging options stay locked while runs are active; view options remain mutable. Changes apply to current session and do not rewrite configuration files.

## Development

Run the checks used for local development:

```bash
go test ./...
go build ./cmd/runny
```

## Contributing

Open an [issue](https://github.com/theopoc/runny/issues) for bugs or proposed changes. Pull requests are welcome at [theopoc/runny](https://github.com/theopoc/runny/pulls).

Before submitting a pull request, run:

```bash
go test ./...
go build ./cmd/runny
```

## License

Licensed under the [MIT License](LICENSE).
