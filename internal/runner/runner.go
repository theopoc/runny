package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/termios"
	"github.com/creack/pty"
	"github.com/theopoc/runny/internal/core"
)

const terminalDrainTimeout = 250 * time.Millisecond

// Request describes one Target execution.
type Request struct {
	Command  string
	Target   core.Target
	OnOutput func([]byte)
}

// Outcome contains command execution facts only. Persistence cannot change it.
type Outcome struct {
	Status   core.Status
	ExitCode int
	Error    string
	Started  time.Time
	Ended    time.Time
}

// Execute runs one command in one Target PTY and streams normalized output.
func Execute(ctx context.Context, req Request) Outcome {
	started := time.Now()
	if ctx.Err() != nil {
		return Outcome{
			Status:  core.StatusCancelled,
			Error:   ctx.Err().Error(),
			Started: started,
			Ended:   started,
		}
	}
	if req.Command == "" {
		return Outcome{
			Status:  core.StatusFailed,
			Error:   "command is required",
			Started: started,
			Ended:   time.Now(),
		}
	}
	cmd := commandForTarget(req.Command, req.Target)
	cmd.Dir = req.Target.AbsPath
	var outputWriter io.Writer = io.Discard
	if req.OnOutput != nil {
		outputWriter = callbackWriter(req.OnOutput)
	}
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 24, Cols: 80})
	if err == nil {
		writeTerminalEOF(ptmx)
		readDone := make(chan struct{})
		go func() {
			if req.OnOutput == nil {
				_, _ = io.Copy(io.Discard, ptmx)
			} else {
				writer := &terminalOutputWriter{dst: outputWriter}
				_, _ = io.Copy(writer, ptmx)
				writer.Flush()
			}
			close(readDone)
		}()
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()
		select {
		case err = <-done:
			waitForTerminalRead(ptmx, readDone)
		case <-ctx.Done():
			_ = ptmx.Close()
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
				_ = cmd.Process.Kill()
			}
			err = <-done
			<-readDone
		}
		_ = ptmx.Close()
	}
	ended := time.Now()
	result := Outcome{Started: started, Ended: ended}
	if ctx.Err() != nil {
		result.Status = core.StatusCancelled
		result.Error = ctx.Err().Error()
		return result
	}
	if err == nil {
		result.Status = core.StatusSucceeded
		return result
	}
	result.Status = core.StatusFailed
	result.Error = err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	}
	return result
}

func waitForTerminalRead(terminal *os.File, readDone <-chan struct{}) {
	timer := time.NewTimer(terminalDrainTimeout)
	defer timer.Stop()
	select {
	case <-readDone:
	case <-timer.C:
		_ = terminal.Close()
		<-readDone
	}
}

func commandForTarget(command string, target core.Target) *exec.Cmd {
	shell := currentShell()
	command = disableJobControl(shell, command)
	if direnv, err := exec.LookPath("direnv"); err == nil {
		return exec.Command(direnv, "exec", target.AbsPath, shell, "-ic", command)
	}
	return exec.Command(shell, "-ic", command)
}

func currentShell() string {
	return resolveShell(os.Getenv, os.Getuid(), os.ReadFile)
}

func resolveShell(getenv func(string) string, uid int, readFile func(string) ([]byte, error)) string {
	if shell := getenv("SHELL"); shell != "" {
		return shell
	}
	passwd, err := readFile("/etc/passwd")
	if err == nil {
		uidText := strconv.Itoa(uid)
		for line := range strings.SplitSeq(string(passwd), "\n") {
			fields := strings.Split(line, ":")
			if len(fields) >= 7 && fields[2] == uidText && fields[6] != "" {
				return fields[6]
			}
		}
	}
	return "/bin/sh"
}

func disableJobControl(shell, command string) string {
	switch filepath.Base(shell) {
	case "sh", "bash", "dash", "ksh", "zsh":
		return "set +m\n" + command
	default:
		return command
	}
}

func writeTerminalEOF(terminal *os.File) {
	settings, err := termios.GetTermios(int(terminal.Fd()))
	if err != nil {
		_, _ = terminal.Write([]byte{4})
		return
	}
	echoEnabled := settings.Lflag&syscall.ECHO != 0
	_ = termios.SetTermios(
		int(terminal.Fd()),
		uint32(settings.Ispeed),
		uint32(settings.Ospeed),
		nil,
		nil,
		nil,
		nil,
		map[termios.L]bool{termios.ECHO: false},
	)
	_, _ = terminal.Write([]byte{4})
	_ = termios.SetTermios(
		int(terminal.Fd()),
		uint32(settings.Ispeed),
		uint32(settings.Ospeed),
		nil,
		nil,
		nil,
		nil,
		map[termios.L]bool{termios.ECHO: echoEnabled},
	)
}

type terminalOutputWriter struct {
	dst       io.Writer
	pendingCR bool
}

func (w *terminalOutputWriter) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p)+1)
	index := 0
	if w.pendingCR {
		if len(p) > 0 && p[0] == '\n' {
			out = append(out, '\n')
			index = 1
		} else {
			out = append(out, '\r')
		}
		w.pendingCR = false
	}
	for index < len(p) {
		if p[index] != '\r' {
			out = append(out, p[index])
			index++
			continue
		}
		if index+1 == len(p) {
			w.pendingCR = true
			break
		}
		if p[index+1] == '\n' {
			out = append(out, '\n')
			index += 2
			continue
		}
		out = append(out, '\r')
		index++
	}
	if len(out) == 0 {
		return len(p), nil
	}
	if _, err := w.dst.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *terminalOutputWriter) Flush() {
	if w.pendingCR {
		_, _ = w.dst.Write([]byte{'\r'})
		w.pendingCR = false
	}
}

type callbackWriter func([]byte)

func (w callbackWriter) Write(p []byte) (int, error) {
	w(p)
	return len(p), nil
}
