package cli

import "testing"

func TestParseCommandAfterDashDash(t *testing.T) {
	opts, err := Parse([]string{"--auto", "-d", "2", "-w", "4", "--", "pnpm", "test"})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Auto || opts.Depth == nil || *opts.Depth != 2 || opts.Workers == nil || *opts.Workers != 4 {
		t.Fatalf("unexpected opts: %#v", opts)
	}
	if opts.Command != "pnpm test" {
		t.Fatalf("command = %q", opts.Command)
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
