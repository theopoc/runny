package logs

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Options struct {
	Root     string
	Save     bool
	Disabled bool
}

type Store struct {
	root     string
	save     bool
	disabled bool
	mu       sync.Mutex
	bufs     map[string]*bytes.Buffer
}

func NewStore(opts Options) (*Store, error) {
	if opts.Save && !opts.Disabled {
		if err := os.MkdirAll(opts.Root, 0o755); err != nil {
			return nil, err
		}
	}
	return &Store{root: opts.Root, save: opts.Save, disabled: opts.Disabled, bufs: map[string]*bytes.Buffer{}}, nil
}

func (s *Store) Append(targetID, text string) error {
	if s.disabled {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bufs[targetID] == nil {
		s.bufs[targetID] = &bytes.Buffer{}
	}
	s.bufs[targetID].WriteString(text)
	if s.save {
		path := filepath.Join(s.root, sanitize(targetID)+".log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.WriteString(text)
		return err
	}
	return nil
}

func (s *Store) String(targetID string) string {
	if s.disabled {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bufs[targetID] == nil {
		return ""
	}
	return s.bufs[targetID].String()
}

func sanitize(id string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return replacer.Replace(id)
}
