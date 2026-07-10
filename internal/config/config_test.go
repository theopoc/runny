package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMergesHomeLocalExplicitAndFlags(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	root := filepath.Join(dir, "repo")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(home, ".runny.yaml"), "workers: 2\ninclude_hidden: true\n")
	write(t, filepath.Join(root, ".runny.yaml"), "serial: true\ndepth: 3\n")
	explicit := filepath.Join(dir, "custom.yaml")
	write(t, explicit, "workers: 5\nserial: false\n")

	cfg, err := Load(LoadOptions{
		HomeDir: home,
		WorkDir: root,
		Config:  explicit,
		Flags: FlagOverrides{
			Depth: intPtr(4),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workers != 5 || cfg.Depth != 4 || !cfg.IncludeHidden || cfg.Serial {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestDefaultsUseDepthThree(t *testing.T) {
	cfg := Defaults()
	if cfg.Depth != 3 {
		t.Fatalf("depth = %d, want 3", cfg.Depth)
	}
}

func TestValidateRejectsMutuallyExclusiveOptions(t *testing.T) {
	cfg := Defaults()
	cfg.Include = []string{"api"}
	cfg.Exclude = []string{"web"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected include/exclude validation error")
	}
}

func TestStrictYamlRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".runny.yaml"), "unknown: true\n")
	_, err := Load(LoadOptions{HomeDir: dir, WorkDir: dir})
	if err == nil {
		t.Fatal("expected strict yaml error")
	}
}

func TestStrictYamlRejectsAutoField(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".runny.yaml"), "auto: true\n")
	_, err := Load(LoadOptions{HomeDir: dir, WorkDir: dir})
	if err == nil {
		t.Fatal("expected auto field to be rejected")
	}
}

func TestLoadRejectsMissingExplicitConfig(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.yaml")

	if _, err := Load(LoadOptions{HomeDir: dir, WorkDir: dir}); err != nil {
		t.Fatalf("missing implicit config files should be optional: %v", err)
	}

	_, err := Load(LoadOptions{HomeDir: dir, WorkDir: dir, Config: missing})
	if err == nil {
		t.Fatal("expected missing explicit config error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error = %q, want explicit path %q", err, missing)
	}
}

func write(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func intPtr(v int) *int {
	return &v
}
