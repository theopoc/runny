# Renovate Dependency Automation Design

## Goal

Automate dependency maintenance for `runny` with the hosted Renovate GitHub
App. Non-major updates may merge without human action only when their manager
policy allows it and the complete CI gate passes. Major updates always require
manual review and merge.

Dependabot is out of scope and must remain disabled. The repository must not
gain a `.github/dependabot.yml` file, Dependabot alerts, or Dependabot security
updates.

## Current Repository Constraints

- `theopoc/runny` is private.
- The current GitHub plan does not provide branch protection or repository
  rulesets for this private repository.
- GitHub native auto-merge is disabled.
- GitHub Actions has read-only default workflow permissions.
- The existing pull-request CI runs formatting, diff, vet, tests, build, and a
  GoReleaser configuration check.
- CI does not currently build the Docker image or notify Telegram.
- Renovate has not previously opened a pull request in this repository.
- Telegram secrets are not configured in `runny`.

These constraints rule out safe GitHub-native auto-merge. Renovate must perform
its own merge decision after observing successful checks by using
`platformAutomerge: false` and `ignoreTests: false`.

## Architecture

The dependency flow is:

```text
Renovate App / OSV
  -> Renovate pull request
  -> runny CI gate
  -> Renovate squash merge or manual major review
  -> main CI
  -> Telegram notification
```

Components:

- `renovate.json` defines manager-specific extraction, pinning, age, and merge
  policies.
- The hosted Renovate GitHub App detects updates, creates and rebases pull
  requests, and performs eligible squash merges.
- Renovate's OSV integration is the only vulnerability source for Go modules.
- `.github/workflows/ci.yml` remains the quality gate and gains Docker build and
  Telegram notification coverage.
- GitHub labels distinguish Renovate, security, and major pull requests.

Renovate's configuration object named `vulnerabilityAlerts` is used only to
control pull-request behavior for OSV-generated fixes. No GitHub Dependabot
alert data is enabled or relied upon. `osvVulnerabilityAlerts` is currently an
experimental Renovate feature; failure to obtain actionable OSV data must fail
closed rather than enabling ordinary Go version updates.

## Branch Protection Recommendation

Branch protection is strongly recommended. It turns the CI policy from a
Renovate convention into a repository-wide enforcement rule: automated,
manual, and administrator merges cannot bypass the required `test` check.

It cannot be enabled today because `theopoc/runny` is private and the current
GitHub plan rejects both branch protection and repository rulesets. This does
not block the initial implementation: `platformAutomerge: false` makes Renovate
wait for observed checks. The limitation remains important because direct
pushes and manual administrator merges are not protected.

If the repository becomes public or the account gains GitHub Pro, create an
active branch ruleset for `main` with these settings:

- require changes through a pull request;
- require the `test` status check from the `ci` workflow;
- require the branch to be up to date before merging;
- block force pushes and branch deletion;
- apply the rules to administrators without a Renovate bypass;
- require zero approving reviews, which keeps a single-maintainer repository
  usable while preserving the separately documented manual-major policy.

GitHub native auto-merge should remain disabled. Renovate-owned automerge with
`platformAutomerge: false` remains the merge controller even after branch
protection becomes available.

## Update Policy

### Go modules

- Disable every ordinary Go module version update.
- Enable OSV vulnerability detection.
- Create security pull requests immediately.
- For security patch and minor updates:
  - require a 14-day `minimumReleaseAge`;
  - require all CI checks to pass;
  - squash-merge automatically through Renovate.
- For security major updates:
  - create the pull request immediately;
  - add `security` and `major-update` labels;
  - never auto-merge;
  - require manual review and squash merge.
- When a release timestamp is missing, treat the age check as unsatisfied.
- When OSV reports no fixed version, do not create or merge a remediation PR.

The age check controls automatic merge eligibility. A major security pull
request remains manual regardless of age or CI result.

### GitHub Actions

- Pin every action reference to an immutable commit SHA.
- Preserve a readable version comment beside each pinned SHA.
- Group compatible GitHub Actions updates.
- Auto-merge initial pin, digest, patch, and minor updates after successful CI.
- Create major update pull requests immediately, pinned to the proposed major's
  exact SHA, but never auto-merge them.
- Major pull requests require manual review and squash merge.

### Dockerfile

