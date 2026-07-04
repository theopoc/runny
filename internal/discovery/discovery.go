package discovery

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/theopoc/runny/internal/core"
)

type Target = core.Target

type Options struct {
	Recursive     bool
	Depth         int
	IncludeHidden bool
	Include       []string
	Exclude       []string
}

func Discover(root string, opts Options) ([]Target, error) {
	if opts.Depth == 0 && !opts.Recursive {
		opts.Depth = 1
	}
	targets := []Target{}
	byID := map[string]int{}
	err := walk(root, root, "", 0, opts, &targets, byID)
	if err != nil {
		return nil, err
	}
	for i := range targets {
		sort.Strings(targets[i].Children)
	}
	return targets, nil
}

func walk(root, dir, parentID string, depth int, opts Options, targets *[]Target, byID map[string]int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		hidden := strings.HasPrefix(name, ".")
		if hidden && !opts.IncludeHidden {
			continue
		}
		abs := filepath.Join(dir, name)
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			return err
		}
		nextDepth := depth + 1
		include := matchesInclude(rel, opts.Include) && !matches(rel, opts.Exclude)
		id := filepath.ToSlash(rel)
		if include {
			target := Target{
				ID:       id,
				RelPath:  rel,
				AbsPath:  abs,
				Name:     name,
				Depth:    nextDepth,
				ParentID: parentID,
				Selected: false,
				Folded:   false,
				Hidden:   hidden,
			}
			byID[id] = len(*targets)
			*targets = append(*targets, target)
			if parentID != "" {
				if parentIdx, ok := byID[parentID]; ok {
					(*targets)[parentIdx].Children = append((*targets)[parentIdx].Children, id)
				}
			}
		}
		if opts.Depth == 0 || nextDepth < opts.Depth {
			childParent := parentID
			if include {
				childParent = id
			}
			if err := walk(root, abs, childParent, nextDepth, opts, targets, byID); err != nil {
				return err
			}
		}
	}
	return nil
}

func matchesInclude(path string, include []string) bool {
	if len(include) == 0 {
		return true
	}
	return matches(path, include)
}

func matches(path string, patterns []string) bool {
	slashPath := filepath.ToSlash(path)
	for _, pattern := range patterns {
		if pattern == slashPath {
			return true
		}
		ok, _ := filepath.Match(pattern, slashPath)
		if ok {
			return true
		}
		if strings.Contains(slashPath, pattern) {
			return true
		}
	}
	return false
}
