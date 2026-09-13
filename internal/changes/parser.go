// Package changes passively reads Terraform/OpenTofu summaries and Terragrunt's
// standard log envelopes. It never changes commands or their original output.
package changes

import (
	"encoding/json"
	"math"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/theopoc/runny/internal/core"
)

const maxLineBytes = 64 << 10
const maxUnits = 4096

var (
	planLine   = regexp.MustCompile("^Plan: (?:[0-9]+ to import, )?([0-9]+) to add, ([0-9]+) to change, ([0-9]+) to destroy(?:, [0-9]+ to forget)?\\.(?: Actions: [0-9]+ to invoke\\.)?$")
	applyLine  = regexp.MustCompile("^Apply complete! Resources: (?:[0-9]+ imported, )?([0-9]+) added, ([0-9]+) changed, ([0-9]+) destroyed(?:, [0-9]+ forgotten)?\\.(?: Actions: [0-9]+ invoked\\.)?$")
	prettyLine = regexp.MustCompile("^(?:\\S+\\s+)?(STDOUT|STDERR|INFO|ERROR|WARN|DEBUG|TRACE)\\s+(?:\\[([^\\]]+)\\]\\s+)?(?:(terraform|tofu):\\s?)?(.*)$")
)

type lineBuffer struct {
	data     []byte
	dropping bool
}

func (b *lineBuffer) write(data []byte, line func(string), overflow func()) {
	for _, c := range data {
		if c == '\n' || c == '\r' {
			if !b.dropping && len(b.data) != 0 {
				line(string(b.data))
			}
			b.data = b.data[:0]
			b.dropping = false
		} else if !b.dropping {
			if len(b.data) == maxLineBytes {
				b.data = b.data[:0]
				b.dropping = true
				overflow()
			} else {
				b.data = append(b.data, c)
			}
		}
	}
}

func (b *lineBuffer) finish(line func(string)) {
	if !b.dropping && len(b.data) != 0 {
		line(string(b.data))
	}
	b.data = nil
}

type unit struct {
	pending lineBuffer
	summary core.ChangeSummary
	badJSON bool
}

type Parser struct {
	command                                       commandInfo
	outer                                         lineBuffer
	units                                         map[string]*unit
	detected, incomplete, diagnosticError, framed bool
	announcing, announced                         bool
	expectedUnits                                 int
}

func New(command string) *Parser {
	return &Parser{command: inspectCommand(command), units: make(map[string]*unit)}
}

func (p *Parser) Detected() bool { return p.detected }

func (p *Parser) Write(chunk []byte) {
	p.outer.write(chunk, p.readLine, func() { p.incomplete = true })
}

func (p *Parser) getUnit(id string) *unit {
	if id != "" {
		id = filepath.Clean(id)
	}
	if u := p.units[id]; u != nil {
		return u
	}
	if len(p.units) >= maxUnits {
		p.incomplete = true
		return nil
	}
	u := &unit{}
	p.units[id] = u
	return u
}

func (p *Parser) readLine(raw string) {
	line := strings.TrimSpace(ansi.Strip(raw))
	if line == "" {
		return
	}
	if strings.HasPrefix(line, "{") {
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(line), &fields) == nil {
			if msg, ok := fields["msg"]; ok && len(fields["level"]) != 0 {
				var value, level, id, engine string
				if json.Unmarshal(msg, &value) != nil {
					return
				}
				p.readAnnouncement(value)
				_ = json.Unmarshal(fields["level"], &level)
				_ = json.Unmarshal(fields["working-dir"], &id)
				if id == "" {
					_ = json.Unmarshal(fields["prefix"], &id)
				}
				id = strings.TrimSuffix(strings.TrimPrefix(id, "["), "]")
				if strings.EqualFold(level, "error") {
					p.diagnosticError = true
				}
				_ = json.Unmarshal(fields["tf-path"], &engine)
				var args []string
				_ = json.Unmarshal(fields["tf-command-args"], &args)
				// Auto-init and dependency output calls may use absolute paths
				// while plan/apply use relative unit paths. They are not extra
				// participants in the resource summary.
				if len(args) > 0 && args[0] != "plan" && args[0] != "apply" {
					return
				}
				motor := filepath.Base(engine) == "terraform" || filepath.Base(engine) == "tofu"
				motor = motor || strings.EqualFold(level, "stdout") || strings.EqualFold(level, "stderr")
				if motor {
					p.framed, p.detected = true, true
					if u := p.getUnit(id); u != nil {
						u.pending.write([]byte(value), func(s string) { p.readEngine(u, s) }, func() { p.incomplete = true })
					}
				}
				return
			}
		}
	}
	if match := prettyLine.FindStringSubmatch(line); match != nil {
		if match[1] == "INFO" {
			p.readAnnouncement(match[4])
		} else {
			p.announcing = false
		}
		if match[1] == "ERROR" {
			p.diagnosticError = true
		}
		if match[3] != "" || match[1] == "STDOUT" || match[1] == "STDERR" {
			p.framed, p.detected = true, true
			if u := p.getUnit(match[2]); u != nil {
				p.readEngine(u, match[4])
			}
		}
		return
	}
	if p.readAnnouncement(line) {
		return
	}
	if isEngineLine(line) {
		if u := p.getUnit(""); u != nil {
			p.readEngine(u, line)
		}
	} else if strings.HasPrefix(strings.TrimLeft(line, "│╷╵ "), "Error:") {
		p.diagnosticError = true
	}
}

