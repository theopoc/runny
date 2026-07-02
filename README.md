# runny

`runny` runs one shell command across selected child directories from a terminal UI.

## Install

```bash
brew install --cask saewyn/tap/runny
```

If your Homebrew setup requires trusted taps:

```bash
brew tap saewyn/tap
brew trust --tap saewyn/tap
brew install --cask runny
```

Brewfile:

```ruby
tap "saewyn/tap", trusted: true
cask "saewyn/tap/runny", trusted: true
```

## Usage

```bash
runny
runny -- pnpm test
runny --auto -- pnpm test
```

By default, `runny` discovers direct child directories, excludes hidden directories, skips symlinked directories, and runs in parallel.

## Flags

| Flag | Short | Description |
| --- | --- | --- |
| `--auto` | `-a` | Run without TUI |
| `--config FILE` | `-c` | Explicit config file |
| `--recursive` | `-r` | Discover recursively |
| `--depth N` | `-d` | Discovery depth, `1` direct children, `0` unlimited |
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

## Config

Config files load in this order:

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

## TUI Shortcuts

| Key | Action |
| --- | --- |
| `space` | Select/deselect focused directory |
| `a` | Select all visible directories |
| `A` | Deselect all visible directories |
| `/` | Focus filter |
| `enter` | Run or confirm |
| `tab` | Change focus |
| `right`, `l` | Unfold focused directory |
| `left` | Fold focused directory |
| `H` | Show command/run history |
| `?` | Show shortcuts |
| `del` | Cancel selected active runs, or focused active run |
| `R` | Rerun all failed with confirmation |
| `ctrl+c` | Cancel active runs and quit cleanly |

## Development

```bash
go test ./...
go build ./cmd/runny
```
