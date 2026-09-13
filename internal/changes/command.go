package changes

import (
	"path/filepath"
	"strings"
	"unicode"
)

type commandInfo struct {
	engine   string
	phase    string
	detailed bool
	multi    bool
	simple   bool
}

func inspectCommand(command string) commandInfo {
	words, simple := literalWords(command)
	var info commandInfo
	// Inspect known shell -c payloads for aggregation hints only. A literal
	// argument to Terraform may contain arbitrary command-looking text.
	if len(words) >= 3 {
		switch filepath.Base(words[0]) {
		case "sh", "bash", "zsh":
			for i, word := range words[1 : len(words)-1] {
				if word == "-c" || word == "-lc" || word == "-ic" {
					info = inspectCommand(words[i+2])
					info.simple = false
					return info
				}
			}
		}
	}
	terragrunt := false
	for i, word := range words {
		if simple && i > 0 {
			break
		}
		engine := filepath.Base(word)
		terragrunt = terragrunt || engine == "terragrunt"
		if engine == "terraform" || engine == "tofu" || engine == "terragrunt" {
			info.engine = engine
			for _, arg := range words[i+1:] {
				if arg == "plan" || arg == "apply" {
					info.phase = arg
					break
				}
			}
		}
	}
	if terragrunt {
		for _, word := range words {
			if word == "--all" || word == "-a" || word == "run-all" || word == "stack" {
				info.multi = true
			}
		}
	}
	if !simple || len(words) < 2 {
		return info
	}
	engine := filepath.Base(words[0])
	if engine != "terraform" && engine != "tofu" && engine != "terragrunt" {
		return info
	}
	info.engine, info.simple = engine, true
	for _, word := range words[1:] {
		switch word {
		case "-detailed-exitcode", "-detailed-exitcode=true":
			info.detailed = true
		case "-detailed-exitcode=false":
			info.detailed = false
		}
	}
	return info
}

// literalWords recognizes literal arguments only. It never evaluates shell code.
// Any expansion, operator, redirect or comment disables exit-code specialization.
func literalWords(command string) ([]string, bool) {
	var words []string
	var word strings.Builder
	var quote rune
	started, simple := false, true
	flush := func() {
		if started {
			words = append(words, word.String())
			word.Reset()
			started = false
		}
	}
	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		if quote == '\'' {
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
			continue
		}
		if char == '\\' {
			if i+1 == len(runes) {
				simple = false
				break
			}
			next := runes[i+1]
			if quote == 0 || strings.ContainsRune("$`\"\\\n", next) {
				i++
				if next != '\n' {
					word.WriteRune(next)
					started = true
				}
			} else {
				word.WriteRune(char)
			}
			continue
		}
		if quote == '"' {
			if char == quote {
				quote = 0
			} else {
				if char == '$' || char == '`' {
					simple = false
				}
				word.WriteRune(char)
			}
			continue
		}
		if strings.ContainsRune("$`;&|<>(){}\n\r*?[]#~", char) {
			simple = false
		}
		if char == '\'' || char == '"' {
			quote = char
			started = true
		} else if unicode.IsSpace(char) || strings.ContainsRune(";&|", char) {
			flush()
		} else {
			started = true
			word.WriteRune(char)
		}
	}
	flush()
	return words, simple && quote == 0
}