// Standard run announcements provide participants even when a unit produces
// no engine frames. A missing summary must never become a partial total.
func (p *Parser) readAnnouncement(value string) bool {
	consumed := false
	for _, raw := range strings.Split(value, "\n") {
		line := strings.TrimSpace(ansi.Strip(raw))
		if strings.Contains(line, "The following units will be run") {
			p.units = make(map[string]*unit)
			p.command.multi = true
			p.announcing, p.announced, p.expectedUnits = true, true, 0
			consumed = true
			continue
		}
		if !p.announcing {
			continue
		}
		if line == "" || line == "." {
			consumed = true
			continue
		}
		if strings.ContainsAny(line, "├╰└") {
			id := strings.TrimLeft(line, "│ ├╰└─\t")
			if id != "" {
				p.getUnit(id)
				p.expectedUnits++
			}
			consumed = true
			continue
		}
		p.announcing = false
	}
	return consumed
}

func isEngineLine(line string) bool {
	return strings.HasPrefix(line, "Plan:") || strings.HasPrefix(line, "Apply complete!") ||
		strings.HasPrefix(line, "No changes.") ||
		strings.HasPrefix(line, "{") && (strings.Contains(line, "\"terraform.ui\"") || strings.Contains(line, "\"tofu.ui\""))
}

func (p *Parser) readEngine(u *unit, raw string) {
	line := strings.TrimSpace(ansi.Strip(raw))
	if strings.HasPrefix(strings.TrimLeft(line, "│╷╵ "), "Error:") {
		p.diagnosticError = true
	}
	if strings.HasPrefix(line, "{") {
		p.readJSON(u, line)
		return
	}
	phase := ""
	var values []string
	if strings.HasPrefix(line, "Plan:") {
		phase, values = "plan", planLine.FindStringSubmatch(line)
	} else if strings.HasPrefix(line, "Apply complete!") {
		phase, values = "apply", applyLine.FindStringSubmatch(line)
	} else {
		switch line {
		case "No changes. Your infrastructure matches the configuration.",
			"No changes. Your infrastructure still matches the configuration.",
			"No changes. No objects need to be destroyed.":
			p.detected = true
			u.summary = core.ChangeSummary{Detected: true, Known: true, Phase: "plan"}
		}
		return
	}
	p.detected = true
	u.summary = core.ChangeSummary{Detected: true, Phase: phase}
	if values == nil {
		return
	}
	counts := [3]int64{}
	for i := range counts {
		n, err := strconv.ParseInt(values[i+1], 10, 64)
		if err != nil {
			return
		}
		counts[i] = n
	}
	u.summary = core.ChangeSummary{Detected: true, Known: true, Phase: phase, Add: counts[0], Change: counts[1], Destroy: counts[2]}
}

func (p *Parser) readJSON(u *unit, line string) {
	var event struct {
		Type    string
		Module  string `json:"@module"`
		UI      string
		Level   string `json:"@level"`
		Changes struct {
			Operation           string
			Add, Change, Remove *int64
		}
	}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		if strings.Contains(line, "\"change_summary\"") {
			p.detected = true
			u.summary = core.ChangeSummary{}
		}
		return
	}
	if event.Module != "terraform.ui" && event.Module != "tofu.ui" {
		return
	}
	p.detected = true
	if event.Level == "error" {
		p.diagnosticError = true
	}
	switch event.Type {
	case "version":
		u.summary = core.ChangeSummary{}
		u.badJSON = strings.Split(event.UI, ".")[0] != "1"
	case "apply_start":
		u.summary = core.ChangeSummary{}
	case "change_summary":
		u.summary = core.ChangeSummary{}
		c := event.Changes
		if u.badJSON || c.Operation != "plan" && c.Operation != "apply" ||
			c.Add == nil || c.Change == nil || c.Remove == nil ||
			*c.Add < 0 || *c.Change < 0 || *c.Remove < 0 {
			return
		}
		u.summary = core.ChangeSummary{Detected: true, Known: true, Phase: c.Operation, Add: *c.Add, Change: *c.Change, Destroy: *c.Remove}
	}
}

// Finish flushes unterminated lines and returns an immutable value.
func (p *Parser) Finish() core.ChangeSummary {
	p.outer.finish(p.readLine)
	for _, u := range p.units {
		u.pending.finish(func(s string) { p.readEngine(u, s) })
	}
	result := core.ChangeSummary{Detected: p.detected}
	if !p.detected || p.incomplete || p.diagnosticError || len(p.units) == 0 || p.announced && p.expectedUnits == 0 {
		return result
	}
	if p.command.multi || len(p.units) > 1 {
		if _, anonymous := p.units[""]; anonymous {
			return result
		}
	}
	phase := ""
	for _, u := range p.units {
		if !u.summary.Known {
			return core.ChangeSummary{Detected: true}
		}
		s := u.summary
		if phase != "" && s.Phase != phase {
			return core.ChangeSummary{Detected: true}
		}
		phase = s.Phase
		if s.Add > math.MaxInt64-result.Add || s.Change > math.MaxInt64-result.Change || s.Destroy > math.MaxInt64-result.Destroy {
			return core.ChangeSummary{Detected: true}
		}
		result.Add += s.Add
		result.Change += s.Change
		result.Destroy += s.Destroy
	}
	if p.command.phase != "" && phase != p.command.phase {
		return core.ChangeSummary{Detected: true}
	}
	result.Known, result.Phase = true, phase
	return result
}

// DetailedPlanSucceeded specializes literal commands only. Terragrunt also
// requires framed complete results and no reported hook or engine error.
func (p *Parser) DetailedPlanSucceeded(summary core.ChangeSummary) bool {
	c := p.command
	if !c.simple || !c.detailed || c.phase != "plan" || p.diagnosticError || !p.detected {
		return false
	}
	if c.engine == "terraform" || c.engine == "tofu" {
		return true
	}
	return c.engine == "terragrunt" && p.framed && summary.Known && summary.Phase == "plan"
}
