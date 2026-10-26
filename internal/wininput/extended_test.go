package wininput

import (
	"errors"
	"relevo/internal/remoteinput"
	"testing"
	"time"
	"unsafe"
)

type extendedFake struct {
	frames             [][]TouchPacket
	keys               [][]KeyPacket
	touchFail, keyFail bool
}

func (f *extendedFake) Touch(p []TouchPacket) error {
	f.frames = append(f.frames, append([]TouchPacket(nil), p...))
	if f.touchFail {
		return errors.New("touch failed")
	}
	return nil
}
func (f *extendedFake) Keyboard(p []KeyPacket) error {
	f.keys = append(f.keys, append([]KeyPacket(nil), p...))
	if f.keyFail {
		return errors.New("keys failed")
	}
	return nil
}
func contact(id uint32, phase string, x float64) remoteinput.Contact {
	return remoteinput.Contact{ID: id, Phase: phase, X: x, Y: .5}
}
func TestExtendedCompleteFramesAndLastPositionRelease(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	if err := c.HandleContacts([]remoteinput.Contact{contact(200, "down", .2), contact(300, "down", .8)}); err != nil {
		t.Fatal(err)
	}
	if err := c.HandleContacts([]remoteinput.Contact{contact(200, "move", .3)}); err != nil {
		t.Fatal(err)
	}
	if len(f.frames[1]) != 2 || f.frames[1][0].ID != 1 || f.frames[1][1].ID != 2 || f.frames[1][0].Phase != "move" || f.frames[1][1].Phase != "move" {
		t.Fatal(f.frames)
	}
	last := f.frames[1][0]
	if err := c.HandleContacts([]remoteinput.Contact{contact(200, "up", .4)}); err != nil {
		t.Fatal(err)
	}
	if f.frames[2][0].X != last.X || f.frames[2][0].Phase != "up" {
		t.Fatal("UP must use previous injected point", f.frames)
	}
	c.Close()
	if f.frames[len(f.frames)-1][0].Phase != "cancel" {
		t.Fatal(f.frames)
	}
}
func TestExtendedPixelCoordinatesNegativeOrigin(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	c.HandleContacts([]remoteinput.Contact{contact(1, "down", 0)})
	if f.frames[0][0].X != -1024 || f.frames[0][0].Y != 384 {
		t.Fatal(f.frames)
	}
}
func TestExtendedTopologyFailureReleasesContactsAndKeys(t *testing.T) {
	f := &extendedFake{}
	stale := false
	c := NewExtended(func() (Target, error) {
		if stale {
			return Target{}, errors.New("stale")
		}
		return target(), nil
	}, f)
	defer c.Close()
	c.HandleContacts([]remoteinput.Contact{contact(1, "down", .5)})
	c.HandleKey(remoteinput.Key{Kind: "down", Code: "Control"})
	stale = true
	if c.HandleKey(remoteinput.Key{Kind: "text", Text: "abc"}) == nil {
		t.Fatal("stale target accepted")
	}
	if f.frames[len(f.frames)-1][0].Phase != "cancel" || !f.keys[len(f.keys)-1][0].Up {
		t.Fatal("inputs stuck")
	}
}
func TestExtendedUnicodeAndPress(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	if e := c.HandleKey(remoteinput.Key{Kind: "text", Text: "ñ😀"}); e != nil {
		t.Fatal(e)
	}
	p := f.keys[0]
	if len(p) != 6 || p[0].Scan != 0xf1 || p[2].Scan != 0xd83d || p[4].Scan != 0xde00 || !p[1].Up || !p[0].Unicode {
		t.Fatal(p)
	}
	c.HandleKey(remoteinput.Key{Kind: "press", Code: "ArrowLeft"})
	p = f.keys[1]
	if len(p) != 2 || p[0].VK != 0x25 || !p[0].Extended || !p[1].Up {
		t.Fatal(p)
	}
}
func TestExtendedInvalidBeforeTargetAndFailureCleanup(t *testing.T) {
	f := &extendedFake{}
	calls := 0
	c := NewExtended(func() (Target, error) { calls++; return target(), nil }, f)
	defer c.Close()
	if c.HandleContacts([]remoteinput.Contact{contact(1, "move", .5)}) == nil || calls != 0 {
		t.Fatal("invalid event reached target")
	}
	c.Close()
	c = NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	f.keyFail = true
	if c.HandleKey(remoteinput.Key{Kind: "down", Code: "Shift"}) == nil {
		t.Fatal("missing failure")
	}
	if len(f.keys) < 2 || !f.keys[1][0].Up {
		t.Fatal("no failure release", f.keys)
	}
	f.keyFail = false
	if c.Close() != nil {
		t.Fatal("cleanup retry failed")
	}
	if !f.keys[len(f.keys)-1][0].Up {
		t.Fatal(f.keys)
	}
}

func TestExtendedShortcutsPhysicalCodes(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	for _, code := range []string{"KeyC", "KeyV", "KeyA", "Digit0", "Space", "F12"} {
		if err := c.HandleKey(remoteinput.Key{Kind: "press", Code: code}); err != nil {
			t.Fatal(err)
		}
	}
	if f.keys[0][0].VK != 0x43 || f.keys[3][0].VK != 0x30 || f.keys[5][0].VK != 0x7b {
		t.Fatal(f.keys)
	}
}

