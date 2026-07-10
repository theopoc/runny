package logs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		if err := os.MkdirAll(opts.Root, 0o700); err != nil {
			return nil, err
		}
	}
	return &Store{root: opts.Root, save: opts.Save, disabled: opts.Disabled, bufs: map[string]*bytes.Buffer{}}, nil
}

func (s *Store) Append(targetID, text string) error {
	if s.disabled {
		return nil
	}
	path, err := targetLogPath(s.root, targetID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bufs[targetID] == nil {
		s.bufs[targetID] = &bytes.Buffer{}
	}
	s.bufs[targetID].WriteString(text)
	if s.save {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := f.WriteString(text)
		return errors.Join(writeErr, f.Close())
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

func targetLogPath(root, targetID string) (string, error) {
	path := filepath.FromSlash(targetID)
	if targetID == "" || path == "." || !filepath.IsLocal(path) {
		return "", fmt.Errorf("invalid target id %q", targetID)
	}
	return filepath.Join(root, path+".log"), nil
}
