#!/usr/bin/env bash

set -Eeuo pipefail

usage() {
  printf 'Usage: %s VERSION SOURCE_SHA256 OUTPUT\n' "${0##*/}" >&2
}

if [[ $# -ne 3 ]]; then
  usage
  exit 2
fi

readonly version="${1#v}"
readonly source_sha256="$2"
readonly output="$3"
output_dir="${output%/*}"
if [[ $output_dir == "$output" ]]; then
  output_dir="."
fi
readonly output_dir

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'invalid release version: %s\n' "$1" >&2
  exit 2
fi
if [[ ! $source_sha256 =~ ^[[:xdigit:]]{64}$ ]]; then
  printf 'invalid source SHA-256: %s\n' "$source_sha256" >&2
  exit 2
fi
if [[ -z ${output##*/} || ! -d $output_dir ]]; then
  printf 'output directory does not exist: %s\n' "$output_dir" >&2
  exit 2
fi

tmp_output="$(mktemp "${output}.tmp.XXXXXX")"
readonly tmp_output
cleanup() {
  rm -f -- "$tmp_output"
}
trap cleanup EXIT

cat >"$tmp_output" <<EOF
class Runny < Formula
  desc "Run shell commands across selected child directories from a TUI"
  homepage "https://github.com/theopoc/runny"
  url "https://github.com/theopoc/runny/archive/refs/tags/v${version}.tar.gz"
  sha256 "${source_sha256}"
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
EOF

chmod 0644 "$tmp_output"
mv -- "$tmp_output" "$output"
