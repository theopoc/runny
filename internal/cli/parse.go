package cli

import (
	"errors"
	"flag"
	"io"
	"strings"
	"unicode"
)

type Options struct {
	Command           string
	Config            string
	Recursive         bool
	RecursiveSet      bool
	Depth             *int
	IncludeHidden     bool
	IncludeHiddenSet  bool
	Include           []string
	Exclude           []string
	Serial            bool
	SerialSet         bool
	Workers           *int
	FailFast          bool
	FailFastSet       bool
	SaveLogs          bool
	SaveLogsSet       bool
	DisableLogging    bool
	DisableLoggingSet bool
	Version           bool
	Help              bool
}

type listFlag []string

func (f *listFlag) String() string { return strings.Join(*f, ",") }
func (f *listFlag) Set(v string) error {
	*f = append(*f, v)
	return nil
}

func Parse(args []string) (Options, error) {
	var opts Options
	fs := flag.NewFlagSet("runny", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.Config, "config", "", "")
	fs.StringVar(&opts.Config, "c", "", "")
	fs.BoolVar(&opts.Recursive, "recursive", false, "")
	fs.BoolVar(&opts.Recursive, "r", false, "")
	depth := fs.Int("depth", 0, "")
	depthShort := fs.Int("d", 0, "")
	workers := fs.Int("workers", 0, "")
	workersShort := fs.Int("w", 0, "")
	fs.BoolVar(&opts.IncludeHidden, "include-hidden", false, "")
	fs.BoolVar(&opts.IncludeHidden, "H", false, "")
	fs.Var((*listFlag)(&opts.Include), "include", "")
	fs.Var((*listFlag)(&opts.Include), "i", "")
	fs.Var((*listFlag)(&opts.Exclude), "exclude", "")
	fs.Var((*listFlag)(&opts.Exclude), "e", "")
	fs.BoolVar(&opts.Serial, "serial", false, "")
	fs.BoolVar(&opts.Serial, "s", false, "")
	fs.BoolVar(&opts.FailFast, "fail-fast", false, "")
	fs.BoolVar(&opts.FailFast, "f", false, "")
	fs.BoolVar(&opts.SaveLogs, "save-logs", false, "")
	fs.BoolVar(&opts.SaveLogs, "L", false, "")
	fs.BoolVar(&opts.DisableLogging, "disable-logging", false, "")
	fs.BoolVar(&opts.DisableLogging, "N", false, "")
	fs.BoolVar(&opts.Version, "version", false, "")
	fs.BoolVar(&opts.Version, "v", false, "")
	fs.BoolVar(&opts.Help, "help", false, "")
	fs.BoolVar(&opts.Help, "h", false, "")
	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "recursive", "r":
			opts.RecursiveSet = true
		case "depth":
			opts.Depth = depth
		case "d":
			opts.Depth = depthShort
		case "include-hidden", "H":
			opts.IncludeHiddenSet = true
		case "serial", "s":
			opts.SerialSet = true
		case "workers":
			opts.Workers = workers
		case "w":
			opts.Workers = workersShort
		case "fail-fast", "f":
			opts.FailFastSet = true
		case "save-logs", "L":
			opts.SaveLogsSet = true
		case "disable-logging", "N":
			opts.DisableLoggingSet = true
		}
	})
	opts.Command = shellJoin(fs.Args())
	if len(opts.Include) > 0 && len(opts.Exclude) > 0 {
		return Options{}, errors.New("include and exclude are mutually exclusive")
	}
	if opts.Serial && opts.Workers != nil {
		return Options{}, errors.New("serial and workers are mutually exclusive")
	}
	if opts.DisableLogging && opts.SaveLogs {
		return Options{}, errors.New("disable-logging and save-logs are mutually exclusive")
	}
	return opts, nil
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_@%+=:,./-", r)
	}) == -1 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
