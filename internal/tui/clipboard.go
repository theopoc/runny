package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type clipboardStatus uint8

const (
	clipboardNone clipboardStatus = iota
	clipboardConfirmed
	clipboardSent
	clipboardFailed
	clipboardEmpty
)

type clipboardResultMsg struct {
	status clipboardStatus
	err    error
}

type clipboardOSCMsg struct {
	text string
}

type clipboardCopyFunc func(context.Context, string) tea.Msg

type clipboardRuntime struct {
	goos     string
	getenv   func(string) string
	lookPath func(string) (string, error)
	run      func(context.Context, string, []string, string) error
}

func defaultClipboardRuntime() clipboardRuntime {
	return clipboardRuntime{
		goos:     runtime.GOOS,
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		run: func(ctx context.Context, path string, args []string, input string) error {
			command := exec.CommandContext(ctx, path, args...)
			command.Stdin = bytes.NewBufferString(input)
			var stderr bytes.Buffer
			command.Stderr = &stderr
			if err := command.Run(); err != nil {
				if detail := stderr.String(); detail != "" {
					return fmt.Errorf("%s: %w", detail, err)
				}
				return err
			}
			return nil
		},
	}
}

func copyToSystemClipboard(ctx context.Context, text string, system clipboardRuntime) tea.Msg {
	if system.getenv("TMUX") != "" {
		path, err := system.lookPath("tmux")
		if err != nil {
			return clipboardResultMsg{status: clipboardFailed, err: err}
		}
		err = system.run(ctx, path, []string{"load-buffer", "-w", "-"}, text)
		if err != nil {
			return clipboardResultMsg{status: clipboardFailed, err: err}
		}
		return clipboardResultMsg{status: clipboardSent}
	}
	if system.getenv("SSH_CONNECTION") != "" || system.getenv("SSH_CLIENT") != "" || system.getenv("SSH_TTY") != "" {
		return clipboardOSCMsg{text: text}
	}

	for _, candidate := range nativeClipboardCandidates(system.goos, system.getenv("WAYLAND_DISPLAY") != "") {
		path, err := system.lookPath(candidate.name)
		if err != nil {
			continue
		}
		if err := system.run(ctx, path, candidate.args, text); err != nil {
			return clipboardResultMsg{status: clipboardFailed, err: err}
		}
		return clipboardResultMsg{status: clipboardConfirmed}
	}
	return clipboardOSCMsg{text: text}
}

type clipboardCandidate struct {
	name string
	args []string
}

func nativeClipboardCandidates(goos string, wayland bool) []clipboardCandidate {
	switch goos {
	case "darwin":
		return []clipboardCandidate{{name: "pbcopy"}}
	case "windows":
		return []clipboardCandidate{{name: "clip.exe"}}
	case "linux":
		candidates := make([]clipboardCandidate, 0, 3)
		if wayland {
			candidates = append(candidates, clipboardCandidate{name: "wl-copy"})
		}
		return append(candidates,
			clipboardCandidate{name: "xclip", args: []string{"-selection", "clipboard"}},
			clipboardCandidate{name: "xsel", args: []string{"--clipboard", "--input"}},
		)
	default:
		return nil
	}
}

type copyToast struct {
	status     clipboardStatus
	generation uint64
}

func (t copyToast) visible() bool {
	return t.status != clipboardNone
}

type dismissCopyToastMsg struct {
	generation uint64
}

const copyToastDuration = 2500 * time.Millisecond

func copyToastTimer(generation uint64) tea.Cmd {
	return tea.Tick(copyToastDuration, func(time.Time) tea.Msg {
		return dismissCopyToastMsg{generation: generation}
	})
}

func (m *Model) showCopyToast(status clipboardStatus) tea.Cmd {
	m.copyToast.generation++
	m.copyToast.status = status
	return copyToastTimer(m.copyToast.generation)
}

func (m Model) copyCurrentOutput() (tea.Model, tea.Cmd) {
	if m.Cursor < 0 || m.Cursor >= len(m.Targets) {
		return m, m.showCopyToast(clipboardEmpty)
	}
	targetID := m.Targets[m.Cursor].ID
	text := ""
	if m.outputSelection.active() && m.outputSelection.targetID == targetID {
		text = m.outputSelection.text()
		m.clearOutputSelection()
	} else {
		text = normalizeOutputText(m.Logs[targetID], m.liveLogTruncated[targetID])
	}
	if text == "" {
		return m, m.showCopyToast(clipboardEmpty)
	}
	copyClipboard := m.clipboardCopy
	ctx := m.lifecycleCtx
	return m, func() tea.Msg {
		return copyClipboard(ctx, text)
	}
}

func (m Model) renderCopyToast(panelWidth int) []string {
	if !m.copyToast.visible() {
		return nil
	}
	label, style := copyToastPresentation(m.copyToast.status, panelWidth < 50)
	toastWidth := min(max(5, ansi.StringWidth(label)+4), max(5, panelWidth-4))
	label = truncateVisible(label, max(1, toastWidth-4))
	return boxLinesWithTitle(toastWidth, 3, "", []string{style.Render(label)}, false, style, style)
}

func copyToastPresentation(status clipboardStatus, narrow bool) (string, lipgloss.Style) {
	switch status {
	case clipboardConfirmed:
		if narrow {
			return "✓ Copied", copyToastSuccessStyle
		}
		return "✓ Copied to clipboard", copyToastSuccessStyle
	case clipboardSent:
		if narrow {
			return "↗ Copy sent", copyToastSentStyle
		}
		return "↗ Copy request sent", copyToastSentStyle
	case clipboardEmpty:
		if narrow {
			return "! Nothing", copyToastErrorStyle
		}
		return "! Nothing to copy", copyToastErrorStyle
	default:
		return "! Copy failed", copyToastErrorStyle
	}
}

func placeBottomRightRows(background, overlay []string, right, bottom int) []string {
	if len(background) == 0 || len(overlay) == 0 {
		return background
	}
	width := ansi.StringWidth(background[0])
	overlayWidth := 0
	for _, row := range overlay {
		overlayWidth = max(overlayWidth, ansi.StringWidth(row))
	}
	left := max(0, width-right-overlayWidth)
	top := max(0, len(background)-bottom-len(overlay))
	result := append([]string(nil), background...)
	for i, overlayRow := range overlay {
		row := top + i
		if row >= len(result) {
			break
		}
		line := padRightANSI(result[row], width)
		result[row] = ansi.Cut(line, 0, left) + overlayRow + ansi.Cut(line, left+overlayWidth, width)
	}
	return result
}

func (m Model) copyToastContains(x, y int) bool {
	if !m.copyToast.visible() {
		return false
	}
	rect, ok := m.outputContentRect()
	if !ok {
		return false
	}
	toast := m.renderCopyToast(rect.width + 4)
	if len(toast) == 0 {
		return false
	}
	toastWidth := ansi.StringWidth(toast[0])
	toastX := rect.x + rect.width - toastWidth
	toastY := rect.y + rect.height - len(toast)
	return x >= toastX && x < toastX+toastWidth && y >= toastY && y < toastY+len(toast)
}
