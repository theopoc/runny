package releasenotes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var trailingReferences = regexp.MustCompile(`( \(\[[^]]+\]\([^)]*\)\))+$`)

func TestChangelogHasNoDuplicateReleaseNotes(t *testing.T) {
	changelogPath := filepath.Join("..", "..", "CHANGELOG.md")
	contents, err := os.ReadFile(changelogPath)
	if err != nil {
		t.Fatalf("read changelog: %v", err)
	}

	duplicates := duplicateReleaseNotes(string(contents))
	if len(duplicates) != 0 {
		t.Fatalf("duplicate release notes:\n%s", strings.Join(duplicates, "\n"))
	}
}

func duplicateReleaseNotes(changelog string) []string {
	var duplicates []string
	currentRelease := ""
	seen := map[string]map[string]int{}

	for line := range strings.SplitSeq(changelog, "\n") {
		if strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "### ") {
			currentRelease = releaseName(line)
			seen[currentRelease] = map[string]int{}
			continue
		}
		if currentRelease == "" || (!strings.HasPrefix(line, "* ") && !strings.HasPrefix(line, "- ")) {
			continue
		}

		label := strings.TrimSpace(line[2:])
		label = trailingReferences.ReplaceAllString(label, "")
		seen[currentRelease][label]++
		if seen[currentRelease][label] == 2 {
			duplicates = append(duplicates, currentRelease+": "+label)
		}
	}

	return duplicates
}

func releaseName(heading string) string {
	name := strings.TrimPrefix(heading, "## ")
	if strings.HasPrefix(name, "[") {
		if end := strings.IndexByte(name, ']'); end > 1 {
			return name[1:end]
		}
	}
	if end := strings.IndexByte(name, ' '); end > 0 {
		return name[:end]
	}
	return name
}
