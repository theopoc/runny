# Renovate Dependency Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Configure hosted Renovate automation for Go security fixes, immutable GitHub Actions and Docker updates, CI-gated squash merges, and best-effort Telegram CI notifications.

**Architecture:** `renovate.json` limits extraction to Go modules, GitHub Actions, and Dockerfiles, then applies manager-specific pinning and merge rules. Renovate uses OSV for Go security fixes and performs its own automerge after CI because GitHub branch protection is unavailable on the current private-repository plan. Existing CI remains the quality gate, gains a Docker build, and feeds a separate non-blocking Telegram notification job.

**Tech Stack:** Renovate 43.257.7, Renovate GitHub App, OSV, GitHub Actions, Go 1.26.x, Docker, GoReleaser v2, `appleboy/telegram-action`, GitHub CLI.

## Global Constraints

- Do not create `.github/dependabot.yml` or enable GitHub Dependabot alerts/security updates.
- Ordinary Go module updates stay disabled; only OSV vulnerability remediations may update Go modules.
- Go security patch/minor updates require `minimumReleaseAge: "14 days"`, a known release timestamp, and green CI before squash automerge.
- Go security major updates create PRs immediately but always require manual review and squash merge.
- GitHub Actions and Docker references stay pinned to immutable SHA/digest for every update type, including majors.
- GitHub Actions and Docker pin/digest/patch/minor updates squash-automerge after green CI; majors never automerge.
- Use `platformAutomerge: false` and `ignoreTests: false`; GitHub native auto-merge remains disabled.
- No update or automerge schedule; process whenever hosted Renovate runs.
- Telegram covers every PR and `main` push CI run but never becomes a merge gate.
- Preserve existing `?`, `ctrl+c`, and `del`/`x` TUI behavior; this work must not modify Go application behavior.
- Preserve unrelated worktree changes and commit only files listed by each task.
- Execute implementation on branch `feat/renovate-automation`, created from the commit containing this plan.

---

## File Structure

- Create `renovate.json`: complete repository-level Renovate extraction, security, pinning, grouping, labeling, and merge policy.
- Modify `.github/workflows/ci.yml`: retain existing Go/GoReleaser gate, add Docker build, and add best-effort Telegram notification job.
- No Go source or test files change.
- No Dependabot file is created.

---

### Task 1: Add repository Renovate policy

**Files:**
- Create: `renovate.json`
- Reference: `docs/superpowers/specs/2026-07-10-renovate-automation-design.md`

**Interfaces:**
- Consumes: hosted Renovate GitHub App repository configuration; managers `gomod`, `github-actions`, and `dockerfile`.
- Produces: OSV-only Go remediation rules, immutable Action/Docker references, labels consumed by GitHub, and Renovate-owned squash automerge decisions consumed by CI.

- [ ] **Step 1: Confirm no competing dependency bot configuration exists**

Run:

```bash
rtk test ! -e .github/dependabot.yml
rtk test ! -e renovate.json
```

Expected: both commands exit `0` with no output.

- [ ] **Step 2: Create exact Renovate repository configuration**

Create `renovate.json` with:

