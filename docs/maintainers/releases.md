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
`contents: write`, `packages: write`, and `pull-requests: write`.

Create the `TAP_GITHUB_TOKEN` Actions secret. It must be able to update the
Homebrew tap repository configured in `.github/workflows/release.yml`. Use a
fine-grained token scoped to that repository with **Contents: Read and write**.
Repository or organization policy may also require SSO authorization or
approval. Never put this token in repository files or logs.

GHCR publication uses the repository `GITHUB_TOKEN`; no registry secret is
required. After the first publication, open the `runny` package settings and
confirm its visibility is **Public** so users can pull it anonymously. If GitHub
created it as private, changing it to public is irreversible.

## Normal release

1. Merge releasable Conventional Commits into `main` (`feat`, `fix`, or `perf`).
2. Wait for workflow `release` to create or update Release Please PR.
3. Review version, `CHANGELOG.md`, manifest change, and CI results.
4. Merge Release Please PR. Do not create tag manually.
5. Confirm same `release` run creates `vX.Y.Z`, publishes GitHub assets,
   replaces `Formula/runny.rb` in Homebrew tap, removes legacy
   `Casks/runny.rb`, and publishes `ghcr.io/OWNER/runny:vX.Y.Z` plus
   `ghcr.io/OWNER/runny:latest` for Linux amd64 and arm64.
6. Install `theopoc/tap/runny` in a clean Homebrew environment and confirm
   `runny --version` reports `X.Y.Z`.
7. Run `docker run --rm ghcr.io/OWNER/runny:vX.Y.Z --version` and confirm the
   binary reports `X.Y.Z`. Inspect the image version and revision annotations,
   then confirm an anonymous pull succeeds.

Release Please creates initial `v0.1.0`. Documentation, maintenance, tests, or
CI-only commits do not start a release by themselves.

## Recover failed artifact publication

Use recovery only when Release Please already created tag and GitHub Release,
but GoReleaser failed before uploading any GitHub assets.

1. Open failed workflow run and identify exact `vX.Y.Z` tag.
2. Inspect release assets, Homebrew Formula, and GHCR tags. If any asset,
   Formula update, or image tag exists, stop and reconcile partial publication
   manually. Do not replace artifacts or published images.
3. Run **Actions > release > Run workflow**, enter exact existing tag, and run.
4. Verify uploaded archives/checksum and Homebrew Formula before closing incident.

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
