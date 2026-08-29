package logs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPersistedReturnsTargetLog(t *testing.T) {
	root := t.TempDir()
	runID := "20260815T080000.000000000Z"
	store := newTestStore(t, Options{Root: filepath.Join(root, runID), Save: true})
	if err := store.Append("services/api", "failed output\n"); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPersisted(root, runID, "services/api")
	if err != nil {
		t.Fatal(err)
	}
	if got != "failed output\n" {
		t.Fatalf("log = %q", got)
	}
}

func TestReadPersistedReturnsBoundedTail(t *testing.T) {
	root := t.TempDir()
	runID := "run"
	runRoot := filepath.Join(root, runID)
	store := newTestStore(t, Options{Root: runRoot, Save: true})
	content := strings.Repeat("x", int(persistedReadLimit)+32) + "tail\n"
	if err := store.Append("api", content); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPersisted(root, runID, "api")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "[older output truncated]\n") || !strings.HasSuffix(got, "tail\n") {
		t.Fatalf("bounded log markers missing: prefix=%q suffix=%q", got[:min(32, len(got))], got[max(0, len(got)-16):])
	}
	if len(got) > int(persistedReadLimit)+len("[older output truncated]\n") {
		t.Fatalf("bounded log length = %d", len(got))
	}
}

func TestReadPersistedRejectsUnsafeIdentifiers(t *testing.T) {
	tests := []struct {
		name     string
		runID    string
		targetID string
	}{
		{name: "empty run", targetID: "api"},
		{name: "nested run", runID: "nested/run", targetID: "api"},
		{name: "parent run", runID: "../run", targetID: "api"},
		{name: "unsafe target", runID: "run", targetID: "../api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ReadPersisted(t.TempDir(), tt.runID, tt.targetID); !errors.Is(err, ErrLogUnavailable) {
				t.Fatalf("ReadPersisted() error = %v, want ErrLogUnavailable", err)
			}
		})
	}
}

func TestReadPersistedMissingLogIsUnavailable(t *testing.T) {
	if _, err := ReadPersisted(t.TempDir(), "run", "api"); !errors.Is(err, ErrLogUnavailable) {
		t.Fatalf("ReadPersisted() error = %v, want ErrLogUnavailable", err)
	}
}

func TestReadPersistedRejectsRunSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "runs")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "api.log"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "run")); err != nil {
		t.Fatal(err)
	}

	if _, err := ReadPersisted(root, "run", "api"); !errors.Is(err, ErrLogUnavailable) {
		t.Fatalf("ReadPersisted() error = %v, want ErrLogUnavailable", err)
	}
}

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