```json
{
  "$schema": "https://docs.renovatebot.com/renovate-schema.json",
  "extends": [
    "config:recommended"
  ],
  "enabledManagers": [
    "gomod",
    "github-actions",
    "dockerfile"
  ],
  "timezone": "Europe/Paris",
  "labels": [
    "dependencies",
    "renovate"
  ],
  "semanticCommits": "enabled",
  "semanticCommitType": "chore",
  "semanticCommitScope": "deps",
  "dependencyDashboard": true,
  "dependencyDashboardTitle": "🤖 Dependency Updates Dashboard",
  "dependencyDashboardLabels": [
    "dependencies",
    "renovate-dashboard"
  ],
  "dependencyDashboardApproval": false,
  "prConcurrentLimit": 3,
  "branchConcurrentLimit": 5,
  "platformAutomerge": false,
  "ignoreTests": false,
  "automergeType": "pr",
  "automergeStrategy": "squash",
  "separateMajorMinor": true,
  "separateMinorPatch": false,
  "osvVulnerabilityAlerts": true,
  "vulnerabilityAlerts": {
    "enabled": true,
    "minimumReleaseAge": "14 days",
    "minimumReleaseAgeBehaviour": "timestamp-required",
    "prCreation": "immediate",
    "vulnerabilityFixStrategy": "lowest",
    "addLabels": [
      "security"
    ]
  },
  "packageRules": [
    {
      "description": "Disable ordinary Go module updates",
      "matchManagers": [
        "gomod"
      ],
      "enabled": false
    },
    {
      "description": "Automerge non-major Go vulnerability fixes after release age",
      "matchManagers": [
        "gomod"
      ],
      "matchUpdateTypes": [
        "minor",
        "patch"
      ],
      "postUpdateOptions": [
        "gomodTidy"
      ],
      "automerge": true,
      "automergeType": "pr",
      "automergeStrategy": "squash"
    },
    {
      "description": "Require manual review for major Go vulnerability fixes",
      "matchManagers": [
        "gomod"
      ],
      "matchUpdateTypes": [
        "major"
      ],
      "postUpdateOptions": [
        "gomodTidy"
      ],
      "automerge": false,
      "addLabels": [
        "major-update"
      ]
    },
    {
      "description": "Group and pin GitHub Actions",
      "matchManagers": [
        "github-actions"
      ],
      "groupName": "GitHub Actions",
      "pinDigests": true
    },
    {
      "description": "Automerge non-major GitHub Actions and digest updates",
      "matchManagers": [
        "github-actions"
      ],
      "matchUpdateTypes": [
        "pin",
        "pinDigest",
        "digest",
        "patch",
        "minor"
      ],
      "automerge": true
    },
    {
      "description": "Require manual review for major GitHub Actions",
      "matchManagers": [
        "github-actions"
      ],
      "matchUpdateTypes": [
        "major"
      ],
      "pinDigests": true,
      "automerge": false,
      "addLabels": [
        "major-update"
      ]
    },
    {
      "description": "Group and pin Dockerfile images",
      "matchManagers": [
        "dockerfile"
      ],
      "groupName": "Docker images",
      "pinDigests": true
    },
    {
      "description": "Automerge non-major Dockerfile and digest updates",
      "matchManagers": [
        "dockerfile"
      ],
      "matchUpdateTypes": [
        "pin",
        "pinDigest",
        "digest",
        "patch",
        "minor"
      ],
      "automerge": true
    },
    {
      "description": "Require manual review for major Dockerfile images",
      "matchManagers": [
        "dockerfile"
      ],
      "matchUpdateTypes": [
        "major"
      ],
      "pinDigests": true,
      "automerge": false,
      "addLabels": [
        "major-update"
      ]
    }
  ]
}
```

- [ ] **Step 3: Verify JSON syntax and policy invariants**

Run:

```bash
rtk jq empty renovate.json
rtk jq -e '.enabledManagers == ["gomod", "github-actions", "dockerfile"]' renovate.json
rtk jq -e '.platformAutomerge == false and .ignoreTests == false' renovate.json
rtk jq -e '.osvVulnerabilityAlerts == true and .vulnerabilityAlerts.minimumReleaseAge == "14 days"' renovate.json
rtk jq -e '[.packageRules[] | select(.matchManagers == ["gomod"] and .enabled == false)] | length == 1' renovate.json
rtk test ! -e .github/dependabot.yml
```

Expected: every command exits `0`; no command prints `false` or an error.

- [ ] **Step 4: Validate against pinned Renovate schema/runtime**

Run:

```bash
rtk npx --yes --package renovate@43.257.7 -- renovate-config-validator --strict
```

Expected output includes:

```text
INFO: Validating renovate.json
INFO: Config validated successfully
```

Any migration warning or validation warning is failure; update JSON to current non-deprecated option names before continuing.

- [ ] **Step 5: Verify manager extraction without repository mutation**

Run:

```bash
rtk npx --yes --package renovate@43.257.7 -- renovate --platform=local --dry-run=extract --log-level=debug
```

Expected: exit `0`; `Dependency extraction complete` reports only `gomod`, `github-actions`, and `dockerfile`. Local-platform dry-run does not create branches or PRs and cannot prove GitHub App vulnerability-alert behavior.

- [ ] **Step 6: Review configuration diff**

Run:

```bash
rtk git diff --check
rtk git diff -- renovate.json
```

Expected: no whitespace errors; diff contains only new `renovate.json`.

- [ ] **Step 7: Commit Renovate policy**

```bash
rtk git add renovate.json
rtk git commit -m "chore(deps): configure Renovate automation"
```

