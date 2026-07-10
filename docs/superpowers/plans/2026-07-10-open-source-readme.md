# Open-Source README Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a community-ready README with verified badges, a Ghostty-recorded TUI demo, and an explicit 100% vibe-coded note.

**Architecture:** Keep product behavior unchanged. Build a temporary three-project workspace, drive a real `runny` session through Ghostty, record the terminal window, encode the recording as `demo/runny.gif`, then reference that artifact from a rewritten README.

**Tech Stack:** Go 1.26.x, Bubble Tea TUI, Ghostty terminal automation, macOS `screencapture`, FFmpeg, GitHub Markdown, Shields.io.

## Global Constraints

- Preserve CLI, config, logging, discovery, and TUI behavior.
- Use canonical repository identity `theopoc/runny`.
- Use exactly one static GitHub repository badge, one static Releases link badge, one Go badge, and one MIT badge, all with `style=for-the-badge`.
- Use no dynamic CI or release badge while the repository is private.
- Omit Brewfile and project-structure sections.
- State plainly that the project is 100% vibe coded without treating that fact as quality or security evidence.
- Commit no temporary demo fixtures or raw recording.
- Keep `wc -l '*'` as the recorded command.

---

### Task 1: Record TUI Demo

**Files:**
- Create: `demo/runny.gif`
- Temporary only: `/tmp/runny-demo/`
- Temporary only: `/tmp/runny-demo.mov`

**Interfaces:**
- Consumes: built binary `/tmp/runny-readme-demo`; repository worktree; Ghostty terminal automation.
- Produces: looping GIF at `demo/runny.gif` for README embedding.

- [ ] **Step 1: Build demo binary and fixtures**

Run:

```bash
go build -o /tmp/runny-readme-demo ./cmd/runny
rm -rf /tmp/runny-demo
mkdir -p /tmp/runny-demo/{api,web,worker}
printf 'package api\n\nfunc Handler() {}\n' > /tmp/runny-demo/api/handlers.go
printf 'GET /health\nPOST /jobs\n' > /tmp/runny-demo/api/routes.txt
printf 'export const App = () => null\n' > /tmp/runny-demo/web/app.tsx
printf 'body {}\nmain {}\nbutton {}\n' > /tmp/runny-demo/web/styles.css
printf 'package worker\n\nfunc Run() {}\n\nfunc Stop() {}\n' > /tmp/runny-demo/worker/jobs.go
printf 'critical\ndefault\nretry\narchive\n' > /tmp/runny-demo/worker/queues.txt
```

Expected: three discoverable child directories containing files with distinct line counts.

- [ ] **Step 2: Prepare dedicated Ghostty terminal**

Create a Ghostty window in `/tmp/runny-demo`, resize it to 120 columns by 36 rows, read the blank shell state, and focus it. Start the app with:

```bash
/tmp/runny-readme-demo -- wc -l '*'
```

Expected: directory panel lists `api`, `web`, and `worker`; command header shows `wc -l *`.

- [ ] **Step 3: Verify visual baseline**

Use Ghostty `wait_idle`, `read`, `cells`, and `screenshot`. Confirm all target names are visible, borders and status colors render, and no text truncation harms the demo.

Expected screenshot: `/tmp/runny-demo-baseline.png` at stable initial selection screen.

- [ ] **Step 4: Record scripted interaction**

Determine focused Ghostty window bounds, then record 15 seconds:

```bash
bounds=$(osascript -e 'tell application "System Events" to tell process "Ghostty" to tell front window to return (position & size) as list' | tr -d ' ')
/usr/sbin/screencapture -v -V15 -R"$bounds" /tmp/runny-demo.mov
```

During recording, use Ghostty automation in this order:

1. wait 1 second;
2. press `a` to select all targets;
3. wait 1 second;
4. press `Enter` to run;
5. wait for all three results to settle;
6. press `j` twice with short pauses to show distinct output;
7. press `?` to show help;
8. hold final help screen until recording ends.

Expected: real TUI sequence with no shell setup visible.

- [ ] **Step 5: Encode optimized GIF**

Install FFmpeg with `brew install ffmpeg` only if `command -v ffmpeg` fails. Encode using a generated palette:

```bash
mkdir -p demo
ffmpeg -y -i /tmp/runny-demo.mov -vf "fps=15,scale=1200:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=128[p];[s1][p]paletteuse=dither=bayer" -loop 0 demo/runny.gif
```

Expected: looping GIF, readable at GitHub README width, no raw `.mov` tracked.

- [ ] **Step 6: Inspect artifact**

Run:

```bash
file demo/runny.gif
du -h demo/runny.gif
git status --short
```

Render first and representative frames locally. Confirm output changes between `api`, `web`, and `worker`, help appears last, and file size remains practical for repository cloning.

- [ ] **Step 7: Commit demo artifact**

```bash
git add demo/runny.gif
git commit -m "docs: add TUI demo"
```

### Task 2: Rewrite README

**Files:**
- Modify: `README.md`

**Interfaces:**
- Consumes: `demo/runny.gif`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `go.mod`, `.goreleaser.yaml`, `LICENSE`.
- Produces: GitHub landing page linking verified project resources and demo artifact.

- [ ] **Step 1: Rewrite hero and overview**

Use centered `runny` heading, concise tagline, and exactly four Shields.io badges: static GitHub repository, static Releases link, Go 1.26.x, and MIT license. Every badge must use `style=for-the-badge`. Do not add dynamic CI or release-status badges because unauthenticated badge requests cannot read the private repository and the release workflow has no run.

Embed demo directly after overview:

```markdown
![runny TUI demo](demo/runny.gif)
```

Add plain statement: `runny is 100% vibe coded.`

- [ ] **Step 2: Add reader-first project guidance**

Order sections as overview, features, installation, quick start, Docker, flags, configuration, shortcuts, development, contributing, and license. Preserve current commands and behavior. Omit Brewfile and project-structure content.

- [ ] **Step 3: Add contribution path**

Link issues and pull requests at `https://github.com/theopoc/runny`. Require `go test ./...` and `go build ./cmd/runny` before PR submission. Do not create or imply nonexistent social channels or governance documents.

- [ ] **Step 4: Validate README facts**

Run:

```bash
rg -n 'TODO|TBD|\{\{|saewyn/runny' README.md
rg -o 'https://[^ )]+' README.md
for url in $(rg -o 'https://img\.shields\.io/[^)]+' README.md); do curl -fsSL "$url"; done | rg 'NOT FOUND|NO STATUS'
git diff --check
```

Expected: first and badge-status scans return no matches; links use `theopoc/runny`; diff check exits successfully. Badge labels cover GitHub, Releases, Go, and MIT exactly once each.

- [ ] **Step 5: Commit README**

```bash
git add README.md
git commit -m "docs: refresh README for open source"
```

### Task 3: Verify and Finalize Documentation

**Files:**
- Verify: `README.md`
- Verify: `demo/runny.gif`
- Verify: `docs/superpowers/specs/2026-07-10-open-source-readme-design.md`
- Verify: `docs/superpowers/plans/2026-07-10-open-source-readme.md`

**Interfaces:**
- Consumes: all branch changes.
- Produces: fully verified, review-ready branch with all documentation artifacts committed.

- [ ] **Step 1: Run fresh project verification**

```bash
go test ./...
go vet ./...
go build ./cmd/runny
git diff --check main...HEAD
git status -sb
```

Expected: 139 tests pass, vet/build exit zero, no whitespace errors, no unrelated files.

- [ ] **Step 2: Review complete diff**

```bash
git diff --stat main...HEAD
git diff main...HEAD -- README.md docs/superpowers/specs/2026-07-10-open-source-readme-design.md docs/superpowers/plans/2026-07-10-open-source-readme.md
```

Expected: documentation design, plan, README, and GIF only.

- [ ] **Step 3: Commit plan if still uncommitted**

```bash
git add docs/superpowers/plans/2026-07-10-open-source-readme.md
git commit -m "docs: add README implementation plan"
```

- [ ] **Step 4: Confirm clean review scope**

```bash
git status -sb
git diff --name-only main...HEAD
```

Expected: clean worktree; branch diff contains only README, GIF, design spec, and implementation plan.

## Post-Review Publication

After the final whole-branch review approves the implementation:

```bash
git push -u origin agent/improve-open-source-readme
```

Open a draft PR titled `docs: refresh README for open source` against `main`. Body must summarize README structure, verified badges, Ghostty demo, vibe-coded disclosure, and exact validation commands.
