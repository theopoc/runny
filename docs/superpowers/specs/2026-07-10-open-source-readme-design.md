# Open-Source README Design

## Goal

Make `README.md` a concise, community-ready entry point for `runny`. New users should understand the tool, see the TUI in action, install it, run a first command, find configuration details, and know how to contribute without reading source code.

## Scope

- Rewrite `README.md` and add `demo/runny.gif`.
- Preserve documented CLI, config, logging, discovery, and TUI behavior.
- Use repository-backed metadata for every badge and link.
- Keep Homebrew and Docker installation paths.
- Add concise development, contributing, and license sections.
- Embed a short Ghostty-driven TUI demonstration near the introduction.
- Omit Brewfile and project-structure sections.

No source code, workflow, release configuration, repository visibility, or community-policy files change. Demo fixtures remain temporary and are not committed.

## Structure

1. Centered project hero with tagline and status badges.
2. Short overview focused on selecting directories and running one command across them.
3. Embedded TUI demo GIF.
4. Clear note that project is 100% vibe coded.
5. Feature summary.
6. Quick start with Homebrew install and first commands.
7. Detailed usage, configuration, Docker, and shortcut references.
8. Development commands.
9. Contribution guidance linking GitHub issues and pull requests.
10. MIT license reference.

## TUI Demo

Create a 12-15 second GIF from a real `runny` session:

- Build `runny` from the feature worktree.
- Create temporary `api`, `web`, and `worker` directories containing text files with different line counts.
- Open and resize a dedicated Ghostty terminal to 120 columns by 36 rows.
- Launch `runny -- wc -l '*'`, select all targets, and start the parallel run.
- Navigate between distinct command outputs, then open help with `?`.
- Use `ghostty-terminal-automation` for keystrokes, stable-screen waits, cell inspection, and final screenshot evidence.
- Record the terminal window, convert the recording to an optimized looping GIF, and save it as `demo/runny.gif`.

The command stays short and uses standard Unix tooling. Quoting `'*'` prevents the invoking shell from expanding the glob; `runny` expands it independently inside each selected directory.

## Badges

Use consistent `style=for-the-badge` Shields URLs. Include exactly four non-overlapping badges:

- Static GitHub repository badge linking to `theopoc/runny`.
- Static Releases badge linking to GitHub Releases without claiming release status.
- Go 1.26.x from `go.mod` and workflow configuration.
- MIT license from `LICENSE` and GitHub repository metadata.

Exclude dynamic CI, release, release-version, download, star-count, social, and package-registry badges. Current repository is private, so unauthenticated workflow badges return `REPO OR WORKFLOW NOT FOUND`; release workflow also has no runs. Static badges must never depend on GitHub API visibility.

## Content Rules

- Commands must match current CLI behavior and existing release configuration.
- Links use canonical repository identity `theopoc/runny`.
- Tone stays direct and technical; no marketing claims.
- State plainly that project is 100% vibe coded without presenting it as quality or security evidence.
- No placeholders, speculative compatibility claims, or unsupported badges.
- README stays proportional to a focused CLI/TUI project.

## Validation

- Run `go test ./...` to ensure documentation work starts and ends from a healthy tree.
- Scan README for placeholders and stale repository identities.
- Fetch every badge image and confirm none contains `NOT FOUND`, `NO STATUS`, or duplicate GitHub information.
- Inspect rendered Markdown structure through source review.
- Inspect the final GIF for readable text, correct dimensions, smooth looping, and reasonable repository size.
- Confirm the recorded sequence shows distinct `wc -l` output for all three targets.
- Confirm final diff contains intended documentation files only.