Expected: one commit containing only `renovate.json`.

---

### Task 2: Extend CI with Docker validation and Telegram notifications

**Files:**
- Modify: `.github/workflows/ci.yml:12-27`
- Reference: `.github/workflows/release.yml:1-29`

**Interfaces:**
- Consumes: existing `test` job, repository secrets `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`, and PR/main events.
- Produces: `ci / test` quality signal consumed by Renovate and a separate best-effort `ci / notify` status plus Telegram message.

- [ ] **Step 1: Record current quality gate identity**

Run:

```bash
rtk gh pr view 17 --repo theopoc/runny --json statusCheckRollup --jq '.statusCheckRollup[] | [.name,.workflowName,.conclusion] | @tsv'
```

Expected:

```text
test	ci	SUCCESS
```

Keep job ID `test` unchanged so Renovate and future branch protection observe stable check `ci / test`.

- [ ] **Step 2: Replace CI workflow with Docker and Telegram-aware version**

Update `.github/workflows/ci.yml` to exactly:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: actions/setup-go@v6
        with:
          go-version: "1.26.x"
          cache: true
      - run: test -z "$(gofmt -l $(find . -name '*.go' -not -path './dist/*'))"
      - run: git diff --exit-code
      - run: go vet ./...
      - run: go test ./...
      - run: go build ./cmd/runny
      - uses: goreleaser/goreleaser-action@v6
        with:
          args: check
      - run: docker build -t runny:ci .

  notify:
    needs: test
    if: ${{ always() }}
    runs-on: ubuntu-latest
    continue-on-error: true
    env:
      TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
      TELEGRAM_CHAT_ID: ${{ secrets.TELEGRAM_CHAT_ID }}
    steps:
      - name: Report missing Telegram secrets
        if: ${{ env.TELEGRAM_BOT_TOKEN == '' || env.TELEGRAM_CHAT_ID == '' }}
        run: echo "::warning::Telegram secrets unavailable; notification skipped"
      - name: Send Telegram notification
        if: ${{ env.TELEGRAM_BOT_TOKEN != '' && env.TELEGRAM_CHAT_ID != '' }}
        uses: appleboy/telegram-action@4cd2253b24c9682a5be99eb6f7ab3484586978fa # master
        with:
          to: ${{ env.TELEGRAM_CHAT_ID }}
          token: ${{ env.TELEGRAM_BOT_TOKEN }}
          format: html
          disable_web_page_preview: true
          message: |
            ${{ needs.test.result == 'success' && '✅' || '❌' }} <b>CI ${{ needs.test.result }}</b>

            <b>Repository:</b> <code>${{ github.repository }}</code>
            <b>Branch:</b> <code>${{ github.head_ref || github.ref_name }}</code>
            <b>Event:</b> ${{ github.event_name }}
            <b>Commit:</b> <code>${{ github.event.pull_request.head.sha || github.sha }}</code>
            <b>Author:</b> ${{ github.actor }}
            <b>Test:</b> ${{ needs.test.result }}

            <a href="${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}">View workflow run</a>
