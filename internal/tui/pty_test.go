package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunnyTUISmokeWithPseudoTTY(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script is not available on windows")
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script is not available")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not available in PATH")
	}

	tmp := t.TempDir()
	bin := filepath.Join(tmp, "runny")
	build := exec.Command("go", "build", "-o", bin, "./cmd/runny")
	build.Dir = repoRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build runny: %v\n%s", err, out)
	}

	root := filepath.Join(tmp, "workspace")
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(tmp, "runny-wrapper.sh")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\n\""+bin+"\" -- true &\npid=$!\nsleep 0.5\nkill \"$pid\" 2>/dev/null || true\nwait \"$pid\" 2>/dev/null || true\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	capture := filepath.Join(tmp, "typescript")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := scriptCommand(ctx, capture, wrapper)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil && ctx.Err() == nil {
		t.Fatalf("run TUI through pseudo-tty: %v\n%s", err, out)
	}
	captured, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(captured), "\x1b[?1049h") {
		t.Fatalf("TUI should enter alternate screen; capture:\n%s", captured)
	}
}

func scriptCommand(ctx context.Context, capture string, command string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.CommandContext(ctx, "script", "-q", capture, command)
	}
	return exec.CommandContext(ctx, "script", "-q", "-c", command, capture)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
