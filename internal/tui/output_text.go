package tui

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
	runpkg "github.com/theopoc/runny/internal/run"
)

// Match Lip Gloss's default tab expansion in both output and selection layout.
const outputTabWidth = 4

func normalizeOutputText(output string, truncated bool) string {
	output = strings.ToValidUTF8(output, "�")
	output = ansi.Strip(output)
	output = strings.ReplaceAll(output, "\r\n", "\n")
	output = strings.ReplaceAll(output, "\r", "\n")
	output = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			return r
		}
		return -1
	}, output)
	if truncated {
		marker := strings.TrimSuffix(runpkg.TruncatedOutputMarker, "\n")
		if strings.HasPrefix(output, marker) {
			output = strings.TrimPrefix(output, marker)
			output = strings.TrimPrefix(output, "\n")
		}
	}
	return strings.TrimRight(output, "\n")
}

type outputPoint struct {
	start int
	end   int
}

type outputVisualRow struct {
	start     int
	end       int
	synthetic string
}

func projectOutputRows(text string, width int, truncated bool) []outputVisualRow {
	width = max(1, width)
	rows := make([]outputVisualRow, 0, strings.Count(text, "\n")+1)
	if truncated {
		rows = append(rows, outputVisualRow{start: -1, end: -1, synthetic: strings.TrimSuffix(runpkg.TruncatedOutputMarker, "\n")})
	}
	if text == "" {
		return rows
	}

	lineStart := 0
	for lineStart <= len(text) {
		lineEnd := strings.IndexByte(text[lineStart:], '\n')
		last := lineEnd < 0
		if last {
			lineEnd = len(text)
		} else {
			lineEnd += lineStart
		}
		rows = appendOutputLineRows(rows, text, lineStart, lineEnd, width)
		if last {
			break
		}
		lineStart = lineEnd + 1
	}
	return rows
}

func appendOutputLineRows(rows []outputVisualRow, text string, start, end, width int) []outputVisualRow {
	if start == end {
		return append(rows, outputVisualRow{start: start, end: end})
	}
	line := text[start:end]
	graphemes := uniseg.NewGraphemes(line)
	rowStart := start
	cells := 0
	for graphemes.Next() {
		from, to := graphemes.Positions()
		graphemeWidth := outputGraphemeWidth(graphemes)
		if cells > 0 && cells+graphemeWidth > width {
			rows = append(rows, outputVisualRow{start: rowStart, end: start + from})
			rowStart = start + from
			cells = 0
		}
		cells += graphemeWidth
		if start+to == end {
			rows = append(rows, outputVisualRow{start: rowStart, end: end})
		}
	}
	return rows
}

func outputPointAt(text string, row outputVisualRow, cell int) (outputPoint, bool) {
	if row.start < 0 || row.start >= row.end || cell < 0 {
		return outputPoint{}, false
	}
	graphemes := uniseg.NewGraphemes(text[row.start:row.end])
	column := 0
	for graphemes.Next() {
		from, to := graphemes.Positions()
		width := outputGraphemeWidth(graphemes)
		if cell >= column && cell < column+width {
			return outputPoint{start: row.start + from, end: row.start + to}, true
		}
		column += width
	}
	return outputPoint{}, false
}

func outputGraphemeWidth(graphemes *uniseg.Graphemes) int {
	if graphemes.Str() == "\t" {
		return outputTabWidth
	}
	return max(1, graphemes.Width())
}
