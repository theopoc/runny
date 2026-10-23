package history

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const FilterLimit = 50

// ReadFilters returns filters in most-recent-first order.
func ReadFilters(path string) ([]string, error) {
	return readJSONL[string](path)
}

// SaveFilters atomically replaces the filter history, keeping the newest exact
// occurrence of each filter. Values must be ordered most recent first.
func SaveFilters(path string, filters []string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".filters-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	enc := json.NewEncoder(f)
	seen := make(map[string]bool)
	for _, filter := range filters {
		if filter == "" || seen[filter] {
			continue
		}
		if err := enc.Encode(filter); err != nil {
			return err
		}
		seen[filter] = true
		if len(seen) == FilterLimit {
			break
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
