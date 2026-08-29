package logs

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrLogUnavailable = errors.New("logs unavailable")

const persistedReadLimit int64 = 1 << 20

type Options struct {
	Root     string
	Save     bool
	Disabled bool
}

type Store struct {
	root     *os.Root
	save     bool
	disabled bool
	mu       sync.Mutex
	bufs     map[string]*bytes.Buffer
}

func NewStore(opts Options) (*Store, error) {
	var root *os.Root
	if opts.Save && !opts.Disabled {
		if err := os.MkdirAll(opts.Root, 0o700); err != nil {
			return nil, err
		}
		var err error
		root, err = os.OpenRoot(opts.Root)
		if err != nil {
			return nil, err
		}
		if err := root.Chmod(".", 0o700); err != nil {
			return nil, errors.Join(err, root.Close())
		}
	}
	return &Store{root: root, save: opts.Save, disabled: opts.Disabled, bufs: map[string]*bytes.Buffer{}}, nil
}

func (s *Store) Append(targetID, text string) error {
	if s.disabled {
		return nil
	}
	path, err := targetLogPath(targetID)
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
		if s.root == nil {
			return os.ErrClosed
		}
		return appendToRoot(s.root, path, text)
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == nil {
		return nil
	}
	root := s.root
	s.root = nil
	return root.Close()
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

func targetLogPath(targetID string) (string, error) {
	path := filepath.FromSlash(targetID)
	if targetID == "" || path == "." || !filepath.IsLocal(path) {
		return "", fmt.Errorf("invalid target id %q", targetID)
	}
	return path + ".log", nil
}

func ReadPersisted(rootPath, runID, targetID string) (string, error) {
	if runID == "" || runID == "." || !filepath.IsLocal(runID) || filepath.Base(runID) != runID {
		return "", fmt.Errorf("%w: invalid run id %q", ErrLogUnavailable, runID)
	}
	targetPath, err := targetLogPath(targetID)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrLogUnavailable, err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrLogUnavailable, err)
	}
	defer root.Close()
	f, err := root.Open(filepath.Join(runID, targetPath))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrLogUnavailable, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	truncated := info.Size() > persistedReadLimit
	if truncated {
		if _, err := f.Seek(-persistedReadLimit, io.SeekEnd); err != nil {
			return "", err
		}
	}
	data, err := io.ReadAll(io.LimitReader(f, persistedReadLimit))
	if err != nil {
		return "", err
	}
	if truncated {
		return "[older output truncated]\n" + string(data), nil
	}
	return string(data), nil
}

func appendToRoot(root *os.Root, path, text string) error {
	parent := filepath.Dir(path)
	if err := root.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	if err := secureDirectories(root, parent); err != nil {
		return err
	}
	f, err := root.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		return errors.Join(err, f.Close())
	}
	_, writeErr := f.WriteString(text)
	return errors.Join(writeErr, f.Close())
}

func secureDirectories(root *os.Root, parent string) error {
	dirs := []string{"."}
	current := ""
	if parent != "." {
		for _, part := range strings.Split(parent, string(filepath.Separator)) {
			current = filepath.Join(current, part)
			dirs = append(dirs, current)
		}
	}
	for _, dir := range dirs {
		dirRoot, err := root.OpenRoot(dir)
		if err != nil {
			return err
		}
		if err := errors.Join(dirRoot.Chmod(".", 0o700), dirRoot.Close()); err != nil {
			return err
		}
	}
	return nil
}
