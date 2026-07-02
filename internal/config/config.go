package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v4"
)

type Config struct {
	Command        string   `yaml:"command"`
	Auto           bool     `yaml:"auto"`
	Recursive      bool     `yaml:"recursive"`
	Depth          int      `yaml:"depth"`
	IncludeHidden  bool     `yaml:"include_hidden"`
	Include        []string `yaml:"include"`
	Exclude        []string `yaml:"exclude"`
	Serial         bool     `yaml:"serial"`
	Workers        int      `yaml:"workers"`
	FailFast       bool     `yaml:"fail_fast"`
	SaveLogs       bool     `yaml:"save_logs"`
	DisableLogging bool     `yaml:"disable_logging"`
}

type FlagOverrides struct {
	Command        *string
	Auto           *bool
	Recursive      *bool
	Depth          *int
	IncludeHidden  *bool
	Include        []string
	Exclude        []string
	Serial         *bool
	Workers        *int
	FailFast       *bool
	SaveLogs       *bool
	DisableLogging *bool
}

type LoadOptions struct {
	HomeDir string
	WorkDir string
	Config  string
	Flags   FlagOverrides
}

func Defaults() Config {
	return Config{
		Depth: 1,
	}
}

func Load(opts LoadOptions) (Config, error) {
	cfg := Defaults()
	paths := []string{
		filepath.Join(opts.HomeDir, ".runny.yaml"),
		filepath.Join(opts.WorkDir, ".runny.yaml"),
	}
	if opts.Config != "" {
		paths = append(paths, opts.Config)
	}
	for _, path := range paths {
		if err := mergeFile(&cfg, path); err != nil {
			return Config{}, err
		}
	}
	applyFlags(&cfg, opts.Flags)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func mergeFile(cfg *Config, path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func applyFlags(cfg *Config, flags FlagOverrides) {
	if flags.Command != nil {
		cfg.Command = *flags.Command
	}
	if flags.Auto != nil {
		cfg.Auto = *flags.Auto
	}
	if flags.Recursive != nil {
		cfg.Recursive = *flags.Recursive
	}
	if flags.Depth != nil {
		cfg.Depth = *flags.Depth
	}
	if flags.IncludeHidden != nil {
		cfg.IncludeHidden = *flags.IncludeHidden
	}
	if flags.Include != nil {
		cfg.Include = flags.Include
	}
	if flags.Exclude != nil {
		cfg.Exclude = flags.Exclude
	}
	if flags.Serial != nil {
		cfg.Serial = *flags.Serial
	}
	if flags.Workers != nil {
		cfg.Workers = *flags.Workers
	}
	if flags.FailFast != nil {
		cfg.FailFast = *flags.FailFast
	}
	if flags.SaveLogs != nil {
		cfg.SaveLogs = *flags.SaveLogs
	}
	if flags.DisableLogging != nil {
		cfg.DisableLogging = *flags.DisableLogging
	}
}

func (c Config) Validate() error {
	if len(c.Include) > 0 && len(c.Exclude) > 0 {
		return errors.New("include and exclude are mutually exclusive")
	}
	if c.Serial && c.Workers > 0 {
		return errors.New("serial and workers are mutually exclusive")
	}
	if c.DisableLogging && c.SaveLogs {
		return errors.New("disable_logging and save_logs are mutually exclusive")
	}
	if c.Depth < 0 {
		return errors.New("depth must be >= 0")
	}
	if c.Workers < 0 {
		return errors.New("workers must be >= 0")
	}
	return nil
}
