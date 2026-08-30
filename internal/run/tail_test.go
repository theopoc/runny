package run

import (
	"strings"
	"testing"
)

func TestTailBufferKeepsBoundedSuffix(t *testing.T) {
	var buffer tailBuffer
	buffer.Append([]byte(strings.Repeat("a", MaxOutputBytes)))
	buffer.Append([]byte("tail"))
	output := buffer.String()
	if !strings.HasPrefix(output, TruncatedOutputMarker) {
		t.Fatalf("output prefix = %q", output[:min(len(output), len(TruncatedOutputMarker))])
	}
	if !strings.HasSuffix(output, "tail") {
		t.Fatalf("output does not keep suffix")
	}
	if len(output) != len(TruncatedOutputMarker)+MaxOutputBytes {
		t.Fatalf("output length = %d", len(output))
	}
}

func TestTailBufferReplacesOversizedChunk(t *testing.T) {
	var buffer tailBuffer
	buffer.Append([]byte("old"))
	buffer.Append([]byte(strings.Repeat("b", MaxOutputBytes+10)))
	if got := buffer.String(); got != TruncatedOutputMarker+strings.Repeat("b", MaxOutputBytes) {
		t.Fatalf("unexpected bounded output length %d", len(got))
	}
}
