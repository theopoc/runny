package runner

import (
	"strings"
	"sync"
	"testing"
)

func TestTailBufferRetainsOutputBelowLimit(t *testing.T) {
	buf := newTailBuffer(8)

	if _, err := buf.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "hello" {
		t.Fatalf("String() = %q, want %q", got, "hello")
	}
}

func TestTailBufferRetainsNewestBytesOverLimit(t *testing.T) {
	buf := newTailBuffer(5)

	if _, err := buf.Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	want := TruncatedOutputMarker + "defgh"
	if got := buf.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestTailBufferRetainsNewestBytesAcrossWrites(t *testing.T) {
	buf := newTailBuffer(5)

	for _, text := range []string{"ab", "cde", "fg"} {
		if _, err := buf.Write([]byte(text)); err != nil {
			t.Fatal(err)
		}
	}
	want := TruncatedOutputMarker + "cdefg"
	if got := buf.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestTailBufferConcurrentWrites(t *testing.T) {
	const writers = 32
	buf := newTailBuffer(writers)
	start := make(chan struct{})
	var wg sync.WaitGroup

	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := buf.Write([]byte("x")); err != nil {
				t.Errorf("Write() error = %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	if got := buf.String(); got != strings.Repeat("x", writers) {
		t.Fatalf("String() length = %d, want %d", len(got), writers)
	}
}
