package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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

	capture := filepath.Join(tmp, "typescript")
	cmd := scriptCommand(capture, bin)
	cmd.Dir = root
	cmd.Stdin = strings.NewReader("\x1b[B q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run TUI through pseudo-tty: %v\n%s", err, out)
	}
}

func scriptCommand(capture string, command string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("script", "-q", capture, command)
	}
	return exec.Command("script", "-q", "-c", command, capture)
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
