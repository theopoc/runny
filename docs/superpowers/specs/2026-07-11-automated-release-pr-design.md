# Automated Release PR Design

## Goal

Automate release preparation after releasable changes reach `main` while keeping final publication behind an explicit maintainer-approved release PR. The first release will be `v0.1.0`.

## Release Policy

- `feat` commits trigger a minor version bump.
- `fix` and `perf` commits trigger a patch version bump.
- commits marked with `!` or a `BREAKING CHANGE` footer trigger a major version bump.
- `docs`, `chore`, `test`, and `ci` commits can appear in release history but do not trigger a release by themselves.
- A maintainer must merge the generated release PR before any tag, GitHub Release, binary, checksum, or Homebrew cask is published.

## Architecture

A single GitHub Actions workflow runs on pushes to `main`. Release Please executes first and either creates or updates a release PR, or creates the tag and GitHub Release after that PR is merged. The same workflow also has an explicit manual recovery entry point for an existing release whose artifact publication failed.

GoReleaser runs in the same workflow only when Release Please reports `release_created == true`. This avoids relying on a second workflow event: GitHub does not start another workflow for a tag created with the workflow's built-in `GITHUB_TOKEN`.

The publication path is:

```text
releasable commit reaches main
  -> Release Please creates or updates release PR
  -> maintainer reviews and merges release PR
  -> Release Please creates vX.Y.Z tag and GitHub Release
  -> workflow checks out that exact tag with full history
  -> GoReleaser builds and uploads artifacts
  -> GoReleaser updates the Homebrew tap
```

## Components

### Release Please configuration

The repository will contain Release Please manifest configuration for one Go project at repository root. Bootstrap state will produce `v0.1.0` as the first release. Generated release PRs will contain version and changelog updates derived from Conventional Commits.

### GitHub Actions workflow

The release workflow will trigger on pushes to `main` and grant only permissions required to manage release PRs, tags, and releases. Release Please will expose release metadata through its action outputs.

Build and publication steps will have a `release_created` condition. They will check out Release Please's `tag_name`, fetch complete Git history, configure Go, and execute `goreleaser release --clean`.

### GoReleaser

Existing cross-platform builds, archives, checksums, version linker flags, and Homebrew cask publishing remain unchanged. GoReleaser will preserve Release Please's existing release notes using `release.mode: keep-existing`. Existing `GITHUB_TOKEN` and `TAP_GITHUB_TOKEN` environment variables remain publication credentials.

## Failure Handling

- Release Please failure stops workflow before publication.
- Normal `main` pushes with no completed release report success without running GoReleaser.
- GoReleaser failure leaves tag and GitHub Release visible, making failure diagnosable and workflow rerunnable.
- Manual recovery accepts only a validated existing `vX.Y.Z`-style tag and checks out that exact tag. It does not create or move tags.
- Recovery refuses drafts and releases with any existing assets. Partial publication therefore requires deliberate maintainer reconciliation; artifacts are never silently replaced.
- Homebrew publication failure remains visible in GoReleaser logs and must fail release job.

## Validation

Before merge:

- validate workflow syntax and Release Please configuration;
- run `goreleaser check`;
- run `goreleaser release --snapshot --clean --skip=publish`;
- run full Go test suite;
- inspect permissions, conditions, tag checkout, and secret propagation.

After merge, expected first-stage behavior is creation of a `v0.1.0` release PR, not immediate publication. Actual release occurs only after maintainer merges that PR. GitHub Actions evidence must show Release Please output, exact tag checkout, GoReleaser publication, GitHub assets, and Homebrew cask update.

## Non-goals

- automatic release after every push to `main`;
- automatic merge of release PRs;
- nightly or snapshot GitHub Releases;
- changing current artifact matrix or Homebrew installation design;
- introducing a PAT or GitHub App token solely to chain workflows.
