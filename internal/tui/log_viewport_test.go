package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/theopoc/runny/internal/core"
	runpkg "github.com/theopoc/runny/internal/run"
)

func scrollBenchmarkModel(lines int) Model {
	m := NewModel(Options{Targets: []core.Target{{ID: "api", RelPath: "api", Selected: true}}})
	m.Width, m.Height, m.Focus = 120, 32, FocusLogs
	m.Logs["api"] = strings.Repeat("abcdefghijklmnopqrstuvwxyz 0123456789 abcdefghijklmnopqrstuvwxyz\n", lines)
	m.syncOutputViewport()
	m.outputViewport.GotoBottom()
	m.LogFollow = false
	_ = m.View()
	return m
}

func scrollFrame(m *Model, up bool) {
	button := tea.MouseWheelDown
	if up {
		button = tea.MouseWheelUp
	}
	next, _ := m.Update(tea.MouseWheelMsg{Button: button})
	*m = next.(Model)
	_ = m.View()
}

// A warm scroll must not allocate per retained line. This exercises the complete
// event/view path without a machine-dependent latency assertion in the test suite.
func TestOutputScrollAllocationsDoNotScaleWithRetainedLines(t *testing.T) {
	measure := func(lines int) float64 {
		m := scrollBenchmarkModel(lines)
		up := false
		return testing.AllocsPerRun(4, func() {
			up = !up
			scrollFrame(&m, up)
		})
	}
	small, large := measure(100), measure(10000)
	if large > small+100 {
		t.Fatalf("allocations per scroll: 100 lines = %.0f, 10000 lines = %.0f; want bounded by visible output", small, large)
	}
}

func BenchmarkOutputScroll(b *testing.B) {
	for _, lines := range []int{100, 10000, 50000} {
		b.Run(fmt.Sprint(lines), func(b *testing.B) {
			m := scrollBenchmarkModel(lines)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				scrollFrame(&m, i%2 == 0)
			}
		})
	}
}

func TestWrappedLogRowsPreserveTextAndANSIIndependently(t *testing.T) {
	for name, text := range map[string]string{
		"ascii":          "  abcdefghijklmnopqrstuvwxyz  ",
		"unicode":        "é界e\u0301👩‍💻🇫🇷 abc界 xyz",
		"color":          "\x1b[31mred red red \x1b[1mbold bold\x1b[22m red\x1b[0m plain",
		"truecolor":      "\x1b[38;2;12;123;234mcolored long content\x1b[39m normal",
		"hyperlink":      "\x1b]8;id=example;https://example.com\x1b\\long linked text\x1b]8;;\x1b\\ plain",
		"empty":          "",
		"trailing-style": "12345678\x1b[0m",
	} {
		t.Run(name, func(t *testing.T) {
			rows := appendWrappedLogLine(nil, text, 8)
			var reconstructed strings.Builder
			var got []uv.Cell
			for _, row := range rows {
				if ansi.StringWidth(row) > 8 {
					t.Fatalf("row wider than viewport: %q", row)
				}
				reconstructed.WriteString(ansi.Strip(row))
				// Parse each row from a fresh terminal state, as when scrolling.
				got = append(got, renderedLogCells(row)...)
			}
			if reconstructed.String() != ansi.Strip(text) {
				t.Fatalf("wrapped text = %q, want %q", reconstructed.String(), ansi.Strip(text))
			}
			want := renderedLogCells(text)
			if len(got) != len(want) {
				t.Fatalf("got %d cells, want %d", len(got), len(want))
			}
			for i := range want {
				if got[i].Content != want[i].Content || got[i].Width != want[i].Width || !got[i].Style.Equal(&want[i].Style) || !got[i].Link.Equal(&want[i].Link) {
					t.Fatalf("cell %d = %#v, want %#v", i, got[i], want[i])
				}
			}
		})
	}
}

func renderedLogCells(text string) []uv.Cell {
	width := ansi.StringWidth(text)
	buf := uv.NewScreenBuffer(max(1, width), 1)
	uv.NewStyledString(text).Draw(buf, uv.Rect(0, 0, max(1, width), 1))
	var cells []uv.Cell
	for x := 0; x < width; {
		cell := buf.CellAt(x, 0)
		cells = append(cells, *cell)
		x += max(1, cell.Width)
	}
	return cells
}

