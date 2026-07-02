package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type CommandEntry struct {
	Command string    `json:"command"`
	Time    time.Time `json:"time"`
}

type RunEntry struct {
	Command   string    `json:"command"`
	Total     int       `json:"total"`
	Succeeded int       `json:"succeeded"`
	Failed    int       `json:"failed"`
	Cancelled int       `json:"cancelled"`
	Time      time.Time `json:"time"`
}

func AppendCommand(path string, entry CommandEntry) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	return appendRetained(path, entry, 50)
}

func ReadCommands(path string) ([]CommandEntry, error) {
	return readJSONL[CommandEntry](path)
}

func AppendRun(path string, entry RunEntry) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	return appendRetained(path, entry, 100)
}

func ReadRuns(path string) ([]RunEntry, error) {
	return readJSONL[RunEntry](path)
}

func appendRetained[T any](path string, entry T, limit int) error {
	entries, err := readJSONL[T](path)
	if err != nil {
		return err
	}
	entries = append(entries, entry)
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, item := range entries {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []T
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry T
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
