package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCapturesMemoryAndPersistsWhenEnabled(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, Options{Root: root, Save: true})
	if err := store.Append("api", "hello\n"); err != nil {
		t.Fatal(err)
	}
	if got := store.String("api"); got != "hello\n" {
		t.Fatalf("log = %q", got)
	}
	data, err := os.ReadFile(filepath.Join(root, "api.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("file log = %q", data)
	}
}

func TestStoreDisabledDropsLogs(t *testing.T) {
	store := newTestStore(t, Options{Disabled: true})
	if err := store.Append("api", "hello"); err != nil {
		t.Fatal(err)
	}
	if got := store.String("api"); got != "" {
		t.Fatalf("log = %q", got)
	}
}

func TestStoreNestedTargetPaths(t *testing.T) {
	root := t.TempDir()
	store := newTestStore(t, Options{Root: root, Save: true})
	if err := store.Append("api/v1", "nested\n"); err != nil {
		t.Fatal(err)
	}
	if err := store.Append("api_v1", "flat\n"); err != nil {
		t.Fatal(err)
	}

	assertFileContents(t, filepath.Join(root, "api", "v1.log"), "nested\n")
	assertFileContents(t, filepath.Join(root, "api_v1.log"), "flat\n")
}

func TestStoreRejectsUnsafeTargetIDs(t *testing.T) {
	tests := []struct {
		name     string
		targetID string
	}{
		{name: "empty", targetID: ""},
		{name: "current directory", targetID: "."},
		{name: "parent traversal", targetID: "../escape"},
		{name: "absolute", targetID: filepath.Join(string(os.PathSeparator), "absolute")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "run")
			store := newTestStore(t, Options{Root: root, Save: true})
			if err := store.Append(tt.targetID, "unsafe\n"); err == nil {
				t.Fatalf("Append(%q) error = nil", tt.targetID)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Fatalf("root entries = %#v, want none", entries)
			}
			if _, err := os.Stat(filepath.Join(base, "escape.log")); !os.IsNotExist(err) {
				t.Fatalf("outside escape file exists or stat failed unexpectedly: %v", err)
			}
		})
	}
}

func TestStoreUsesPrivateModes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")
	logPath := filepath.Join(root, "team", "service", "api.log")
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, filepath.Join(root, "team"), filepath.Dir(logPath)} {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(logPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(logPath, 0o644); err != nil {
		t.Fatal(err)
	}

	store := newTestStore(t, Options{Root: root, Save: true})
	if err := store.Append("team/service/api", "private\n"); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{root, filepath.Join(root, "team"), filepath.Join(root, "team", "service")} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Fatalf("directory %q mode = %04o, want 0700", dir, got)
		}
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("log mode = %04o, want 0600", got)
	}
}

func TestStoreRejectsParentSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "run")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, Options{Root: root, Save: true})
	if err := os.Symlink(outside, filepath.Join(root, "api")); err != nil {
		t.Fatal(err)
	}

	if err := store.Append("api/v1", "unsafe\n"); err == nil {
		t.Fatal("Append() error = nil, want parent symlink rejection")
	}
	if _, err := os.Stat(filepath.Join(outside, "v1.log")); !os.IsNotExist(err) {
		t.Fatalf("outside log exists or stat failed unexpectedly: %v", err)
	}
}

func TestStoreRejectsFinalLogSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "run")
	outside := filepath.Join(base, "outside.log")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, Options{Root: root, Save: true})
	if err := os.Symlink(outside, filepath.Join(root, "api.log")); err != nil {
		t.Fatal(err)
	}

	if err := store.Append("api", "unsafe\n"); err == nil {
		t.Fatal("Append() error = nil, want final symlink rejection")
	}
	assertFileContents(t, outside, "original\n")
}

func TestStoreRootReplacementCannotEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "run")
	originalRoot := filepath.Join(base, "original-run")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, Options{Root: root, Save: true})
	if err := os.Rename(root, originalRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Fatal(err)
	}

	if err := store.Append("api", "safe\n"); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, filepath.Join(originalRoot, "api.log"), "safe\n")
	if _, err := os.Stat(filepath.Join(outside, "api.log")); !os.IsNotExist(err) {
		t.Fatalf("outside log exists or stat failed unexpectedly: %v", err)
	}
}

func TestStoreCloseIsSafeAndIdempotent(t *testing.T) {
	store := newTestStore(t, Options{Root: filepath.Join(t.TempDir(), "run"), Save: true})
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := store.Append("api", "closed\n"); err == nil {
		t.Fatal("Append() error = nil after Close()")
	}
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != want {
		t.Fatalf("file %q = %q, want %q", path, got, want)
	}
}

func newTestStore(t *testing.T, opts Options) *Store {
	t.Helper()
	store, err := NewStore(opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	return store
}