func TestOutputLayoutRefreshesForContentWidthTargetAndTruncation(t *testing.T) {
	m := NewModel(Options{Targets: []core.Target{{ID: "api"}, {ID: "web"}}})
	m.LogFollow = false
	m.Logs["api"], m.Logs["web"] = "abcdefghijkl", "ZYXWVUTSRQPO"
	text := func(id string, width, height int) string {
		return ansi.Strip(strings.Join(m.renderOutputLines(id, width, height), "\n"))
	}
	if got := text("api", 6, 2); got != "abcdef\nghijkl" {
		t.Fatalf("initial: %q", got)
	}
	if got := text("api", 4, 3); got != "abcd\nefgh\nijkl" {
		t.Fatalf("resize: %q", got)
	}
	if got := text("web", 6, 2); got != "ZYXWVU\nTSRQPO" {
		t.Fatalf("other target: %q", got)
	}
	// Equal byte length is not enough to identify cached content.
	m.Logs["api"] = "123456789012"
	if got := text("api", 6, 2); got != "123456\n789012" {
		t.Fatalf("replacement: %q", got)
	}
	m.Logs["api"] += "\r\nnext"
	if got := text("api", 12, 2); !strings.Contains(got, "next") || strings.Contains(got, "\r") {
		t.Fatalf("append/CRLF: %q", got)
	}
	m.Logs["api"] = "first\r\n\r\n"
	_ = text("api", 12, 3)
	if got, want := m.cachedOutputLineCount("api"), outputLineCount(m.Logs["api"]); got != want {
		t.Fatalf("CRLF line count = %d, want %d", got, want)
	}
	m.Logs["api"] = runpkg.TruncatedOutputMarker + "retained"
	if got := text("api", 40, 2); !strings.Contains(got, "output truncated") || !strings.Contains(got, "retained") {
		t.Fatalf("truncation: %q", got)
	}
	m.Logs["api"] = ""
	if got := text("api", 40, 2); got != "" {
		t.Fatalf("clear: %q", got)
	}
}

func TestOutputLayoutCacheDoesNotMutateOlderViewports(t *testing.T) {
	m := NewModel(Options{Targets: []core.Target{{ID: "api"}}})
	m.LogFollow = false
	m.Logs["api"] = "abcdefghijkl"
	first := m.configuredOutputViewport("api", 6, 2)
	want := first.View()
	m.Logs["api"] = "different output"
	_ = m.configuredOutputViewport("api", 4, 4).View()
	if got := first.View(); got != want {
		t.Fatalf("preparing another document changed an older viewport: %q != %q", got, want)
	}
}

func TestHistoryLayoutPreservesANSIStateAcrossLogicalLines(t *testing.T) {
	m := NewModel(Options{})
	m.HistoryLog = "\x1b[31malpha\nbeta\x1b[0m"
	v := m.configuredHistoryLogViewport(8, 1)
	v.SetYOffset(1)
	got := renderedLogCells(v.View())[0]
	want := renderedLogCells("\x1b[31mb\x1b[0m")[0]
	if got.Content != want.Content || !got.Style.Equal(&want.Style) {
		t.Fatalf("scrolling past color sequence lost ANSI state: %#v, want %#v", got, want)
	}
}

func TestWrappedLogErrorStyleUsesWholeLogicalLine(t *testing.T) {
	m := NewModel(Options{Targets: []core.Target{{ID: "api"}}})
	m.LogFollow = false
	m.Logs["api"] = "abcdefgh error"
	v := m.configuredOutputViewport("api", 4, 4)
	for row := 0; row < v.TotalLineCount(); row++ {
		if got, want := v.StyleLineFunc(row).Render("x"), logErrorStyle.Render("x"); got != want {
			t.Fatalf("fragment %d lost logical-line error style", row)
		}
	}
}

func TestOutputSelectionProjectionMatchesWrappedTabsAndUnicode(t *testing.T) {
	for _, text := range []string{"abc\tde\tx", "a界e\u0301👩‍💻🇫🇷 bc", "a\tb\tc"} {
		m := NewModel(Options{Targets: []core.Target{{ID: "api"}}})
		m.LogFollow = false
		m.Logs["api"] = text
		got := m.renderOutputLines("api", 6, 20)
		projected := projectOutputRows(text, 6, false)
		v := m.configuredOutputViewport("api", 6, 20)
		if v.TotalLineCount() != len(projected) {
			t.Fatalf("%q: %d output rows, %d selection rows", text, v.TotalLineCount(), len(projected))
		}
		for i, row := range projected {
			want := strings.ReplaceAll(text[row.start:row.end], "\t", "    ")
			if content := strings.TrimRight(ansi.Strip(got[i]), " "); content != strings.TrimRight(want, " ") {
				t.Fatalf("%q row %d: output %q, selection %q", text, i, content, want)
			}
			if point, ok := outputPointAt(text, row, 0); !ok || point.start != row.start {
				t.Fatalf("row %d does not map to original bytes: %#v", i, point)
			}
		}
	}
}

func BenchmarkOutputLayout(b *testing.B) {
	for name, text := range map[string]string{
		"lines":       strings.Repeat("abcdefghijklmnopqrstuvwxyz 0123456789 abcdefghijklmnopqrstuvwxyz\n", 50000),
		"single-line": strings.Repeat("x", 3250000),
		"ansi":        strings.Repeat("\x1b[32mabcdefghijklmnopqrstuvwxyz 0123456789 abcdefghijklmnopqrstuvwxyz\x1b[0m\n", 40000),
	} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var cache logLayoutCache
				cache.prepare(text, 64)
			}
		})
	}
}
