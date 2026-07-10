package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkipsHiddenAndSymlinkDirsByDefault(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "api"))
	mkdir(t, filepath.Join(root, ".git"))
	mkdir(t, filepath.Join(root, "real"))
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}

	targets, err := Discover(root, Options{Depth: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := rels(targets); len(got) != 2 || got[0] != "api" || got[1] != "real" {
		t.Fatalf("targets = %#v", got)
	}
}

func TestDiscoverStartsTargetsUnselected(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "api", "cmd"))

	targets, err := Discover(root, Options{Depth: 2, Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range targets {
		if target.Selected {
			t.Fatalf("target %s should start unselected", target.RelPath)
		}
	}
}

func TestDiscoverDepthAndTreeChildren(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "api", "cmd"))
	mkdir(t, filepath.Join(root, "api", "internal", "deep"))

	targets, err := Discover(root, Options{Depth: 2, Recursive: true})
	if err != nil {
		t.Fatal(err)
	}
	got := rels(targets)
	want := []string{"api", filepath.Join("api", "cmd"), filepath.Join("api", "internal")}
	if len(got) != len(want) {
		t.Fatalf("targets = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("target[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(targets[0].Children) != 2 {
		t.Fatalf("api children = %#v", targets[0].Children)
	}
}

func TestDiscoverIncludeExclude(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "api"))
	mkdir(t, filepath.Join(root, "web"))
	targets, err := Discover(root, Options{Depth: 1, Include: []string{"api"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := rels(targets); len(got) != 1 || got[0] != "api" {
		t.Fatalf("targets = %#v", got)
	}
}

func TestDiscoverPrunesExcludedDirectories(t *testing.T) {
	root := t.TempDir()
	mkdir(t, filepath.Join(root, "kept"))
	blocked := filepath.Join(root, "excluded", "blocked")
	mkdir(t, blocked)
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(blocked, 0o755); err != nil {
			t.Errorf("restore blocked directory permissions: %v", err)
		}
	})

	targets, err := Discover(root, Options{
		Recursive: true,
		Depth:     0,
		Exclude:   []string{"excluded"},
	})
	if err != nil {
		t.Fatalf("discover with excluded unreadable subtree: %v", err)
	}
	if got := rels(targets); len(got) != 1 || got[0] != "kept" {
		t.Fatalf("targets = %#v, want only kept", got)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func rels(targets []Target) []string {
	out := make([]string, len(targets))
	for i, target := range targets {
		out[i] = target.RelPath
	}
	return out
}
