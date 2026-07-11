# Automated Release PR Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create maintainer-approved Release Please PRs from releasable commits on `main`, then publish `runny` artifacts and Homebrew cask from the same workflow after the release PR is merged.

**Architecture:** One `release` workflow runs Release Please for every push to `main`. Ordinary pushes only create or update a release PR; merging that PR makes Release Please create a tag and GitHub Release, then conditional steps check out that exact tag and run GoReleaser against the existing release. A manual dispatch can recover a failed publication only for a validated existing release with no uploaded assets.

**Tech Stack:** GitHub Actions, `googleapis/release-please-action@v4`, Release Please manifest configuration, GoReleaser v2, Go 1.26, JSON, YAML

## Global Constraints

- First release must be `v0.1.0`.
- A maintainer must merge the generated release PR before publication.
- `feat`, `fix`, and `perf` commits are releasable; `docs`, `chore`, `test`, and `ci` alone do not trigger a release.
- `feat` increments minor, `fix` and `perf` increment patch, and `!` or `BREAKING CHANGE` increments major.
- Use one workflow; do not add a PAT or GitHub App token to chain workflows.
- Preserve current Linux and Darwin, amd64 and arm64 artifacts, checksums, linker version, and Homebrew cask publication.
- Preserve Release Please release notes when GoReleaser uploads artifacts.
- Prefix shell commands with `rtk`.

---

### Task 1: Bootstrap Release Please configuration

**Files:**
- Create: `release-please-config.json`
- Create: `.release-please-manifest.json`

**Interfaces:**
- Consumes: Conventional Commit messages on repository default branch.
- Produces: Release Please package definition for root path `.` and current released version state `0.0.0`; workflow in Task 2 consumes both files automatically.

- [ ] **Step 1: Add Release Please configuration**

Create `release-please-config.json`:

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "packages": {
    ".": {
      "release-type": "go",
      "package-name": "runny",
      "initial-version": "0.1.0",
      "include-component-in-tag": false,
      "include-v-in-tag": true,
      "changelog-path": "CHANGELOG.md"
    }
  }
}
```

Create `.release-please-manifest.json`:

```json
{
  ".": "0.0.0"
}
```

- [ ] **Step 2: Validate both JSON documents**

Run:

```bash
rtk jq empty release-please-config.json .release-please-manifest.json
rtk jq -e '.packages["."]."release-type" == "go" and .packages["."]."initial-version" == "0.1.0" and .packages["."]."include-component-in-tag" == false and .packages["."]."include-v-in-tag" == true' release-please-config.json
rtk jq -e '.["."] == "0.0.0"' .release-please-manifest.json
```

Expected: every command exits `0`; final two commands print `true`.

- [ ] **Step 3: Inspect scoped diff**

Run:

```bash
rtk git diff --check
rtk git status --short
```

Expected: no whitespace errors; only both Release Please JSON files are new, plus already committed plan if plan commit has not happened yet.

- [ ] **Step 4: Commit configuration**

Use `commit` skill. Stage only configuration files:

```bash
rtk git add release-please-config.json .release-please-manifest.json
```

Commit message:

```text
ci: configure release please
```

### Task 2: Integrate Release Please and GoReleaser in one workflow

**Files:**
- Modify: `.github/workflows/release.yml`
- Modify: `.goreleaser.yaml`

**Interfaces:**
- Consumes: `release-please-config.json`, `.release-please-manifest.json`, `secrets.GITHUB_TOKEN`, and `secrets.TAP_GITHUB_TOKEN`.
- Produces: Release Please outputs `release_created` and `tag_name`; conditional GoReleaser publication uploads artifacts to that existing tag and release while preserving release notes.

- [ ] **Step 1: Replace release workflow**

Replace `.github/workflows/release.yml` with:

```yaml
name: release

on:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Create or update release PR
        id: release
        uses: googleapis/release-please-action@v4

      - name: Check out release tag
        if: ${{ steps.release.outputs.release_created == 'true' }}
        uses: actions/checkout@v5
        with:
          ref: ${{ steps.release.outputs.tag_name }}
          fetch-depth: 0

      - name: Set up Go
        if: ${{ steps.release.outputs.release_created == 'true' }}
        uses: actions/setup-go@v6
        with:
          go-version: "1.26.x"
          cache: true

      - name: Publish release artifacts
        if: ${{ steps.release.outputs.release_created == 'true' }}
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          TAP_GITHUB_TOKEN: ${{ secrets.TAP_GITHUB_TOKEN }}
```

- [ ] **Step 2: Make release-note preservation explicit**

Add this top-level section to `.goreleaser.yaml` after `changelog`:

```yaml
release:
  mode: keep-existing
