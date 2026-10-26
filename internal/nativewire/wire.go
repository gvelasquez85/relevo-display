// Package nativewire implements the independent Relevo RVL1 transport.
package nativewire

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type Type uint16

const (
	Hello         Type = 1
	Start         Type = 2
	DisplayJPEG   Type = 3
	Touch         Type = 4
	Status        Type = 5
	Ping          Type = 6
	Pong          Type = 7
	CameraJPEG    Type = 8
	AudioPCM      Type = 9
	FrameApplied  Type = 10
	MultiTouch    Type = 11
	Keyboard      Type = 12
	MicrophonePCM Type = 13
	InputFocus    Type = 14
	Pointer       Type = 15
	InputMode     Type = 16
)

type Frame struct {
	Type     Type
	Sequence uint32
	Payload  []byte
}

func limit(t Type) (uint32, error) {
	switch t {
	case Hello, Start, Touch, Status, InputFocus, Pointer, InputMode:
		return 4096, nil
	case MultiTouch, Keyboard:
		return 8192, nil
	case MicrophonePCM:
		return 890, nil
	case CameraJPEG:
		return 512 * 1024, nil
	case AudioPCM:
		return 65536, nil
	case DisplayJPEG:
		return 4 * 1024 * 1024, nil
	case Ping, Pong, FrameApplied:
		return 0, nil
	}
	return 0, fmt.Errorf("unsupported frame type %d", t)
}
func Read(r io.Reader) (Frame, error) {
	var h [16]byte
	var f Frame
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return f, e
	}
	if string(h[:4]) != "RVL1" || binary.BigEndian.Uint16(h[6:8]) != 0 {
		return f, fmt.Errorf("invalid RVL1 header")
	}
	f.Type = Type(binary.BigEndian.Uint16(h[4:6]))
	m, e := limit(f.Type)
	if e != nil {
		return f, e
	}
	n := binary.BigEndian.Uint32(h[12:])
	if n > m {
		return f, fmt.Errorf("frame exceeds type limit")
	}
	f.Sequence = binary.BigEndian.Uint32(h[8:12])
	f.Payload = make([]byte, int(n))
	_, e = io.ReadFull(r, f.Payload)
	if e == nil && isJSON(f.Type) && !json.Valid(f.Payload) {
		return f, fmt.Errorf("invalid JSON payload")
	}
	return f, e
}
func Write(w io.Writer, f Frame) error {
	m, e := limit(f.Type)
	if e != nil {
		return e
	}
	if uint64(len(f.Payload)) > uint64(m) {
		return fmt.Errorf("frame exceeds type limit")
	}
	if isJSON(f.Type) && !json.Valid(f.Payload) {
		return fmt.Errorf("invalid JSON payload")
	}
	var h [16]byte
	copy(h[:4], "RVL1")
	binary.BigEndian.PutUint16(h[4:6], uint16(f.Type))
	binary.BigEndian.PutUint32(h[8:12], f.Sequence)
	binary.BigEndian.PutUint32(h[12:], uint32(len(f.Payload)))
	for _, b := range [][]byte{h[:], f.Payload} {
		for len(b) > 0 {
			n, e := w.Write(b)
			if e != nil {
				return e
			}
			if n <= 0 || n > len(b) {
				return io.ErrShortWrite
			}
			b = b[n:]
		}
	}
	return nil
}

func isJSON(t Type) bool {
	return t == Hello || t == Start || t == Touch || t == Status || t == MultiTouch || t == Keyboard || t == InputFocus || t == Pointer || t == InputMode
}

// ValidatePCM validates the fixed legacy-device audio format and bounded packet duration.
func ValidatePCM(b []byte) error {
	if len(b) < 10 || len(b) > 890 || (len(b)-8)%2 != 0 || binary.LittleEndian.Uint32(b) != 22050 || binary.LittleEndian.Uint16(b[4:]) != 1 || binary.LittleEndian.Uint16(b[6:]) != 16 {
		return fmt.Errorf("invalid 22050 Hz mono PCM16 packet")
	}
	return nil
}
