package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theopoc/runny/internal/cli"
	"github.com/theopoc/runny/internal/config"
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

func TestFlagOverridesPreservesExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("serial: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := cli.Parse([]string{"--serial=false"})
	if err != nil {
		t.Fatal(err)
	}
	overrides := flagOverrides(parsed)
	if overrides.Serial == nil || *overrides.Serial {
		t.Fatalf("serial override = %#v, want non-nil false", overrides.Serial)
	}

	cfg, err := config.Load(config.LoadOptions{
		HomeDir: dir,
		WorkDir: dir,
		Config:  configPath,
		Flags:   overrides,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Serial {
		t.Fatal("serial = true, want explicit CLI false to override config file")
	}
}
