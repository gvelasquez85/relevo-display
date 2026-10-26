package winplayback

import (
	"encoding/binary"
	"testing"
)

type sink struct {
	calls  int
	closed int
	pcm    []byte
}

func (s *sink) WritePCM(b []byte) error { s.calls++; s.pcm = append([]byte(nil), b...); return nil }
func (s *sink) Close() error            { s.closed++; return nil }
func TestMonitorValidatesFormatAndOnlySendsPCM(t *testing.T) {
	s := &sink{}
	p := New(s)
	bad := make([]byte, 890)
	if p.Packet(bad) == nil || s.calls != 0 {
		t.Fatal("invalid audio forwarded")
	}
	binary.LittleEndian.PutUint32(bad, 22050)
	binary.LittleEndian.PutUint16(bad[4:], 1)
	binary.LittleEndian.PutUint16(bad[6:], 16)
	bad[8] = 7
	if e := p.Packet(bad); e != nil {
		t.Fatal(e)
	}
	if s.calls != 1 || len(s.pcm) != 882 || s.pcm[0] != 7 {
		t.Fatal("header forwarded or audio lost")
	}
	if e := p.Close(); e != nil {
		t.Fatal(e)
	}
	p.Close()
	if s.closed != 1 {
		t.Fatal("close repeated")
	}
	if p.Packet(bad) == nil {
		t.Fatal("audio after close")
	}
}
