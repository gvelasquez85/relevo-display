package yaminput

import (
	"relevo/internal/wininput"
	"testing"
)

func TestKeyboardFocusOnlyAfterSuccessfulSingleTap(t *testing.T) {
	f := &inputFake{}
	calls := 0
	c, err := New(func() (wininput.Target, error) { return inputGeometry(), nil }, f, f, Options{Touch: true, Keyboard: true, Tap: func(x, y float64) {
		calls++
		if x != .2 || y != .3 {
			t.Fatal("tap coordinates changed")
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	emit(c, "SingleTap", point(.2, .3))
	if calls != 1 {
		t.Fatal("successful tap did not request contextual keyboard")
	}
	emit(c, "TwoFingerSingleTap", point(.2, .3))
	emit(c, "SingleTap", point(-1, .3))
	f.failTouch = true
	emit(c, "SingleTap", point(.2, .3))
	if calls != 1 {
		t.Fatal("failed input or multi-finger gesture requested keyboard")
	}
}
func TestKeyboardFocusDisabledWithKeyboardOption(t *testing.T) {
	f := &inputFake{}
	c, err := New(func() (wininput.Target, error) { return inputGeometry(), nil }, f, f, Options{Touch: true, Tap: func(float64, float64) { t.Fatal("disabled keyboard requested") }})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	emit(c, "SingleTap", point(.2, .3))
}
