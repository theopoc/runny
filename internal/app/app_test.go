package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	code := Run(Options{Args: []string{"--version"}, Stdout: &out})
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if out.String() == "" {
		t.Fatal("expected version output")
	}
}

func TestRunHelpDoesNotAdvertiseAuto(t *testing.T) {
	var out bytes.Buffer
	code := Run(Options{Args: []string{"--help"}, Stdout: &out, Stderr: &out})
	if code != 0 {
		t.Fatalf("code = %d output=%s", code, out.String())
	}
	if strings.Contains(out.String(), "--auto") {
		t.Fatalf("help should not contain --auto:\n%s", out.String())
	}
}

func TestRunRejectsAutoMode(t *testing.T) {
	var out bytes.Buffer
	code := Run(Options{Args: []string{"--auto"}, Stdout: &out, Stderr: &out})
	if code != 2 {
		t.Fatalf("code = %d output=%s", code, out.String())
	}
}
