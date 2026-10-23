package history

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

func TestFilterHistoryRoundTripKeepsExactDistinctValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".runny", "filters.jsonl")
	if entries, err := ReadFilters(path); err != nil || len(entries) != 0 {
		t.Fatalf("missing history = %q, %v", entries, err)
	}
	want := []string{"api", "API", " api ", "re:^services/(api|web)$", "'exact path", "é👩‍💻\nquoted\""}
	input := append([]string{"api", "", "API", "api"}, want[2:]...)
	if err := SaveFilters(path, input); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFilters(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("history = %q, %v; want %q", got, err, want)
	}
}

func TestFilterHistoryRetentionCountsUniqueFilters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.jsonl")
	var filters []string
	for i := range FilterLimit + 5 {
		filters = append(filters, strconv.Itoa(i), strconv.Itoa(i))
	}
	if err := SaveFilters(path, filters); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFilters(path)
	if err != nil || len(got) != FilterLimit || got[0] != "0" || got[FilterLimit-1] != "49" {
		t.Fatalf("retained history = %q, %v", got, err)
	}
	if err := SaveFilters(path, []string{"new"}); err != nil {
		t.Fatal(err)
	}
	got, err = ReadFilters(path)
	if err != nil || len(got) != 1 || got[0] != "new" {
		t.Fatalf("replaced history = %q, %v", got, err)
	}
}

func TestFilterHistoryReportsReadAndWriteErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "filters.jsonl")
	if err := os.WriteFile(path, []byte("broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFilters(path); err == nil {
		t.Fatal("corrupt history should report an error")
	}
	if err := SaveFilters(filepath.Join(path, "child"), []string{"api"}); err == nil {
		t.Fatal("history with a file as parent should report an error")
	}
	if err := SaveFilters(root, []string{"api"}); err == nil {
		t.Fatal("history with a directory as destination should report an error")
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(root), ".filters-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files after failed rename = %q, %v", leftovers, err)
	}
}
