package logs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreCapturesMemoryAndPersistsWhenEnabled(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(Options{Root: root, Save: true})
	if err != nil {
		t.Fatal(err)
	}
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
	store, err := NewStore(Options{Disabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Append("api", "hello"); err != nil {
		t.Fatal(err)
	}
	if got := store.String("api"); got != "" {
		t.Fatalf("log = %q", got)
	}
}
