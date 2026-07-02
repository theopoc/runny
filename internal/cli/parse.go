package cli

import (
	"errors"
	"flag"
	"strings"
)

type Options struct {
	Command        string
	Auto           bool
	Config         string
	Recursive      bool
	Depth          *int
	IncludeHidden  bool
	Include        []string
	Exclude        []string
	Serial         bool
	Workers        *int
	FailFast       bool
	SaveLogs       bool
	DisableLogging bool
	Version        bool
	Help           bool
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
	fs.BoolVar(&opts.Auto, "auto", false, "")
	fs.BoolVar(&opts.Auto, "a", false, "")
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
		case "depth":
			opts.Depth = depth
		case "d":
			opts.Depth = depthShort
		case "workers":
			opts.Workers = workers
		case "w":
			opts.Workers = workersShort
		}
	})
	opts.Command = strings.Join(fs.Args(), " ")
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
