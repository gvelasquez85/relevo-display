package wininput

import (
	"image"
	"testing"
)

type pointerSink struct{ packets []Packet }

func (s *pointerSink) Send(p Packet) error { s.packets = append(s.packets, p); return nil }
func TestRelativePointerAndDisconnectRelease(t *testing.T) {
	s := &pointerSink{}
	c := NewPointer(func() (Target, error) { return Target{Screen: image.Rect(0, 0, 1024, 768)}, nil }, s)
	if e := c.Handle(PointerEvent{DX: 12, DY: -5, Button: "left", Phase: "down"}); e != nil {
		t.Fatal(e)
	}
	if len(s.packets) != 1 || !s.packets[0].Relative || s.packets[0].X != 12 || !s.packets[0].Down {
		t.Fatal(s.packets)
	}
	if e := c.Close(); e != nil {
		t.Fatal(e)
	}
	if !s.packets[len(s.packets)-1].Up {
		t.Fatal("drag left held")
	}
}
func TestRelativePointerRejectsInvalidMotion(t *testing.T) {
	s := &pointerSink{}
	c := NewPointer(func() (Target, error) { return Target{}, nil }, s)
	if c.Handle(PointerEvent{DX: 99999}) == nil || len(s.packets) != 0 {
		t.Fatal("unbounded motion")
	}
}
