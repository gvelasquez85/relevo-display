package wire

import (
	"encoding/binary"
	"fmt"
	"io"
)

// VideoWriter frames JPEG messages with a reusable buffer. It is synchronous,
// retains no queued frames, and must be used by only one goroutine at a time.
// The destination must follow io.Writer's rule against retaining the slice.
type VideoWriter struct{ buffer []byte }

func (v *VideoWriter) WriteJPEG(w io.Writer, jpg []byte) error {
	if len(jpg) > MaxPayload-13 {
		return fmt.Errorf("JPEG payload exceeds limit %d", MaxPayload-13)
	}
	n := 16 + 13 + len(jpg)
	if cap(v.buffer) < n {
		v.buffer = make([]byte, n)
	} else {
		v.buffer = v.buffer[:n]
	}
	p := v.buffer
	binary.BigEndian.PutUint32(p[0:4], 1)
	binary.BigEndian.PutUint32(p[4:8], 108)
	binary.BigEndian.PutUint32(p[8:12], 0)
	binary.BigEndian.PutUint32(p[12:16], uint32(13+len(jpg)))
	binary.LittleEndian.PutUint32(p[16:20], uint32(len(jpg)+5))
	binary.LittleEndian.PutUint32(p[20:24], 1)
	copy(p[24:29], []byte{0, 0, 0, 1, 0x21})
	copy(p[29:], jpg)
	return writeAll(w, p)
}
