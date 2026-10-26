package mediabridge

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCameraPacketLayout(t *testing.T) {
	b, e := Encode(CameraJPEG, 7, 123, []byte{255, 216, 255, 217})
	if e != nil {
		t.Fatal(e)
	}
	if len(b) != 28 || string(b[:4]) != "RVM1" || binary.LittleEndian.Uint16(b[4:]) != 1 || binary.LittleEndian.Uint16(b[6:]) != 1 || binary.LittleEndian.Uint32(b[8:]) != 7 || binary.LittleEndian.Uint32(b[12:]) != 4 || binary.LittleEndian.Uint64(b[16:]) != 123 {
		t.Fatalf("bad packet %x", b)
	}
}
func TestRejectInvalidPayload(t *testing.T) {
	for _, c := range []struct {
		k Kind
		p []byte
	}{{CameraJPEG, nil}, {CameraJPEG, bytes.Repeat([]byte{1}, 512*1024+1)}, {MicrophonePCM, []byte{1, 2}}, {Disconnect, []byte{1}}, {Kind(9), nil}} {
		if _, e := Encode(c.k, 0, 0, c.p); e == nil {
			t.Fatalf("accepted kind %d", c.k)
		}
	}
}
func TestMicrophoneFormatAndCopy(t *testing.T) {
	pcm := make([]byte, 890)
	binary.LittleEndian.PutUint32(pcm, 22050)
	binary.LittleEndian.PutUint16(pcm[4:], 1)
	binary.LittleEndian.PutUint16(pcm[6:], 16)
	b, e := Encode(MicrophonePCM, 1, 20, pcm)
	if e != nil {
		t.Fatal(e)
	}
	pcm[8] = 42
	if b[32] != 0 {
		t.Fatal("retained producer buffer")
	}
	for _, index := range []int{0, 4, 6} {
		invalid := append([]byte(nil), pcm...)
		invalid[index]++
		if _, e := Encode(MicrophonePCM, 0, 0, invalid); e == nil {
			t.Fatal("accepted wrong audio format")
		}
	}
	if b, e := Encode(Disconnect, 0, 0, nil); e != nil || len(b) != 24 {
		t.Fatal("disconnect must be header only")
	}
}
