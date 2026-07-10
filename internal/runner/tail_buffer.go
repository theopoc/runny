package runner

import "sync"

const truncatedOutputMarker = "[runny: output truncated]\n"

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

	n := len(p)
	if n == 0 {
		return 0, nil
	}
	if b.limit <= 0 {
		b.truncated = true
		return n, nil
	}
	if n >= b.limit {
		b.truncated = b.truncated || len(b.buf) > 0 || n > b.limit
		b.buf = append(b.buf[:0], p[n-b.limit:]...)
		return n, nil
	}
	if overflow := len(b.buf) + n - b.limit; overflow > 0 {
		copy(b.buf, b.buf[overflow:])
		b.buf = b.buf[:len(b.buf)-overflow]
		b.truncated = true
	}
	b.buf = append(b.buf, p...)
	return n, nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.truncated {
		return truncatedOutputMarker + string(b.buf)
	}
	return string(b.buf)
}
