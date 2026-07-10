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

func TestResolveDiscoveryOptionsHonorsRecursiveDepthPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		configYAML    string
		args          []string
		wantRecursive bool
		wantDepth     int
	}{
		{
			name:          "explicit false forces direct children",
			configYAML:    "recursive: true\ndepth: 3\n",
			args:          []string{"--recursive=false"},
			wantRecursive: false,
			wantDepth:     1,
		},
		{
			name:          "explicit true uses unlimited depth",
			configYAML:    "recursive: false\ndepth: 3\n",
			args:          []string{"--recursive=true"},
			wantRecursive: true,
			wantDepth:     0,
		},
		{
			name:          "explicit depth wins over recursive false",
			configYAML:    "recursive: true\ndepth: 3\n",
			args:          []string{"--recursive=false", "--depth", "2"},
			wantRecursive: true,
			wantDepth:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0o644); err != nil {
				t.Fatal(err)
			}

			parsed, err := cli.Parse(tt.args)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(config.LoadOptions{
				HomeDir: dir,
				WorkDir: dir,
				Config:  configPath,
				Flags:   flagOverrides(parsed),
			})
			if err != nil {
				t.Fatal(err)
			}

			got := resolveDiscoveryOptions(cfg, parsed)
			if got.Recursive != tt.wantRecursive || got.Depth != tt.wantDepth {
				t.Fatalf("discovery options = {Recursive:%t Depth:%d}, want {Recursive:%t Depth:%d}",
					got.Recursive, got.Depth, tt.wantRecursive, tt.wantDepth)
			}
		})
	}
}