```

Existing Action tags intentionally remain unpinned in this implementation commit. Renovate's first eligible `pinDigest` PR must pin them and then maintain every SHA, including majors.

- [ ] **Step 3: Validate GitHub Actions syntax**

Run:

```bash
rtk go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml
```

Expected: exit `0` with no output.

- [ ] **Step 4: Run local quality gate in workflow order**

Run:

```bash
rtk test -z "$(gofmt -l $(find . -name '*.go' -not -path './dist/*'))"
rtk git diff --exit-code -- '*.go'
rtk go vet ./...
rtk go test ./...
rtk go build ./cmd/runny
rtk goreleaser check
rtk docker build -t runny:ci .
```

Expected: every command exits `0`; Go tests pass; GoReleaser reports configuration valid; Docker emits successful image build.

- [ ] **Step 5: Verify notification cannot become merge gate**

Run:

```bash
rtk rg -n 'needs: test|if: \$\{\{ always\(\) \}\}|continue-on-error: true|Telegram secrets unavailable' .github/workflows/ci.yml
rtk rg -n 'appleboy/telegram-action@4cd2253b24c9682a5be99eb6f7ab3484586978fa' .github/workflows/ci.yml
```

Expected: first command finds all four safeguards; second finds immutable Telegram Action SHA exactly once.

- [ ] **Step 6: Review workflow diff**

Run:

```bash
rtk git diff --check
rtk git diff -- .github/workflows/ci.yml
```

Expected: only Docker build and `notify` job differ from current workflow.

- [ ] **Step 7: Commit CI changes**

```bash
rtk git add .github/workflows/ci.yml
rtk git commit -m "ci: validate Docker and notify Telegram"
```

Expected: one commit containing only `.github/workflows/ci.yml`.

---

### Task 3: Prepare GitHub labels and verify disabled features

**Files:**
- Verify absent: `.github/dependabot.yml`
- No tracked file changes.

**Interfaces:**
- Consumes: `renovate.json` labels and current GitHub repository administration access.
- Produces: labels accepted by Renovate PRs and verified GitHub settings for safe hosted-App rollout.

- [ ] **Step 1: Create or normalize Renovate labels**

Run:

```bash
rtk gh label create dependencies --repo theopoc/runny --color 0366d6 --description "Dependency updates" --force
rtk gh label create renovate --repo theopoc/runny --color 1f883d --description "Managed by Renovate" --force
rtk gh label create renovate-dashboard --repo theopoc/runny --color 8250df --description "Renovate Dependency Dashboard" --force
rtk gh label create security --repo theopoc/runny --color d73a4a --description "Security remediation" --force
rtk gh label create major-update --repo theopoc/runny --color b60205 --description "Major dependency update requiring manual review" --force
```

Expected: each command succeeds; rerunning commands remains idempotent.

- [ ] **Step 2: Verify label inventory**

Run:

```bash
rtk gh label list --repo theopoc/runny --limit 100 --json name --jq 'map(.name) | map(select(. == "dependencies" or . == "renovate" or . == "renovate-dashboard" or . == "security" or . == "major-update")) | sort | .[]'
```

Expected:

```text
dependencies
major-update
renovate
renovate-dashboard
security
```

- [ ] **Step 3: Verify Dependabot remains absent and disabled**

Run:

```bash
rtk test ! -e .github/dependabot.yml
rtk gh api -i repos/theopoc/runny/vulnerability-alerts
rtk gh api repos/theopoc/runny/automated-security-fixes --jq '{enabled,paused}'
```

Expected: file assertion exits `0`; vulnerability-alert request returns HTTP `404` with `"Vulnerability alerts are disabled."`; automated security fixes returns:

```json
{"enabled":false,"paused":false}
```

The expected HTTP `404` makes that command non-zero; record output as proof rather than treating it as implementation failure.

- [ ] **Step 4: Verify current branch-protection limitation without changing it**

Run:

```bash
rtk gh api repos/theopoc/runny/branches/main/protection
```

Expected: HTTP `403` with `"Upgrade to GitHub Pro or make this repository public to enable this feature."` Do not enable GitHub native auto-merge or change repository visibility.

- [ ] **Step 5: Confirm GitHub metadata task changed no tracked files**

Run:

```bash
rtk git status --short
```

Expected: no new tracked or untracked changes from Task 3. No commit is created because labels/settings are GitHub metadata, not repository files.

---

### Task 4: Publish, complete manual prerequisites, and activate Renovate

**Files:**
- Verify: `renovate.json`
- Verify: `.github/workflows/ci.yml`
- Verify: `docs/superpowers/specs/2026-07-10-renovate-automation-design.md`
- Verify: `docs/superpowers/plans/2026-07-11-renovate-automation.md`

**Interfaces:**
- Consumes: Tasks 1-3, repository owner access, Telegram credentials, Renovate GitHub App installation.
- Produces: merged configuration on `main`, active hosted Renovate processing, Dependency Dashboard, initial immutable pin PRs, and CI/Telegram evidence.

- [ ] **Step 1: Run final local verification**

Run:

```bash
rtk npx --yes --package renovate@43.257.7 -- renovate-config-validator --strict
rtk go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml
rtk go vet ./...
rtk go test ./...
rtk go build ./cmd/runny
rtk goreleaser check
rtk docker build -t runny:ci .
rtk git diff --check
rtk git status --short --branch
```

Expected: validators and builds exit `0`; tests pass; worktree is clean on implementation branch.

- [ ] **Step 2: Manual checkpoint — copy Telegram secrets before merge**

Repository owner performs:

1. Open `theopoc/runny` -> **Settings** -> **Secrets and variables** -> **Actions**.
2. Create `TELEGRAM_BOT_TOKEN` from original Telegram bot token.
3. Create `TELEGRAM_CHAT_ID` from destination chat ID.

Then verify names only:

```bash
rtk gh secret list --repo theopoc/runny --app actions
```

Expected: output contains `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`, and existing `TAP_GITHUB_TOKEN`. Never print secret values.

- [ ] **Step 3: Push implementation branch and open pull request**

Push implementation branch `feat/renovate-automation` and create PR with:

```bash
rtk git push -u origin HEAD
rtk gh pr create --repo theopoc/runny --base main --head feat/renovate-automation --title "chore(deps): add Renovate automation" --body-file - <<'EOF'
## Summary
- configure OSV-only Go security remediation with 14-day release age
- pin and automate non-major GitHub Actions and Docker updates
- add Docker CI validation and best-effort Telegram notifications
- keep Dependabot disabled and all major updates manual

