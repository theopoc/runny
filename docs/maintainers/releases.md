# Release Runbook

`runny` uses one GitHub Actions workflow for Release Please and GoReleaser. A
normal push to `main` can prepare a release PR, but only a maintainer merging
that PR authorizes a release.

## Repository setup

In **Settings > Actions > General > Workflow permissions**:

- keep the default workflow permission at **Read repository contents**;
- enable **Allow GitHub Actions to create and approve pull requests**.

The second setting is exposed by the Actions permissions API as
`can_approve_pull_request_reviews: true`. Verify both settings with:

```bash
gh api repos/OWNER/runny/actions/permissions/workflow
```

The release workflow requests its narrower runtime permissions explicitly:
`contents: write` and `pull-requests: write`.

In **Settings > General > Pull Requests**, allow squash merging only. Configure
the squash commit title from the pull request title and leave the commit body
blank. Disable merge commits and rebase merging. Verify the invariant with:

```bash
gh api repos/OWNER/runny \
  --jq '{allow_merge_commit,allow_squash_merge,allow_rebase_merge,squash_merge_commit_title,squash_merge_commit_message}'
```

Feature and fix pull request titles must use Conventional Commit syntax. One
pull request must produce one commit on `main` and one release-note entry.

Create the `TAP_GITHUB_TOKEN` Actions secret. It must be able to update the
Homebrew tap repository configured in `.goreleaser.yaml`. Use a fine-grained
token scoped to that repository with **Contents: Read and write**. Repository or
organization policy may also require SSO authorization or approval. Never put
this token in repository files or logs.

## Normal release

1. Give each releasable pull request a Conventional Commit title (`feat`,
   `fix`, or `perf`) and squash-merge it into `main`.
2. Wait for workflow `release` to create or update Release Please PR.
3. Review version, `CHANGELOG.md`, manifest change, CI results, and confirm each
   logical change appears exactly once.
4. Merge Release Please PR. Do not create tag manually.
5. Confirm same `release` run creates `vX.Y.Z`, publishes GitHub assets, and
   updates Homebrew tap.

Release Please creates initial `v0.1.0`. Documentation, maintenance, tests, or
CI-only commits do not start a release by themselves.

## Recover failed artifact publication

Use recovery only when Release Please already created tag and GitHub Release,
but GoReleaser failed before uploading any GitHub assets.

1. Open failed workflow run and identify exact `vX.Y.Z` tag.
2. Inspect release assets and Homebrew tap. If any asset or tap update exists,
   stop and reconcile partial publication manually. Do not replace artifacts.
3. Run **Actions > release > Run workflow**, enter exact existing tag, and run.
4. Verify uploaded archives/checksum and Homebrew cask before closing incident.

Recovery requires exact `vX.Y.Z` input, existing tag, and non-draft
GitHub Release, checks out exact tag, and refuses releases containing assets.
It never creates or moves tags and never overwrites existing artifacts. If a
partial run uploaded assets, preserve evidence and decide explicitly whether to
delete whole failed release/assets and retry or publish corrected new version.

## Action runtime upgrades

`googleapis/release-please-action@v4` may produce GitHub's Node 20 retirement
warning. Upstream v5 moves to Node 24 and includes breaking changes. Treat v5 as
separate reviewed dependency upgrade; do not change release behavior merely to
silence warning.
