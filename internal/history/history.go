package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/theopoc/runny/internal/core"
)

type CommandEntry struct {
	Command string    `json:"command"`
	Time    time.Time `json:"time"`
}

type RunEntry struct {
	Command   string        `json:"command"`
	Total     int           `json:"total"`
	Succeeded int           `json:"succeeded"`
	Failed    int           `json:"failed"`
	Cancelled int           `json:"cancelled"`
	Time      time.Time     `json:"time"`
	Started   time.Time     `json:"started,omitempty"`
	Ended     time.Time     `json:"ended,omitempty"`
	LogID     string        `json:"log_id,omitempty"`
	Targets   []TargetEntry `json:"targets,omitempty"`
}

type TargetEntry struct {
	Changes  core.ChangeSummary `json:"changes,omitzero"`
	ID       string             `json:"id"`
	RelPath  string             `json:"rel_path"`
	Status   core.Status        `json:"status"`
	ExitCode int                `json:"exit_code"`
	Error    string             `json:"error,omitempty"`
	Started  time.Time          `json:"started,omitempty"`
	Ended    time.Time          `json:"ended,omitempty"`
}

func AppendCommand(path string, entry CommandEntry) error {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	entries, err := ReadCommands(path)
	if err != nil {
		return err
	}
	unique := entries[:0]
	for _, existing := range entries {
		if existing.Command != entry.Command {
			unique = append(unique, existing)
		}
	}
	entries = append(unique, entry)
	if len(entries) > 50 {
		entries = entries[len(entries)-50:]
	}
	return writeJSONL(path, entries)
}

func ReadCommands(path string) ([]CommandEntry, error) {
	entries, err := readJSONL[CommandEntry](path)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(entries))
	unique := make([]CommandEntry, 0, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		if _, exists := seen[entries[i].Command]; exists {
			continue
		}
		seen[entries[i].Command] = struct{}{}
		unique = append(unique, entries[i])
	}
	for left, right := 0, len(unique)-1; left < right; left, right = left+1, right-1 {
		unique[left], unique[right] = unique[right], unique[left]
	}
	return unique, nil
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
	return writeJSONL(path, entries)
}

func writeJSONL[T any](path string, entries []T) error {
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
	// A single Run can contain hundreds of Target results, including summaries.
	scanner.Buffer(make([]byte, 4096), 16<<20)
	for scanner.Scan() {
		var entry T
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