## Verification
- `renovate-config-validator --strict`
- `actionlint`
- `go vet ./...`
- `go test ./...`
- `go build ./cmd/runny`
- `goreleaser check`
- `docker build -t runny:ci .`

## Manual prerequisites
- [x] `TELEGRAM_BOT_TOKEN` added to `runny`
- [x] `TELEGRAM_CHAT_ID` added to `runny`
- [ ] add `runny` to Renovate App only after this PR merges
EOF
```

Expected: branch push succeeds; PR URL returned; implementation PR CI starts.

- [ ] **Step 4: Verify PR CI and Telegram success notification**

Run:

```bash
rtk gh pr checks --repo theopoc/runny --watch
```

Expected: `ci / test` passes; `ci / notify` completes without blocking; Telegram receives success message containing matching repository, branch, event, SHA, actor, and workflow link.

If notification is absent, inspect `ci / notify`; fix secrets or action configuration without weakening `ci / test`.

- [ ] **Step 5: Merge implementation PR with squash strategy**

After all checks pass:

```bash
rtk gh pr merge --repo theopoc/runny --squash --delete-branch
```

Expected: PR merges into `main`; main push CI starts and sends Telegram notification.

- [ ] **Step 6: Manual checkpoint — grant Renovate App repository access**

Repository owner performs after config exists on `main`:

1. Open <https://github.com/settings/installations> as `theopoc`.
2. Select **Configure** for Renovate.
3. Under **Repository access**, add `runny` when using selected repositories; leave **All repositories** unchanged if already selected.
4. Save.

Expected: Renovate processes `runny` without opening a generic onboarding config PR because `renovate.json` already exists on `main`.

- [ ] **Step 7: Verify Dependency Dashboard and initial PR policy**

After Renovate run completes, run:

```bash
rtk gh issue list --repo theopoc/runny --state open --search 'Dependency Updates Dashboard in:title' --json number,title,author,url
rtk gh pr list --repo theopoc/runny --state open --search 'author:app/renovate' --json number,title,labels,headRefName,url
```

Expected:

- Dashboard issue authored by `app/renovate` exists.
- Initial GitHub Actions and Docker pin PRs use immutable SHA/digest.
- Pin/digest/patch/minor PR bodies state `Automerge: Enabled` and wait for CI.
- Major PR bodies state `Automerge: Disabled by config`, carry `major-update`, and remain manual.
- No ordinary Go version-update PR exists.
- Any Go remediation PR is OSV-backed, carries `security`, and enforces 14-day release age before automerge.

- [ ] **Step 8: Record branch-protection follow-up without blocking rollout**

Repository owner retains future manual checklist from design spec. When repository becomes public or account gains GitHub Pro, create active `main` ruleset requiring PRs, `ci / test`, branches up to date, blocked force-push/deletion, zero approval count, and no Renovate/admin bypass. Until then, keep `platformAutomerge: false` and GitHub native auto-merge disabled.

Expected: rollout completes without changing plan, visibility, or branch protection.

---

## Rollback

If Renovate behavior violates policy:

1. Remove `runny` from Renovate GitHub App repository access.
2. Disable Renovate by adding top-level `"enabled": false` in `renovate.json` through reviewed PR, or revert Task 1 commit.
3. Leave Docker CI and Telegram notification in place unless they independently fail.
4. Close unwanted Renovate PRs after App access is removed.
5. Re-run Dependabot status checks and confirm it remains disabled.

Rollback must not enable floating Action/Docker references or bypass CI.
