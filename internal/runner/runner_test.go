package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/theopoc/runny/internal/core"
)

func executeCaptured(ctx context.Context, command string, target core.Target) (Outcome, string) {
	var output strings.Builder
	outcome := Execute(ctx, Request{
		Command: command,
		Target:  target,
		OnOutput: func(chunk []byte) {
			output.Write(chunk)
		},
	})
	return outcome, output.String()
}

func TestExecuteRunsCommandInTargetDirectory(t *testing.T) {
	target := core.Target{ID: "api", RelPath: "api", AbsPath: t.TempDir()}
	outcome, output := executeCaptured(context.Background(), "pwd", target)
	if outcome.Status != core.StatusSucceeded {
		t.Fatalf("outcome = %#v", outcome)
	}
	if strings.TrimSpace(output) != target.AbsPath {
		t.Fatalf("output = %q, want %q", output, target.AbsPath)
	}
}

func TestExecuteUsesUserInteractiveShell(t *testing.T) {
	shell := filepath.Join(t.TempDir(), "user-shell")
	shellScript := "#!/bin/sh\n[ -t 0 ] && [ -t 1 ] && [ -t 2 ] || exit 64\nprintf 'shell-args=%s|%s\\n' \"$1\" \"$2\"\n"
	if err := os.WriteFile(shell, []byte(shellScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	t.Setenv("PATH", t.TempDir())
	target := core.Target{ID: "api", AbsPath: t.TempDir()}

	outcome, output := executeCaptured(context.Background(), "printf ignored", target)
	if outcome.Status != core.StatusSucceeded {
		t.Fatalf("outcome = %#v", outcome)
	}
	if output != "shell-args=-ic|printf ignored\n" {
		t.Fatalf("output = %q, want user shell invoked interactively", output)
	}
}

func TestResolveShell(t *testing.T) {
	tests := []struct {
		name    string
		shell   string
		uid     int
		passwd  string
		readErr error
		want    string
	}{
		{name: "environment wins", shell: "/bin/bash", uid: 1001, passwd: "runny:x:1001:1001::/home/runny:/bin/zsh\n", want: "/bin/bash"},
		{name: "login shell from passwd", uid: 1001, passwd: "root:x:0:0:root:/root:/bin/sh\nrunny:x:1001:1001::/home/runny:/bin/zsh\n", want: "/bin/zsh"},
		{name: "missing user falls back", uid: 1001, passwd: "root:x:0:0:root:/root:/bin/sh\n", want: "/bin/sh"},
		{name: "unreadable passwd falls back", uid: 1001, readErr: errors.New("read failed"), want: "/bin/sh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveShell(
				func(string) string { return tt.shell },
				tt.uid,
				func(string) ([]byte, error) { return []byte(tt.passwd), tt.readErr },
			)
			if got != tt.want {
				t.Fatalf("resolveShell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExecuteLoadsZshAlias(t *testing.T) {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		t.Skip("zsh is not available")
	}
	zdotdir := t.TempDir()
	zshrc := "alias runny_alias_probe='printf \"RUNNY_ZSH_ALIAS=loaded\\\\n\"'\n"
	if err := os.WriteFile(filepath.Join(zdotdir, ".zshrc"), []byte(zshrc), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", zsh)
	t.Setenv("ZDOTDIR", zdotdir)
	target := core.Target{ID: "api", AbsPath: t.TempDir()}

	outcome, output := executeCaptured(context.Background(), "runny_alias_probe", target)
	if outcome.Status != core.StatusSucceeded {
		t.Fatalf("outcome = %#v", outcome)
	}
	if output != "RUNNY_ZSH_ALIAS=loaded\n" {
		t.Fatalf("output = %q, want alias from .zshrc", output)
	}
}

func TestExecuteLoadsTargetDirenvEnvironment(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	shell := filepath.Join(binDir, "user-shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\n[ \"$1\" = -ic ] || exit 64\nexec /bin/sh -c \"$2\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	direnv := filepath.Join(binDir, "direnv")
	if err := os.WriteFile(direnv, []byte("#!/bin/sh\n[ \"$1\" = exec ] || exit 64\ntarget=$2\nshift 2\n. \"$target/.envrc\"\nexec \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHELL", shell)
	t.Setenv("PATH", binDir)
	targetDir := filepath.Join(root, "project")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, ".envrc"), []byte("export RUNNY_DIRENV_SENTINEL=loaded\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := core.Target{ID: "api", AbsPath: targetDir}

	outcome, output := executeCaptured(context.Background(), `printf 'RUNNY_DIRENV_SENTINEL=%s\n' "$RUNNY_DIRENV_SENTINEL"`, target)
	if outcome.Status != core.StatusSucceeded {
		t.Fatalf("outcome = %#v", outcome)
	}
	if output != "RUNNY_DIRENV_SENTINEL=loaded\n" {
		t.Fatalf("output = %q, want target direnv environment", output)
	}
}

func TestTerminalOutputWriterNormalizesSplitCRLF(t *testing.T) {
	var output strings.Builder
	writer := &terminalOutputWriter{dst: &output}
	for _, chunk := range []string{"first\r", "\nsecond\r", "third"} {
		if _, err := writer.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	writer.Flush()
	if output.String() != "first\nsecond\rthird" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestExecuteMarksFailure(t *testing.T) {
	target := core.Target{ID: "api", AbsPath: t.TempDir()}
	outcome, _ := executeCaptured(context.Background(), "exit 7", target)
	if outcome.Status != core.StatusFailed || outcome.ExitCode != 7 {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func TestExecuteStreamsStdoutAndStderrBeforeCommandFinishes(t *testing.T) {
	target := core.Target{ID: "api", AbsPath: t.TempDir()}
	chunks := make(chan string, 4)
	done := make(chan Outcome, 1)
	go func() {
		done <- Execute(context.Background(), Request{
			Command: "printf 'stdout-before-finish\n'; sleep 1; printf 'stderr-before-finish\n' >&2",
			Target:  target,
			OnOutput: func(chunk []byte) {
				chunks <- string(chunk)
			},
		})
	}()

	var streamed strings.Builder
	deadline := time.After(500 * time.Millisecond)
	for !strings.Contains(streamed.String(), "stdout-before-finish\n") {
		select {
		case chunk := <-chunks:
			streamed.WriteString(chunk)
		case <-done:
			t.Fatal("command finished before first output")
		case <-deadline:
			t.Fatalf("first output missing: %q", streamed.String())
		}
	}
	select {
	case outcome := <-done:
		t.Fatalf("command finished too early: %#v", outcome)
	case <-time.After(100 * time.Millisecond):
	}
	for !strings.Contains(streamed.String(), "stderr-before-finish\n") {
		select {
		case chunk := <-chunks:
			streamed.WriteString(chunk)
		case <-time.After(2 * time.Second):
			t.Fatalf("stderr missing: %q", streamed.String())
		}
	}
	select {
	case outcome := <-done:
		if outcome.Status != core.StatusSucceeded {
			t.Fatalf("outcome = %#v", outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command did not finish")
	}
}

func TestExecuteWithoutOutputCallbackDoesNotCapture(t *testing.T) {
	target := core.Target{ID: "api", AbsPath: t.TempDir()}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	outcome := Execute(context.Background(), Request{
		Command: "dd if=/dev/zero bs=1048576 count=32 2>/dev/null",
		Target:  target,
	})
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if outcome.Status != core.StatusSucceeded {
		t.Fatalf("outcome = %#v", outcome)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Fatalf("allocated %d bytes, want no output-sized capture", allocated)
	}
}

func TestExecuteCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	target := core.Target{ID: "api", AbsPath: t.TempDir()}
	outcome := Execute(ctx, Request{Command: "sleep 1", Target: target})
	if outcome.Status != core.StatusCancelled {
		t.Fatalf("outcome = %#v", outcome)
	}
}

func TestExecuteCancellationKillsChildProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are unix-specific")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	target := core.Target{ID: "api", AbsPath: dir}
	done := make(chan Outcome, 1)
	go func() {
		done <- Execute(ctx, Request{
			Command: "sleep 20 & echo $! > child.pid; wait",
			Target:  target,
		})
	}()
	var pid int
	for range 50 {
		data, err := os.ReadFile(pidFile)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		t.Fatal("child pid was not written")
	}
	cancel()
	select {
	case outcome := <-done:
		if outcome.Status != core.StatusCancelled {
			t.Fatalf("outcome = %#v", outcome)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("execution did not cancel")
	}
	time.Sleep(100 * time.Millisecond)
	if processAlive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Fatalf("child process %d still alive", pid)
	}
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}
