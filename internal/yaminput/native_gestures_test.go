package yaminput

import (
	"relevo/internal/wininput"
	"testing"
)

func newNativeInput(t *testing.T) (*Controller, *inputFake) {
	t.Helper()
	f := &inputFake{}
	c, e := New(func() (wininput.Target, error) { return inputGeometry(), nil }, f, f, Options{Touch: true, Keyboard: true, NativeGestures: true})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c, f
}
func TestNativePinchPairAndEnd(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "Zoomed", map[string]interface{}{"scale": 1., "startOfZoom": true})
	emit(c, "Zoomed", map[string]interface{}{"scale": 2.})
	emit(c, "Zoomed", map[string]interface{}{"scale": 2., "endOfZoom": true})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.frames) != 3 {
		t.Fatal(f.frames)
	}
	for _, frame := range f.frames {
		if len(frame) != 2 {
			t.Fatal(frame)
		}
	}
	if f.frames[0][0].Phase != "down" || f.frames[1][0].Phase != "move" || f.frames[2][0].Phase != "up" {
		t.Fatal(f.frames)
	}
	if f.frames[1][1].X-f.frames[1][0].X <= f.frames[0][1].X-f.frames[0][0].X {
		t.Fatal("pinch did not expand")
	}
	if len(f.mouse) != 0 || len(f.keys) != 0 {
		t.Fatal("native gesture emitted wheel or modifiers")
	}
}
func TestNativePanAmbiguityCancels(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "TwoFingerPanned", map[string]interface{}{"x": 0., "y": 0., "startOfPan": true})
	emit(c, "TwoFingerPanned", point(.1, .1))
	emit(c, "Zoomed", map[string]interface{}{"scale": 1., "startOfZoom": true})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.frames) < 3 || f.frames[len(f.frames)-1][0].Phase != "cancel" {
		t.Fatal(f.frames)
	}
	if len(f.mouse) != 0 || len(f.keys) != 0 {
		t.Fatal("unexpected legacy injection")
	}
}
func TestNativeOrphanAndDiscreteZoomDoNotInventTouches(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "TwoFingerPanned", point(.1, .1))
	emit(c, "Zoomed", map[string]interface{}{"zoom": 1})
	if len(f.frames) != 0 || len(f.mouse) != 0 || len(f.keys) != 0 {
		t.Fatal("invented gesture")
	}
}
func TestNativeCancelAndTapRecovery(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "Zoomed", map[string]interface{}{"scale": 1., "startOfZoom": true})
	if e := c.CancelActive(); e != nil {
		t.Fatal(e)
	}
	emit(c, "SingleTap", point(.5, .5))
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.frames[1][0].Phase != "cancel" || len(f.frames[len(f.frames)-1]) != 1 {
		t.Fatal(f.frames)
	}
}

func TestNativePairBoundsAndResize(t *testing.T) {
	f := &inputFake{}
	target := inputGeometry()
	target.Content.Min.X = 100
	target.Content.Max.X = 924
	c, e := New(func() (wininput.Target, error) { return target, nil }, f, f, Options{Touch: true, NativeGestures: true})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	emit(c, "HoverEvent", point(.5, .5))
	emit(c, "TwoFingerPanned", map[string]interface{}{"x": 0., "y": 0., "startOfPan": true})
	emit(c, "TwoFingerPanned", point(64, 64))
	f.mu.Lock()
	frame := append([]wininput.TouchPacket(nil), f.frames[len(f.frames)-1]...)
	f.mu.Unlock()
	if len(frame) != 2 || frame[0].X < -1024 || frame[1].X > -1 || frame[0].Y < 0 || frame[0].Y > 767 {
		t.Fatal(frame)
	}
	target.FrameWidth = 2048
	emit(c, "TwoFingerPanned", point(0, 0))
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.frames[len(f.frames)-1][0].Phase != "cancel" {
		t.Fatal(f.frames)
	}
}
func TestNativeMalformedScaleCancels(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "Zoomed", map[string]interface{}{"scale": 1., "startOfZoom": true})
	emit(c, "Zoomed", map[string]interface{}{"scale": "invalid"})
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.frames[len(f.frames)-1][0].Phase != "cancel" {
		t.Fatal(f.frames)
	}
}

func TestNativePinchUsesReceivedGestureCenter(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "Zoomed", map[string]interface{}{"scale": 1., "startOfZoom": true, "x": .2, "y": .3})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.frames) != 1 || len(f.frames[0]) != 2 {
		t.Fatal("missing pair")
	}
	target := inputGeometry()
	centerX := (f.frames[0][0].X + f.frames[0][1].X) / 2
	centerY := f.frames[0][0].Y
	wantX := int32(target.Screen.Min.X) + int32(.2*float64(target.Screen.Dx()-1))
	wantY := int32(target.Screen.Min.Y) + int32(.3*float64(target.Screen.Dy()-1))
	if abs32(centerX-wantX) > 1 || abs32(centerY-wantY) > 1 {
		t.Fatal("pinch anchored to stale cursor", centerX, centerY, wantX, wantY)
	}
}
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}
func TestNativeScrollTouchTypeThreeRoutesPair(t *testing.T) {
	c, f := newNativeInput(t)
	emit(c, "Scrolled", map[string]interface{}{"x": 0., "y": 0., "touchType": "3", "startOfScroll": true})
	emit(c, "Scrolled", map[string]interface{}{"x": .1, "y": .1, "touchType": "3", "endOfScroll": true})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.frames) != 2 || len(f.frames[0]) != 2 || f.frames[1][0].Phase != "up" {
		t.Fatal("confirmed two-finger scroll stayed wheel", f.frames)
	}
	if len(f.mouse) != 0 {
		t.Fatal("native scroll emitted wheel")
	}
}
