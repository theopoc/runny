package runner

import (
	"strings"
	"sync"
)

const (
	MaxOutputBytes        = 4 << 20
	TruncatedOutputMarker = "[runny: output truncated]\n"
)

type tailBuffer struct {
	mu        sync.Mutex
	limit     int
	buf       []byte
	truncated bool
}

func newTailBuffer(limit int) *tailBuffer {
	return &tailBuffer{limit: limit, buf: make([]byte, 0, max(limit, 0))}
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf, b.truncated = appendTail(b.buf, p, b.limit, b.truncated)
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.truncated {
		return TruncatedOutputMarker + string(b.buf)
	}
	return string(b.buf)
}

func AppendOutputTail(current string, chunk string, truncated bool) (string, bool) {
	if truncated {
		current = strings.TrimPrefix(current, TruncatedOutputMarker)
	}
	buf, truncated := appendTail([]byte(current), []byte(chunk), MaxOutputBytes, truncated)
	if truncated {
		return TruncatedOutputMarker + string(buf), true
	}
	return string(buf), false
}

func appendTail(buf []byte, p []byte, limit int, truncated bool) ([]byte, bool) {
	n := len(p)
	if n == 0 {
		return buf, truncated
	}
	if limit <= 0 {
		return buf[:0], true
	}
	if n >= limit {
		truncated = truncated || len(buf) > 0 || n > limit
		return append(buf[:0], p[n-limit:]...), truncated
	}
	if overflow := len(buf) + n - limit; overflow > 0 {
		copy(buf, buf[overflow:])
		buf = buf[:len(buf)-overflow]
		truncated = true
	}
	return append(buf, p...), truncated
}
