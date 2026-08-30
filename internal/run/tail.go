package run

const (
	MaxOutputBytes        = 4 << 20
	TruncatedOutputMarker = "[runny: output truncated]\n"
)

type tailBuffer struct {
	data      []byte
	truncated bool
}

func (b *tailBuffer) Append(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	if len(chunk) >= MaxOutputBytes {
		b.data = append(b.data[:0], chunk[len(chunk)-MaxOutputBytes:]...)
		b.truncated = true
		return
	}
	if overflow := len(b.data) + len(chunk) - MaxOutputBytes; overflow > 0 {
		copy(b.data, b.data[overflow:])
		b.data = b.data[:len(b.data)-overflow]
		b.truncated = true
	}
	b.data = append(b.data, chunk...)
}

func (b *tailBuffer) String() string {
	if b.truncated {
		return TruncatedOutputMarker + string(b.data)
	}
	return string(b.data)
}
