package yamkeyboard

import (
	"image"
	"relevo/internal/wininput"
	"sync/atomic"
	"testing"
	"time"
)

func TestContextualKeyboardAndSessionCleanup(t *testing.T) {
	var shown, closed, probed atomic.Int32
	var editable atomic.Bool
	target := func() (wininput.Target, error) {
		return wininput.Target{Screen: image.Rect(-1024, 0, 0, 768), Virtual: image.Rect(-1024, 0, 1920, 1080), Content: image.Rect(0, 0, 1024, 768), FrameWidth: 1024, FrameHeight: 768}, nil
	}
	m := New(target, func(x, y int32) (bool, error) {
		probed.Add(1)
		if x >= 0 || y < 0 {
			t.Error("physical coordinates lost")
		}
		return editable.Load(), nil
	}, func(wininput.Target) (func(), error) { shown.Add(1); return func() { closed.Add(1) }, nil }, nil)
	m.Tap(.2, .3)
	time.Sleep(130 * time.Millisecond)
	if shown.Load() != 0 || probed.Load() == 0 {
		t.Fatal("non-editable control opened keyboard or probe absent")
	}
	editable.Store(true)
	m.Tap(.2, .3)
	time.Sleep(130 * time.Millisecond)
	if shown.Load() != 1 {
		t.Fatal("editable control did not show keyboard")
	}
	m.Close()
	m.Close()
	m.Tap(.2, .3)
	m.Manual()
	time.Sleep(100 * time.Millisecond)
	if shown.Load() != 1 || closed.Load() != 1 {
		t.Fatal("panel outlived session", shown.Load(), closed.Load())
	}
}
func TestCancelInvalidatesPendingTap(t *testing.T) {
	var calls atomic.Int32
	m := New(func() (wininput.Target, error) {
		return wininput.Target{Screen: image.Rect(0, 0, 1024, 768), Virtual: image.Rect(0, 0, 1920, 1080), Content: image.Rect(0, 0, 1024, 768), FrameWidth: 1024, FrameHeight: 768}, nil
	}, func(int32, int32) (bool, error) { calls.Add(1); return true, nil }, func(wininput.Target) (func(), error) { t.Error("cancelled tap opened panel"); return func() {}, nil }, nil)
	m.Tap(.5, .5)
	m.Cancel()
	time.Sleep(130 * time.Millisecond)
	m.Close()
	if calls.Load() != 0 {
		t.Fatal("cancelled tap queried focus")
	}
}
