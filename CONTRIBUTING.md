# Contributing to Runny

Thanks for contributing. Runny executes one shell command across selected child directories, so changes must preserve command safety, hidden-directory defaults, and stable keyboard behavior.

## Before opening an issue

Use an issue form. Redact tokens, credentials, private paths, and command output containing sensitive data. Report security vulnerabilities privately; do not create a public issue.

## Development

Requirements: Go version declared in `go.mod`, Docker for container checks, and a terminal supporting the TUI.

Run local validation before a pull request:

```bash
rtk go vet ./...
rtk go test ./...
rtk go test -race ./...
rtk go build ./cmd/runny
```

For TUI changes, update focused tests and golden fixtures under `internal/tui/testdata/`. Inspect the changed flow in a real terminal and include a capture when visual output changes.

## Pull requests

- Keep each pull request focused.
- Add tests for behavior changes.
- Explain flag, config, history, logging, and shortcut changes.
- Preserve SHA or digest pinning for GitHub Actions and Docker images.
- Do not commit `.runny/runs/`, credentials, generated binaries, or local editor files.

By contributing, you agree to follow the [Code of Conduct](CODE_OF_CONDUCT.md).
