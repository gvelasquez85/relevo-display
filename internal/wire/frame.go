// Package wire implements the version 1 PeerTalk frame format used by Yam Display.
package wire

import (
	"encoding/binary"
	"fmt"
	"io"
)

const MaxPayload = 32 * 1024 * 1024

type Frame struct {
	Type    uint32
	Tag     uint32
	Payload []byte
}

// Read reads one complete frame, rejecting unsupported versions and oversized payloads.
func Read(r io.Reader) (Frame, error) {
	var h [16]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Frame{}, err
	}
	if v := binary.BigEndian.Uint32(h[:4]); v != 1 {
		return Frame{}, fmt.Errorf("unsupported PeerTalk version %d", v)
	}
	n := binary.BigEndian.Uint32(h[12:])
	if n > MaxPayload {
		return Frame{}, fmt.Errorf("payload length %d exceeds limit %d", n, MaxPayload)
	}
	f := Frame{Type: binary.BigEndian.Uint32(h[4:8]), Tag: binary.BigEndian.Uint32(h[8:12]), Payload: make([]byte, int(n))}
	if _, err := io.ReadFull(r, f.Payload); err != nil {
		return Frame{}, err
	}
	return f, nil
}

// Write writes one frame, including when the writer accepts only partial chunks.
func Write(w io.Writer, f Frame) error {
	if len(f.Payload) > MaxPayload {
		return fmt.Errorf("payload length %d exceeds limit %d", len(f.Payload), MaxPayload)
	}
	var h [16]byte
	binary.BigEndian.PutUint32(h[:4], 1)
	binary.BigEndian.PutUint32(h[4:8], f.Type)
	binary.BigEndian.PutUint32(h[8:12], f.Tag)
	binary.BigEndian.PutUint32(h[12:], uint32(len(f.Payload)))
	if err := writeAll(w, h[:]); err != nil {
		return err
	}
	return writeAll(w, f.Payload)
}
func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if n < 0 || n > len(p) {
			return io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

// Text prefixes UTF-8 bytes with their four-byte, big-endian byte length.
func Text(s string) []byte {
	b := make([]byte, 4+len(s))
	binary.BigEndian.PutUint32(b[:4], uint32(len(s)))
	copy(b[4:], s)
	return b
}

// ParseText requires exactly the declared number of UTF-8 bytes.
func ParseText(b []byte) (string, error) {
	if len(b) < 4 {
		return "", io.ErrUnexpectedEOF
	}
	n := binary.BigEndian.Uint32(b[:4])
	if uint64(n) != uint64(len(b)-4) {
		return "", fmt.Errorf("text length %d does not match %d bytes", n, len(b)-4)
	}
	return string(b[4:]), nil
}
