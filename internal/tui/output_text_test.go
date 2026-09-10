package tui

import (
	"strings"
	"testing"

	runpkg "github.com/theopoc/runny/internal/run"
)

func TestNormalizeOutputText(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		truncated bool
		want      string
	}{
		{name: "ansi csi and osc", input: "\x1b[31mred\x1b[0m \x1b]8;;https://example.test\x1b\\link\x1b]8;;\x1b\\\n", want: "red link"},
		{name: "line endings and controls", input: "one\rprogress\r\ntwo\x00\x7f\u0085\tend\n\n", want: "one\nprogress\ntwo\tend"},
		{name: "invalid utf8", input: string([]byte{'a', 0xff, 'b'}), want: "a�b"},
		{name: "unicode form preserved", input: "café 👨‍💻 e\u0301lan\n", want: "café 👨‍💻 e\u0301lan"},
		{name: "synthetic marker excluded", input: runpkg.TruncatedOutputMarker + "tail\n", truncated: true, want: "tail"},
		{name: "literal marker kept when not truncated", input: runpkg.TruncatedOutputMarker + "tail\n", want: strings.TrimSuffix(runpkg.TruncatedOutputMarker, "\n") + "\ntail"},
		{name: "synthetic diagnostic kept", input: "exit status 1\n", want: "exit status 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOutputText(tt.input, tt.truncated); got != tt.want {
				t.Fatalf("normalizeOutputText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOutputProjectionWrapsLongLinesWithoutChangingSelectionOffsets(t *testing.T) {
	text := strings.Repeat("ab", 100)
	rows := projectOutputRows(text, 17, false)
	if len(rows) != 12 {
		t.Fatalf("wrapped rows = %d, want 12", len(rows))
	}
	if rows[0].start != 0 || rows[0].end != 17 || rows[len(rows)-1].end != len(text) {
		t.Fatalf("unexpected long-line projection: first=%#v last=%#v", rows[0], rows[len(rows)-1])
	}
	selection := outputSelection{snapshot: text, moved: true, anchor: outputPoint{start: 160, end: 161}, head: outputPoint{start: 2, end: 3}}
	if got := selection.text(); got != text[2:161] {
		t.Fatalf("backward long-line selection changed offsets: bytes=%d, want=%d", len(got), len(text[2:161]))
	}
}

func TestOutputProjectionMapsWideAndCombiningGraphemes(t *testing.T) {
	text := "a👨‍💻e\u0301z"
	rows := projectOutputRows(text, 8, false)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}

	first, ok := outputPointAt(text, rows[0], 1)
	if !ok || text[first.start:first.end] != "👨‍💻" {
		t.Fatalf("wide cell point = %#v/%t", first, ok)
	}
	secondCell, ok := outputPointAt(text, rows[0], 2)
	if !ok || secondCell != first {
		t.Fatalf("second wide cell = %#v/%t, want %#v", secondCell, ok, first)
	}
	combining, ok := outputPointAt(text, rows[0], 3)
	if !ok || text[combining.start:combining.end] != "e\u0301" {
		t.Fatalf("combining point = %#v/%t", combining, ok)
	}
}

func TestOutputSelectionTextOrdersBackwardBounds(t *testing.T) {
	text := "first\nsecond"
	rows := projectOutputRows(text, 20, false)
	start, _ := outputPointAt(text, rows[0], 2)
	end, _ := outputPointAt(text, rows[1], 2)
	selection := outputSelection{snapshot: text, anchor: end, head: start, moved: true}
	if got := selection.text(); got != "rst\nsec" {
		t.Fatalf("selection text = %q, want %q", got, "rst\nsec")
	}
}