- Pin every base image to an immutable digest while retaining its readable tag.
- Group compatible Docker image updates.
- Auto-merge initial pin, digest, patch, and minor updates after successful CI.
- Create major update pull requests immediately, pinned to the proposed major's
  exact digest, but never auto-merge them.
- Major pull requests require manual review and squash merge.

### Shared Renovate Behavior

- Extend Renovate's recommended configuration but use explicit local rules for
  the three managers.
- Detect updates whenever the hosted Renovate App runs; define no repository
  schedule or automerge window.
- Use pull-request automerge with squash strategy.
- Set `platformAutomerge: false` and `ignoreTests: false`.
- Enable Conventional Commit messages using `chore(deps): ...`.
- Enable the Dependency Dashboard without an approval gate.
- Keep major updates separate from non-major updates.
- Apply `dependencies` and `renovate` labels to all Renovate pull requests.
- Apply `security` to Go OSV remediation pull requests.
- Apply `major-update` to every major pull request.
- Bound concurrent Renovate pull requests and branches to avoid CI storms; use
  the proven `laptop_setup` limits of three PRs and five branches.

## CI Gate

The `test` job must execute, in order:

1. Check Go formatting.
2. Verify the worktree remains unchanged.
3. Run `go vet ./...`.
4. Run `go test ./...`.
5. Build `./cmd/runny`.
6. Run the GoReleaser configuration check.
7. Build the Docker image as `runny:ci`.

Any failure blocks Renovate automerge. Renovate may rebase and retry through a
new CI run, but automation must never force-merge a failing or pending change.

## Telegram Notifications

Add a separate notification job after the CI gate with `if: always()`.
Notifications cover every pull-request and `main` push CI run, matching the
operating model used by `laptop_setup`.

The message contains:

- success or failure indicator;
- repository;
- branch;
- event;
- commit SHA;
- actor;
- CI gate result;
- workflow-run link.

Use `appleboy/telegram-action` pinned to an immutable commit SHA, never
`@master`. Renovate subsequently maintains that digest under the GitHub Actions
policy.

Telegram is observability, not a merge gate. A missing secret, Telegram outage,
or action delivery failure must be logged or skipped without changing the CI
gate result.

## Error Handling

- CI failure: leave the PR open and disable merge eligibility.
- Pending 14-day age check: leave the PR open and pending.
- Missing release timestamp: fail closed and do not auto-merge.
- Missing fixed OSV version: expose the condition in Renovate output/dashboard;
  do not substitute an ordinary Go update.
- Merge conflict or stale branch: let Renovate rebase, then require fresh CI.
- Telegram failure: report best-effort failure without blocking dependency
  updates.
- Major update: never infer approval from green CI; only a human merge completes
  it.

## Manual Operations

### Required before activation

#### Give Renovate App access to `runny`

The current `gh` token cannot change GitHub App installation selection. The
repository owner must:

1. Open <https://github.com/settings/installations> while signed in as
   `theopoc`.
2. Find Renovate and select **Configure**.
3. Under **Repository access**, keep **All repositories** if already selected;
   otherwise choose **Only select repositories**.
4. Add `runny` to the selected repositories.
5. Save the installation changes.
6. After `renovate.json` reaches the default branch, open the repository issues
   and confirm Renovate creates or updates the Dependency Dashboard.

#### Add Telegram Actions secrets

GitHub does not expose existing secret values, so automation cannot copy them
from `laptop_setup`. The repository owner must use the original values and:

1. Open `theopoc/runny` on GitHub.
2. Open **Settings** -> **Secrets and variables** -> **Actions**.
3. Select **New repository secret**.
4. Create `TELEGRAM_BOT_TOKEN` with the Telegram bot token.
5. Select **New repository secret** again.
6. Create `TELEGRAM_CHAT_ID` with the destination chat ID.
7. Run or re-run CI and confirm the Telegram message links to that workflow
   run.

The implementation should be merged only after both secrets exist, preventing
the first production CI run from silently skipping notification.

### Required during normal operation

#### Review and merge a major update

Use the same process for Go security, GitHub Actions, and Docker majors:

1. Open the Renovate pull request carrying `major-update`.
2. Confirm GitHub Actions references use an immutable SHA and Docker images use
   an immutable digest; reject any floating reference.
