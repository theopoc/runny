# Docker Usage and TUI Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Add Docker usage for `runny`, document Docker and TUI behavior, and harden TUI interaction tests.

**Architecture:** Keep Docker support outside application code through a multi-stage Dockerfile and `.dockerignore`. Keep TUI fixes inside `internal/tui` with tests that exercise model key handling and a pseudo-tty smoke test for the real binary.

**Tech Stack:** Go 1.26, Bubble Tea v2, Docker multi-stage builds, Alpine runtime.

---

## File Structure

- Create `Dockerfile`: build static Go binary and package it in a non-root Alpine runtime.
- Create `.dockerignore`: keep build context small and avoid local artifacts.
- Modify `README.md`: add Docker build/run examples and TUI notes.
- Modify `internal/tui/model_test.go`: extend interaction coverage for selection after cursor movement, filter escape, and visible select-all.
- Create `internal/tui/pty_test.go`: smoke test the built TUI through a pseudo-terminal using `script` when available.

## Task 1: Docker Support

- [x] Write `Dockerfile` with builder and runtime stages.
- [x] Write `.dockerignore`.
- [x] Build image with `docker build -t runny:test .`.
- [x] Smoke test auto mode with `docker run --rm -v "$PWD:/workspace" -w /workspace runny:test --auto -- true`.

## Task 2: TUI Interaction Tests

- [x] Add failing model tests for cursor movement plus selection, filter escape, and select-all visible targets.
- [x] Run `go test ./internal/tui` and confirm failure.
- [x] Implement minimal TUI model fixes.
- [x] Run `go test ./internal/tui` and confirm pass.

## Task 3: Real TUI Smoke Test

- [x] Add pseudo-tty test that builds the runny binary in a temp dir.
- [x] Run it through `script` with key input `down`, `space`, `q`.
- [x] Skip cleanly when `script` is unavailable.
- [x] Run `go test ./internal/tui`.

## Task 4: README and Verification

- [x] Add Docker section to `README.md`.
- [x] Run `gofmt`.
- [x] Run `go test ./...`.
- [x] Run `go vet ./...`.
- [x] Run `docker build -t runny:test .`.
- [x] Run Docker auto smoke test.
- [x] Commit changes.