```

Do not modify builds, archives, checksum, changelog, or `homebrew_casks`.

- [ ] **Step 3: Validate workflow structure**

Run:

```bash
rtk ruby -e 'require "yaml"; YAML.load_file(".github/workflows/release.yml", aliases: true); YAML.load_file(".goreleaser.yaml", aliases: true)'
rtk rg -n "push:|branches: \[main\]|release_created|tag_name|fetch-depth: 0|mode: keep-existing|TAP_GITHUB_TOKEN" .github/workflows/release.yml .goreleaser.yaml
```

Expected: Ruby exits `0`; search shows branch trigger, all three conditional release checks, exact tag checkout, full history, explicit note-preservation mode, and tap token.

- [ ] **Step 4: Validate GoReleaser configuration**

Run:

```bash
rtk goreleaser check
rtk goreleaser release --snapshot --clean --skip=publish
```

Expected: configuration validation succeeds; snapshot creates Linux and Darwin archives for amd64 and arm64 plus `checksums.txt` and generated Homebrew cask under `dist/`, without publishing.

- [ ] **Step 5: Run repository verification**

Run:

```bash
rtk go test ./...
rtk git diff --check
rtk git status --short
```

Expected: all Go tests pass; no whitespace errors; only intended workflow and GoReleaser files remain uncommitted for this task. Generated `dist/` remains ignored.

- [ ] **Step 6: Commit workflow integration**

Use `commit` skill. Stage only release workflow and GoReleaser configuration:

```bash
rtk git add .github/workflows/release.yml .goreleaser.yaml
```

Commit message:

```text
ci: automate release PR publication
```

### Task 3: Verify first-release behavior after publishing changes

**Files:**
- No repository files created or modified.

**Interfaces:**
- Consumes: pushed configuration commits and GitHub Actions state.
- Produces: evidence that Release Please creates a `v0.1.0` release PR without prematurely running GoReleaser.

- [ ] **Step 1: Push implementation branch through normal review path**

Push branch and merge through repository's normal PR process. Do not create tag manually.

- [ ] **Step 2: Inspect release workflow run**

Run:

```bash
gh run list --workflow release.yml --branch main --limit 5 --json databaseId,displayTitle,status,conclusion,url
gh pr list --state open --search 'label:autorelease:pending' --json number,title,url,headRefName,baseRefName
```

Expected: latest release workflow succeeds; one open Release Please PR targets `main`; no GoReleaser publication step ran.

- [ ] **Step 3: Verify bootstrap release PR content**

Resolve and inspect release PR:

```bash
RELEASE_PR_NUMBER="$(gh pr list --state open --search 'label:autorelease:pending' --json number --jq '.[0].number')"
gh pr view "$RELEASE_PR_NUMBER" --json title,body,files,labels,url
```

Expected: title identifies release `v0.1.0`; PR contains generated `CHANGELOG.md` and manifest version update; label includes `autorelease: pending`.

- [ ] **Step 4: Stop before publication**

Do not merge release PR during implementation verification. Report PR URL and workflow URL to maintainer. Merge remains explicit publication approval required by design.

## Final Verification Checklist

- [ ] `release-please-config.json` and `.release-please-manifest.json` parse and select root Go package.
- [ ] Release workflow triggers only on pushes to `main`.
- [ ] GoReleaser steps run only when `release_created == 'true'`.
- [ ] Release tag is checked out with `fetch-depth: 0`.
- [ ] GoReleaser preserves Release Please notes with `mode: keep-existing`.
- [ ] Existing artifact matrix, checksum, linker flags, and Homebrew cask configuration remain unchanged.
- [ ] `rtk goreleaser check`, snapshot release, and `rtk go test ./...` pass.
- [ ] Initial post-merge run creates release PR for `v0.1.0`, not immediate release.
- [ ] Release PR remains unmerged for maintainer approval.
- [ ] Manual recovery rejects invalid/missing tags, drafts, and releases with existing assets before GoReleaser runs.