3. Read the release notes, changelog, migration notes, and security advisory
   when applicable.
4. Inspect the diff for removed options, changed defaults, or required code and
   configuration migrations.
5. Wait for the complete `ci / test` check to pass.
6. If additional confidence is needed, check out the branch locally and rerun
   the relevant Go or Docker verification.
7. Apply required compatibility changes in a separate reviewed change or on the
   Renovate branch, then require fresh CI.
8. Select **Squash and merge** manually. Never enable auto-merge for that PR.

#### Investigate a Renovate PR with failing CI

1. Open the failed `ci / test` check from the pull request.
2. Identify whether failure comes from dependency incompatibility, a flaky
   test, Docker build, or workflow syntax/action behavior.
3. Do not merge or rerun until the failure has a concrete explanation.
4. For a flaky external failure, rerun the failed jobs and retain evidence.
5. For incompatibility, prepare a compatibility fix, update or rebase the
   Renovate branch, and require a new successful CI run.
6. If update cannot be made safe, close the PR or add a narrowly documented
   Renovate ignore rule; never weaken the global CI gate.

#### Handle an OSV vulnerability without a fixed release

1. Open the OSV advisory linked from the Dependency Dashboard or Renovate log.
2. Confirm affected package, current version, severity, and exploit conditions.
3. Check upstream issue/advisory for a pending fix and available mitigations.
4. Apply a documented mitigation, replace the dependency, or remove affected
   functionality when risk requires immediate action.
5. Track the advisory until a fixed version exists; do not convert it into an
   unrestricted ordinary Go update.

#### Enable branch protection when GitHub permits it

This future operation becomes available after making the repository public or
upgrading the account to GitHub Pro:

1. Open `theopoc/runny` -> **Settings** -> **Rules** -> **Rulesets**.
2. Select **New ruleset** -> **New branch ruleset**.
3. Name it `main protection`, set enforcement to **Active**, and target the
   default branch.
4. Enable pull-request-only changes with zero required approving reviews.
5. Enable required status checks and select `test` from workflow `ci`.
6. Require branches to be up to date before merging.
7. Block force pushes and deletions.
8. Do not add administrator or Renovate bypass actors.
9. Save, then verify with a test PR that merge stays blocked until `ci / test`
   succeeds.

No manual Dependabot operation is required because Dependabot remains disabled.

## Automated Operations

Implementation automates:

- repository Renovate configuration;
- CI Docker validation and Telegram messages;
- creation of missing Renovate labels;
- config, YAML, Go, Docker, and GoReleaser validation;
- verification that Dependabot remains disabled;
- eligible pull-request creation, rebasing, CI retry, and squash merge;
- Renovate Dependency Dashboard maintenance.

## Validation

Before publishing implementation changes:

- validate `renovate.json` with `renovate-config-validator`;
- run a Renovate dry-run and confirm only `gomod`, `github-actions`, and
  `dockerfile` managers are relevant;
- confirm ordinary Go updates are disabled and OSV remediation remains enabled;
- validate the workflow YAML;
- run `rtk go test ./...`;
- run `rtk go vet ./...`;
- run `rtk go build ./cmd/runny`;
- run `rtk docker build -t runny:ci .`;
- run the GoReleaser configuration check;
- run `rtk git diff --check`.

After GitHub activation:

- confirm the pull-request CI gate passes;
- confirm Telegram success and failure messages;
- confirm Renovate creates its Dependency Dashboard;
- inspect Renovate extraction for Go modules, GitHub Actions, and Dockerfile;
- confirm no ordinary Go update PR is proposed;
- confirm an eligible non-major PR states that automerge is enabled;
- confirm major PRs state that automerge is disabled;
- confirm Action and Docker updates, including majors, retain immutable digests.

## Rollback

To stop dependency automation, remove `runny` from Renovate App repository
access or disable Renovate in `renovate.json`. The CI Docker build and Telegram
notification may remain independently. No runtime dependency or long-lived
automation credential is added to `runny`.

## Out of Scope

- Dependabot configuration, alerts, or security updates.
- GitHub plan upgrades or repository visibility changes.
- Native GitHub auto-merge, branch protection, or merge queue.
- Automatic merge of any major dependency update.
- Container image CVE scanning beyond Renovate's version and digest management.