func TestExtendedABIAndFlags(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("x64 ABI")
	}
	var p winPointerInfo
	var touch winTouchInfo
	var k winKeyboardInput
	if unsafe.Sizeof(p) != 96 || unsafe.Offsetof(p.Performance) != 80 || unsafe.Sizeof(touch) != 144 || unsafe.Offsetof(touch.Contact) != 104 || unsafe.Sizeof(k) != 40 || unsafe.Offsetof(k.VK) != 8 || unsafe.Offsetof(k.Extra) != 24 {
		t.Fatal("incorrect Windows x64 ABI", unsafe.Sizeof(p), unsafe.Sizeof(touch), unsafe.Sizeof(k))
	}
	for phase, flags := range map[string]uint32{"down": 0x10006, "move": 0x20006, "up": 0x40000, "cancel": 0x48000} {
		if got := touchInfo(TouchPacket{ID: 1, Phase: phase, X: -10, Y: 2}); got.Pointer.Flags != flags || got.Pointer.Pixel.X != -10 {
			t.Fatal(got)
		}
	}
	if got := keyboardInfo(KeyPacket{Scan: 0xd83d, Unicode: true, Up: true}); got.Flags != 6 || got.VK != 0 || got.Scan != 0xd83d {
		t.Fatal(got)
	}
}
func TestPartialKeyboardBatchReleasesOnlyUnmatchedDowns(t *testing.T) {
	packets := []KeyPacket{{VK: 0x11}, {Scan: 0xd83d, Unicode: true}, {Scan: 0xd83d, Unicode: true, Up: true}, {Scan: 0xde00, Unicode: true}}
	r := keyReleases(packets)
	if len(r) != 2 || r[0].VK != 0x11 || r[1].Scan != 0xde00 || !r[0].Up || !r[1].Up {
		t.Fatal(r)
	}
}
func TestExtendedIgnoredLetterboxAndTenContacts(t *testing.T) {
	f := &extendedFake{}
	tgt := target()
	tgt.Content.Min.Y = 96
	tgt.Content.Max.Y = 672
	c := NewExtended(func() (Target, error) { return tgt, nil }, f)
	defer c.Close()
	outside := contact(100, "down", .5)
	outside.Y = 0
	if err := c.HandleContacts([]remoteinput.Contact{outside}); err != nil || len(f.frames) != 0 {
		t.Fatal(err, f.frames)
	}
	outside.Phase = "move"
	outside.Y = .5
	c.HandleContacts([]remoteinput.Contact{outside})
	if len(f.frames) != 0 {
		t.Fatal("reentered black-band contact")
	}
	outside.Phase = "up"
	c.HandleContacts([]remoteinput.Contact{outside})
	batch := make([]remoteinput.Contact, 10)
	for i := range batch {
		batch[i] = contact(uint32(100+i), "down", .5)
	}
	if err := c.HandleContacts(batch); err != nil || len(f.frames[0]) != 10 {
		t.Fatal(err, f.frames)
	}
	if err := c.Close(); err != nil || len(f.frames[len(f.frames)-1]) != 10 {
		t.Fatal(err, f.frames)
	}
}
func TestExtendedStationaryHeartbeatAndCloseStopsIt(t *testing.T) {
	// A synchronized backend is necessary because updates originate on a timer.
	f := &heartbeatFake{seen: make(chan []TouchPacket, 8)}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	if err := c.HandleContacts([]remoteinput.Contact{contact(1, "down", .5)}); err != nil {
		t.Fatal(err)
	}
	<-f.seen
	select {
	case frame := <-f.seen:
		if frame[0].Phase != "move" {
			t.Fatal(frame)
		}
	case <-time.After(time.Second):
		t.Fatal("no stationary update")
	}
	c.Close()
	select {
	case frame := <-f.seen:
		if frame[0].Phase != "cancel" {
			t.Fatal(frame)
		}
	case <-time.After(time.Second):
		t.Fatal("no release")
	}
	select {
	case frame := <-f.seen:
		t.Fatal("update after Close", frame)
	case <-time.After(80 * time.Millisecond):
	}
}

type heartbeatFake struct{ seen chan []TouchPacket }

func (f *heartbeatFake) Touch(p []TouchPacket) error {
	f.seen <- append([]TouchPacket(nil), p...)
	return nil
}
func (f *heartbeatFake) Keyboard([]KeyPacket) error { return nil }

func TestExtendedContactReplacementDoesNotExceedInjectionLimit(t *testing.T) {
	f := &extendedFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	defer c.Close()
	batch := make([]remoteinput.Contact, 10)
	for i := range batch {
		batch[i] = contact(uint32(i+1), "down", .5)
	}
	c.HandleContacts(batch)
	if err := c.HandleContacts([]remoteinput.Contact{contact(1, "up", .5), contact(2, "cancel", .5), contact(20, "down", .4), contact(21, "down", .6)}); err != nil {
		t.Fatal(err)
	}
	if len(f.frames) != 3 || len(f.frames[1]) != 10 || len(f.frames[2]) != 10 {
		t.Fatal("replacement must release slots before new downs", f.frames)
	}
}

type pendingFake struct {
	extendedFake
	releaseCalls int
}

func (f *pendingFake) Release() error { f.releaseCalls++; return nil }
func TestExtendedCloseFlushesBackendPartialUnicode(t *testing.T) {
	f := &pendingFake{}
	c := NewExtended(func() (Target, error) { return target(), nil }, f)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if f.releaseCalls != 1 {
		t.Fatal("pending backend input not released")
	}
}
