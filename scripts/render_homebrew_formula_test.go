package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const validSHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestRenderHomebrewFormula(t *testing.T) {
	t.Parallel()

	output := filepath.Join(t.TempDir(), "runny.rb")
	cmd := exec.Command("bash", "render-homebrew-formula.sh", "v1.2.3", validSHA256, output)
	if combined, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render formula: %v\n%s", err, combined)
	}

	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read formula: %v", err)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat formula: %v", err)
	}
	if gotMode := info.Mode().Perm(); gotMode != 0o644 {
		t.Fatalf("formula mode = %o, want 644", gotMode)
	}
	want := `class Runny < Formula
  desc "Run shell commands across selected child directories from a TUI"
  homepage "https://github.com/theopoc/runny"
  url "https://github.com/theopoc/runny/archive/refs/tags/v1.2.3.tar.gz"
  sha256 "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  license "MIT"

  depends_on "go" => :build

  def install
    ldflags = "-s -w -X github.com/theopoc/runny/internal/app.Version=#{version}"
    system "go", "build", *std_go_args(ldflags:), "./cmd/runny"
  end

  test do
    assert_match "runny #{version}", shell_output("#{bin}/runny --version")
  end
end
`
	if string(got) != want {
		t.Fatalf("unexpected formula:\n%s", got)
	}
}

func TestRenderHomebrewFormulaRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		sha256  string
		wantErr string
	}{
		{name: "version", version: "latest", sha256: validSHA256, wantErr: "invalid release version"},
		{name: "checksum", version: "1.2.3", sha256: "not-a-checksum", wantErr: "invalid source SHA-256"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "runny.rb")
			cmd := exec.Command("bash", "render-homebrew-formula.sh", tt.version, tt.sha256, output)
			combined, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected failure, got success")
			}
			if !strings.Contains(string(combined), tt.wantErr) {
				t.Fatalf("expected %q in %q", tt.wantErr, combined)
			}
			if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
				t.Fatalf("output created after invalid input: %v", statErr)
			}
		})
	}
}
