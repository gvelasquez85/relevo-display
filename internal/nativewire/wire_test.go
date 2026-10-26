package nativewire

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(b []byte) (int, error) {
	if len(b) > 2 {
		b = b[:2]
	}
	return w.Buffer.Write(b)
}
func TestVectorAndShortWrite(t *testing.T) {
	w := new(shortWriter)
	if e := Write(w, Frame{Type: Ping, Sequence: 42}); e != nil {
		t.Fatal(e)
	}
	if got := hex.EncodeToString(w.Bytes()); got != "52564c31000600000000002a00000000" {
		t.Fatal(got)
	}
	f, e := Read(w)
	if e != nil || f.Type != Ping || f.Sequence != 42 {
		t.Fatal(f, e)
	}
}
func TestRejectHeadersBeforePayload(t *testing.T) {
	for _, tc := range []struct {
		typ   Type
		n     uint32
		flags uint16
	}{{Hello, 4097, 0}, {DisplayJPEG, 4*1024*1024 + 1, 0}, {Ping, 1, 0}, {CameraJPEG, 512*1024 + 1, 0}, {AudioPCM, 65537, 0}, {11, 0, 0}, {Hello, 0, 1}} {
		var h [16]byte
		copy(h[:], "RVL1")
		binary.BigEndian.PutUint16(h[4:], uint16(tc.typ))
		binary.BigEndian.PutUint16(h[6:], tc.flags)
		binary.BigEndian.PutUint32(h[12:], tc.n)
		if _, e := Read(bytes.NewReader(h[:])); e == nil {
			t.Fatal(tc)
		}
	}
}
func TestTruncatedAndBadMagic(t *testing.T) {
	for _, b := range [][]byte{nil, make([]byte, 15), make([]byte, 16)} {
		if _, e := Read(bytes.NewReader(b)); e == nil {
			t.Fatal("accepted invalid header")
		}
	}
}

func TestPCMValidation(t *testing.T) {
	b := make([]byte, 890)
	binary.LittleEndian.PutUint32(b, 22050)
	binary.LittleEndian.PutUint16(b[4:], 1)
	binary.LittleEndian.PutUint16(b[6:], 16)
	if e := ValidatePCM(b); e != nil {
		t.Fatal(e)
	}
	for _, n := range []int{0, 8, 9, 889} {
		if ValidatePCM(b[:n]) == nil {
			t.Fatal(n)
		}
	}
	b[4] = 2
	if ValidatePCM(b) == nil {
		t.Fatal("stereo accepted")
	}
}

func TestExtensionFramesRemainBounded(t *testing.T) {
	for _, kind := range []Type{MultiTouch, Keyboard} {
		var b bytes.Buffer
		payload := append([]byte(`{"data":"`), bytes.Repeat([]byte{'x'}, 8192)...)
		payload = append(payload, []byte(`"}`)...)
		if Write(&b, Frame{Type: kind, Payload: payload}) == nil {
			t.Fatal("oversized input accepted", kind)
		}
	}
	var b bytes.Buffer
	if Write(&b, Frame{Type: MicrophonePCM, Payload: make([]byte, 891)}) == nil {
		t.Fatal("unbounded microphone frame")
	}
}
