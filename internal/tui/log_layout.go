package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// logLayoutCache retains only the last document/width. Copies of Model share
// this derived cache, but never their scroll position. Prepared content is
// immutable once installed, including while an older viewport still uses it.
type logLayoutCache struct {
	text      string
	width     int
	lineCount int
	viewport  viewport.Model
	errorRows []bool
}

func (c *logLayoutCache) prepare(text string, width int) {
	width = max(1, width)
	if c.width == width && c.text == text {
		return
	}
	lines := outputLines(text)
	rows := make([]string, 0, len(lines))
	errors := make([]bool, 0, len(lines))
	var state logANSIState
	for _, line := range lines {
		// outputLines has already removed the LF from a CRLF terminator.
		line = strings.TrimSuffix(line, "\r")
		start := len(rows)
		rows = appendWrappedLogLineWithState(rows, line, width, &state)
		isError := logLineIsError(line)
		for i := start; i < len(rows); i++ {
			errors = append(errors, isError)
		}
	}
	model := newLogViewport()
	model.SoftWrap = false // Rows are already wrapped; offsets are direct indices.
	model.SetWidth(width)
	model.SetContentLines(rows)
	*c = logLayoutCache{text: text, width: width, lineCount: len(lines), viewport: model, errorRows: errors}
}

func (c *logLayoutCache) configured(text string, width, height, offset int, styled bool) viewport.Model {
	c.prepare(text, width)
	model := c.viewport
	model.SetHeight(max(1, height))
	model.SetYOffset(offset)
	if styled {
		// Capture immutable row metadata, not the cache or an entire Model.
		errors := c.errorRows
		model.StyleLineFunc = func(row int) lipgloss.Style {
			if errors[row] {
				return logErrorStyle
			}
			return logInfoStyle
		}
	}
	return model
}

func logLineIsError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "exit status")
}

// appendWrappedLogLine scans each grapheme/escape only once, even for a single
// enormous line. Each resulting row restores its ANSI state independently so
// scrolling into the middle of a colored line retains its colors and links.
func appendWrappedLogLine(rows []string, line string, width int) []string {
	return appendWrappedLogLineWithState(rows, line, width, &logANSIState{})
}

type logANSIState struct {
	pen  uv.Style
	link uv.Link
}

func (s *logANSIState) prefix() string {
	var prefix string
	if !s.pen.IsZero() {
		prefix = s.pen.String()
	}
	if !s.link.IsZero() {
		prefix += ansi.SetHyperlink(s.link.URL, s.link.Params)
	}
	return prefix
}

func (s *logANSIState) suffix() string {
	var suffix string
	if !s.link.IsZero() {
		suffix = ansi.SetHyperlink("")
	}
	if !s.pen.IsZero() {
		suffix += ansi.ResetStyle
	}
	return suffix
}

func appendWrappedLogLineWithState(rows []string, line string, width int, style *logANSIState) []string {
	plainASCII := true
	for i := range len(line) {
		if line[i] < ' ' || line[i] > '~' {
			plainASCII = false
			break
		}
	}
	if plainASCII {
		prefix, suffix := style.prefix(), style.suffix()
		for len(line) > width {
			rows = append(rows, prefix+line[:width]+suffix)
			line = line[width:]
		}
		return append(rows, prefix+line+suffix)
	}

	p := ansi.GetParser()
	defer ansi.PutParser(p)
	var state byte
	var row strings.Builder
	row.WriteString(style.prefix())
	cells := 0
	flush := func() {
		row.WriteString(style.suffix())
		rows = append(rows, row.String())
		row.Reset()
		cells = 0
	}
	for len(line) > 0 {
		seq, size, n, next := ansi.DecodeSequence(line, state, p)
		if seq == "\t" {
			size = outputTabWidth
		}
		if size > 0 && cells > 0 && cells+size > width {
			flush()
			row.WriteString(style.prefix())
		}
		row.WriteString(seq)
		cells += size
		switch {
		case ansi.HasCsiPrefix(seq) && p.Command() == 'm':
			uv.ReadStyle(p.Params(), &style.pen)
		case ansi.HasOscPrefix(seq) && p.Command() == 8:
			uv.ReadLink(p.Data(), &style.link)
		}
		line, state = line[n:], next
	}
	flush()
	return rows
}
