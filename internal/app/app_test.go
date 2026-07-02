package app

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestRunAutoMode(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	code := Run(Options{Args: []string{"--auto", "--", "true"}, WorkDir: root, HomeDir: root, Stdout: &out, Stderr: &out})
	if code != 0 {
		t.Fatalf("code = %d output=%s", code, out.String())
	}
}
