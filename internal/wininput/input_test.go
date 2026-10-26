package wininput

import (
	"errors"
	"image"
	"math"
	"testing"
)

type fake struct{ packets []Packet }

func (f *fake) Send(p Packet) error { f.packets = append(f.packets, p); return nil }
func target() Target {
	return Target{Screen: image.Rect(-1024, 0, 0, 768), Content: image.Rect(0, 0, 1024, 768), Virtual: image.Rect(-1024, 0, 1920, 1080), FrameWidth: 1024, FrameHeight: 768}
}
func TestNegativeVirtualOrigin(t *testing.T) {
	x, y, e := Coordinates(target(), 0, 0)
	if e != nil || x != 0 || y != 0 {
		t.Fatal(x, y, e)
	}
	x, _, e = Coordinates(target(), 1, 1)
	if e != nil || x != int32(math.Round(1023.0*65535/2943)) {
		t.Fatal(x, e)
	}
}
func TestReleaseOnCancelDisconnect(t *testing.T) {
	for _, phase := range []string{"cancel", "close", "up"} {
		f := &fake{}
		c := New(func() (Target, error) { return target(), nil }, f)
		if e := c.Handle(Event{"down", .5, .5}); e != nil {
			t.Fatal(e)
		}
		if phase == "close" {
			c.Close()
		} else {
			c.Handle(Event{phase, .5, .5})
		}
		if len(f.packets) != 2 || !f.packets[1].Up {
			t.Fatal(f.packets)
		}
		if phase != "up" && f.packets[1].Move {
			t.Fatal("release moved pointer")
		}
		c.Close()
		if len(f.packets) != 2 {
			t.Fatal("double release")
		}
	}
}
func TestInvalidBeforeTarget(t *testing.T) {
	f := &fake{}
	calls := 0
	c := New(func() (Target, error) { calls++; return target(), nil }, f)
	if c.Handle(Event{"down", math.NaN(), 0}) == nil || calls != 0 || len(f.packets) != 0 {
		t.Fatal("invalid event reached OS")
	}
}
func TestTopologyFailureReleases(t *testing.T) {
	f := &fake{}
	stale := false
	c := New(func() (Target, error) {
		if stale {
			return Target{}, errors.New("stale")
		}
		return target(), nil
	}, f)
	c.Handle(Event{"down", .5, .5})
	stale = true
	if c.Handle(Event{"move", .6, .6}) == nil || len(f.packets) != 2 || !f.packets[1].Up || f.packets[1].Move {
		t.Fatal(f.packets)
	}
}

func TestLetterboxTouchIgnoredWithoutDisconnect(t *testing.T) {
	f := &fake{}
	tgt := target()
	tgt.Content = image.Rect(0, 96, 1024, 672)
	c := New(func() (Target, error) { return tgt, nil }, f)
	if err := c.Handle(Event{"down", .5, 0}); err != nil || len(f.packets) != 0 {
		t.Fatalf("black band ended session: %v", err)
	}
	if err := c.Handle(Event{"down", .5, .5}); err != nil {
		t.Fatal(err)
	}
	if err := c.Handle(Event{"move", .5, 0}); err != nil {
		t.Fatal(err)
	}
	if len(f.packets) != 2 || !f.packets[1].Up || f.packets[1].Move {
		t.Fatal("leaving image did not safely release", f.packets)
	}
	if err := c.Handle(Event{"up", .5, 0}); err != nil {
		t.Fatal(err)
	}
	if err := c.Handle(Event{"down", .5, .5}); err != nil {
		t.Fatal("next gesture failed", err)
	}
	c.Close()
}
