package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCopyToSystemClipboardChoosesPortableTransport(t *testing.T) {
	tests := []struct {
		name       string
		goos       string
		env        map[string]string
		available  map[string]string
		wantPath   string
		wantArgs   []string
		wantStatus clipboardStatus
		wantOSC    bool
	}{
		{name: "tmux before ssh", goos: "linux", env: map[string]string{"TMUX": "/tmp/tmux", "SSH_TTY": "/dev/pts/1"}, available: map[string]string{"tmux": "/bin/tmux"}, wantPath: "/bin/tmux", wantArgs: []string{"load-buffer", "-w", "-"}, wantStatus: clipboardSent},
		{name: "remote uses osc52", goos: "linux", env: map[string]string{"SSH_CONNECTION": "remote"}, wantOSC: true},
		{name: "macos native", goos: "darwin", available: map[string]string{"pbcopy": "/usr/bin/pbcopy"}, wantPath: "/usr/bin/pbcopy", wantStatus: clipboardConfirmed},
		{name: "wayland native", goos: "linux", env: map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, available: map[string]string{"wl-copy": "/usr/bin/wl-copy"}, wantPath: "/usr/bin/wl-copy", wantStatus: clipboardConfirmed},
		{name: "xclip fallback", goos: "linux", available: map[string]string{"xclip": "/usr/bin/xclip"}, wantPath: "/usr/bin/xclip", wantArgs: []string{"-selection", "clipboard"}, wantStatus: clipboardConfirmed},
		{name: "xsel fallback", goos: "linux", available: map[string]string{"xsel": "/usr/bin/xsel"}, wantPath: "/usr/bin/xsel", wantArgs: []string{"--clipboard", "--input"}, wantStatus: clipboardConfirmed},
		{name: "windows native", goos: "windows", available: map[string]string{"clip.exe": `C:\\Windows\\clip.exe`}, wantPath: `C:\\Windows\\clip.exe`, wantStatus: clipboardConfirmed},
		{name: "no local tool uses osc52", goos: "linux", wantOSC: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotInput string
			var gotArgs []string
			runtime := clipboardRuntime{
				goos:   tt.goos,
				getenv: func(key string) string { return tt.env[key] },
				lookPath: func(name string) (string, error) {
					if path := tt.available[name]; path != "" {
						return path, nil
					}
					return "", errors.New("not found")
				},
				run: func(_ context.Context, path string, args []string, input string) error {
					gotPath, gotArgs, gotInput = path, args, input
					return nil
				},
			}

			msg := copyToSystemClipboard(context.Background(), "payload", runtime)
			if tt.wantOSC {
				osc, ok := msg.(clipboardOSCMsg)
				if !ok || osc.text != "payload" {
					t.Fatalf("message = %#v, want OSC payload", msg)
				}
				return
			}
			result, ok := msg.(clipboardResultMsg)
			if !ok || result.status != tt.wantStatus || result.err != nil {
				t.Fatalf("message = %#v, want status %v", msg, tt.wantStatus)
			}
			if gotPath != tt.wantPath || !reflect.DeepEqual(gotArgs, tt.wantArgs) || gotInput != "payload" {
				t.Fatalf("run = %q %#v %q, want %q %#v payload", gotPath, gotArgs, gotInput, tt.wantPath, tt.wantArgs)
			}
		})
	}
}

func TestCopyToSystemClipboardKeepsLargePayloadIntact(t *testing.T) {
	payload := strings.Repeat("x", 4<<20)
	got := ""
	runtime := clipboardRuntime{
		goos:     "darwin",
		getenv:   func(string) string { return "" },
		lookPath: func(string) (string, error) { return "/usr/bin/pbcopy", nil },
		run: func(_ context.Context, _ string, _ []string, input string) error {
			got = input
			return nil
		},
	}
	result := copyToSystemClipboard(context.Background(), payload, runtime).(clipboardResultMsg)
	if result.status != clipboardConfirmed || got != payload {
		t.Fatalf("large payload changed: status=%v bytes=%d, want=%d", result.status, len(got), len(payload))
	}
}

func TestCopyToSystemClipboardReportsConfirmedFailure(t *testing.T) {
	wantErr := errors.New("clipboard unavailable")
	runtime := clipboardRuntime{
		goos:     "darwin",
		getenv:   func(string) string { return "" },
		lookPath: func(string) (string, error) { return "/usr/bin/pbcopy", nil },
		run:      func(context.Context, string, []string, string) error { return wantErr },
	}
	result := copyToSystemClipboard(context.Background(), "payload", runtime).(clipboardResultMsg)
	if result.status != clipboardFailed || !errors.Is(result.err, wantErr) {
		t.Fatalf("result = %#v", result)
	}
}
