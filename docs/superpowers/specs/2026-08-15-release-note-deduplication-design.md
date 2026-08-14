# Release Note Deduplication Design

## Goal

Remove duplicate historical changelog entries from releases `v0.1.0`,
`v0.1.1`, and `v0.2.0`, keep the corresponding GitHub Release bodies in
sync, and add a regression test that rejects exact duplicate release-note
labels within one release section.

The published tags, tag targets, assets, checksums, dates, headings, section
ordering, and unique release notes must remain unchanged.

## Context

Release Please 17.6.0 parsed both sides of GitHub merge commits: the original
Conventional Commit from the pull-request branch and the Conventional Commit
title embedded in the merge commit body. This produced two changelog entries
for one logical change.

The repository now allows squash merges only, uses the pull-request title as
the squash commit title, and leaves the squash commit body blank. Release
`v0.3.0` validates the new behavior: pull requests 45 and 46 each produced one
commit on `main` and one release-note entry.

## Scope

### Repository changes

- Deduplicate all release sections from `v0.1.0` through `v0.2.0` in
  `CHANGELOG.md`.
- Leave the already-clean `v0.3.0` section unchanged.
- Add an artifact-level Go regression test under `internal/releasenotes`.
- Document the squash-only release-note invariant in the maintainer runbook.

### GitHub changes

- Update only the Markdown bodies of GitHub Releases `v0.1.0`, `v0.1.1`, and
  `v0.2.0` to match their cleaned `CHANGELOG.md` sections.
- Do not update `v0.3.0`.
- Do not move or recreate tags and do not add, delete, or replace assets.

### Excluded changes

- No Release Please or GoReleaser configuration changes.
- No history rewrite.
- No ruleset changes.
- No semantic or fuzzy duplicate detection.

## Deduplication Rule

For every historical pair representing the same pull request, retain the
two-parent merge commit entry and remove the original branch commit entry.
This preserves the SHA representing integration onto `main`.

The cleanup covers both identical labels and the known `v0.2.0` paraphrase:
`remove filter input slash` versus `remove slash from filter input`. The merge
commit label and SHA win in both cases.

Every unrelated entry remains byte-for-byte unchanged. Empty lines left by
removed bullets may be collapsed only enough to preserve existing Markdown
section formatting.

## Regression-Test Seam

The public seam is generated `CHANGELOG.md`, because this is the artifact
reviewed in Release Please pull requests and copied into GitHub Release bodies.

A Go test reads `../../CHANGELOG.md`, splits content at level-two release
headings, and inspects Markdown bullet entries within each release independently.
For each bullet, it removes every trailing Markdown reference group used as
issue, pull-request, or commit metadata and compares the remaining rendered
label. Repeated labels within one release fail the test and report the release
plus duplicate label.

The test deliberately does not use Git history, network access, or semantic
similarity. This keeps `go test ./...` deterministic with the current shallow
GitHub Actions checkout. Squash-only merging prevents the structural source;
the test acts as a cheap artifact guard for exact regressions.

## Red-Green Flow

1. Add the changelog artifact test against current `origin/main` content.
2. Run the focused test and observe failures for existing duplicate sections.
3. Remove duplicate bullets while retaining merge commit entries.
4. Run the focused test again and observe success.
5. Run the full repository verification chain.

## GitHub Release Synchronization

Before editing releases, capture each target tag SHA and complete asset
inventory containing asset IDs, names, sizes, and digests. Build each new body
from the corresponding cleaned `CHANGELOG.md` section rather than maintaining
a second handwritten copy.

After editing each body, fetch it again and compare it with the local section.
Then fetch tag targets and asset inventories again. Any tag or asset difference
is a failure and stops further publication changes. Since release-body edits do
not modify tags or assets, detected drift requires restoring the previous body
and investigating before continuing.

## Documentation

The maintainer runbook will state:

- feature and fix pull requests require Conventional Commit titles;
- squash merge is the only supported merge method;
- one pull request should produce one commit on `main` and one release note;
- generated release pull requests must be reviewed for duplicate entries before
  merge.

## Verification

- Focused regression test fails before cleanup and passes after cleanup.
- `rtk go test ./...` passes.
- `rtk go test -race ./...` passes.
- `rtk git diff --check` passes.
- Every cleaned changelog section contains only intended merge SHAs.
- `v0.3.0` remains unchanged.
- GitHub Release bodies `v0.1.0` through `v0.2.0` match local sections.
- Tag targets and asset inventories match their pre-edit snapshots.
- Worktree contains only scoped repository changes.

## Delivery

Keep changes local until review. Do not commit, push, or open a pull request
without explicit user authorization.
