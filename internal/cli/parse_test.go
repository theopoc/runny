package cli

import "testing"

func TestParseCommandAfterDashDash(t *testing.T) {
	opts, err := Parse([]string{"-d", "2", "-w", "4", "--", "pnpm", "test"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.Depth == nil || *opts.Depth != 2 || opts.Workers == nil || *opts.Workers != 4 {
		t.Fatalf("unexpected opts: %#v", opts)
	}
	if opts.Command != "pnpm test" {
		t.Fatalf("command = %q", opts.Command)
	}
}

func TestParsePreservesCommandArgumentBoundaries(t *testing.T) {
	opts, err := Parse([]string{"--", "printf", "%s\n", "hello; touch /tmp/runny-should-not-exist"})
	if err != nil {
		t.Fatal(err)
	}
	const want = "printf '%s\n' 'hello; touch /tmp/runny-should-not-exist'"
	if opts.Command != want {
		t.Fatalf("command = %q, want %q", opts.Command, want)
	}
}

func TestParseQuotesShellSensitiveArguments(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "empty argument",
			arg:  "",
			want: "printf ''",
		},
		{
			name: "single quote",
			arg:  "it's safe",
			want: `printf 'it'"'"'s safe'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := Parse([]string{"--", "printf", tt.arg})
			if err != nil {
				t.Fatal(err)
			}
			if opts.Command != tt.want {
				t.Fatalf("command = %q, want %q", opts.Command, tt.want)
			}
		})
	}
}

func TestParseTracksExplicitFalseBooleanFlags(t *testing.T) {
	tests := []struct {
		name  string
		flag  string
		value func(Options) bool
		set   func(Options) bool
	}{
		{
			name:  "recursive",
			flag:  "--recursive=false",
			value: func(opts Options) bool { return opts.Recursive },
			set:   func(opts Options) bool { return opts.RecursiveSet },
		},
		{
			name:  "include hidden",
			flag:  "--include-hidden=false",
			value: func(opts Options) bool { return opts.IncludeHidden },
			set:   func(opts Options) bool { return opts.IncludeHiddenSet },
		},
		{
			name:  "serial",
			flag:  "--serial=false",
			value: func(opts Options) bool { return opts.Serial },
			set:   func(opts Options) bool { return opts.SerialSet },
		},
		{
			name:  "fail fast",
			flag:  "--fail-fast=false",
			value: func(opts Options) bool { return opts.FailFast },
			set:   func(opts Options) bool { return opts.FailFastSet },
		},
		{
			name:  "save logs",
			flag:  "--save-logs=false",
			value: func(opts Options) bool { return opts.SaveLogs },
			set:   func(opts Options) bool { return opts.SaveLogsSet },
		},
		{
			name:  "disable logging",
			flag:  "--disable-logging=false",
			value: func(opts Options) bool { return opts.DisableLogging },
			set:   func(opts Options) bool { return opts.DisableLoggingSet },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := Parse([]string{tt.flag})
			if err != nil {
				t.Fatal(err)
			}
			if tt.value(opts) {
				t.Fatalf("%s value = true, want false", tt.flag)
			}
			if !tt.set(opts) {
				t.Fatalf("%s presence = false, want true", tt.flag)
			}
		})
	}
}

func TestParseRejectsAutoFlag(t *testing.T) {
	if _, err := Parse([]string{"--auto"}); err == nil {
		t.Fatal("expected --auto to be rejected")
	}
	if _, err := Parse([]string{"-a"}); err == nil {
		t.Fatal("expected -a to be rejected")
	}
}

func TestParseRejectsIncludeAndExclude(t *testing.T) {
	_, err := Parse([]string{"--include", "api", "--exclude", "web"})
	if err == nil {
		t.Fatal("expected include/exclude error")
	}
}

func TestParseShortFlagsAndVersion(t *testing.T) {
	opts, err := Parse([]string{"-v"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Version {
		t.Fatal("expected version flag")
	}
}
