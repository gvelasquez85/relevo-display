package wininput

import (
	"errors"
	"testing"

	"relevo/internal/remoteinput"
)

func TestPhysicalCoordinatesExport(t *testing.T) {
	x, y, e := PhysicalCoordinates(target(), 0, .5)
	if e != nil || x != -1024 || y != 384 {
		t.Fatalf("%d %d %v", x, y, e)
	}
	if _, _, e = PhysicalCoordinates(Target{}, .5, .5); e == nil {
		t.Fatal("invalid target accepted")
	}
}

func TestCancelActiveKeepsControllerUsable(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	if e := c.HandleContacts([]remoteinput.Contact{contact(10, "down", .5)}); e != nil {
		t.Fatal(e)
	}
	if e := c.HandleKey(remoteinput.Key{Kind: "down", Code: "Control"}); e != nil {
		t.Fatal(e)
	}
	if e := c.CancelActive(); e != nil {
		t.Fatal(e)
	}
	if len(c.active) != 0 || len(c.held) != 0 {
		t.Fatal("input still active")
	}
	if f.frames[len(f.frames)-1][0].Phase != "cancel" || !f.keys[len(f.keys)-1][0].Up {
		t.Fatal("release not injected")
	}
	if e := c.HandleContacts([]remoteinput.Contact{contact(10, "down", .5)}); e != nil {
		t.Fatal("controller cannot be reused", e)
	}
}

func TestCancelActiveClosedAndFailed(t *testing.T) {
	c := NewExtended(func() (Target, error) { return target(), nil }, &extendedFake{})
	c.Close()
	if c.CancelActive() == nil {
		t.Fatal("closed accepted")
	}
	f := &extendedFake{}
	c = NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	problem := errors.New("prior failure")
	c.mu.Lock()
	c.failure = problem
	c.mu.Unlock()
	if !errors.Is(c.CancelActive(), problem) {
		t.Fatal("failure not preserved")
	}
}

func TestCancelActiveReleaseFailureIsSticky(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	if e := c.HandleContacts([]remoteinput.Contact{contact(10, "down", .5)}); e != nil {
		t.Fatal(e)
	}
	c.mu.Lock()
	f.touchFail = true
	c.mu.Unlock()
	if c.CancelActive() == nil {
		t.Fatal("release failure ignored")
	}
	if c.HandleKey(remoteinput.Key{Kind: "text", Text: "x"}) == nil {
		t.Fatal("input accepted after failed release")
	}
}
