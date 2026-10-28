package yaminput

import (
	"errors"
	"image"
	"relevo/internal/wininput"
	"sync"
	"sync/atomic"
	"testing"
)

type inputFake struct {
	mu                   sync.Mutex
	frames               [][]wininput.TouchPacket
	keys                 []wininput.KeyPacket
	mouse                []wininput.Packet
	failTouch, failMouse bool
}

func (f *inputFake) Touch(p []wininput.TouchPacket) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.frames = append(f.frames, append([]wininput.TouchPacket(nil), p...))
	if f.failTouch {
		return errors.New("private touch detail")
	}
	return nil
}
func (f *inputFake) Keyboard(p []wininput.KeyPacket) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, p...)
	return nil
}
func (f *inputFake) Send(p wininput.Packet) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mouse = append(f.mouse, p)
	if f.failMouse {
		return errors.New("private mouse detail")
	}
	return nil
}
func inputGeometry() wininput.Target {
	return wininput.Target{Screen: image.Rect(-1024, 0, 0, 768), Content: image.Rect(0, 0, 1024, 768), Virtual: image.Rect(-1024, 0, 1920, 1080), FrameWidth: 1024, FrameHeight: 768}
}
func newInput(t *testing.T) (*Controller, *inputFake) {
	t.Helper()
	f := &inputFake{}
	c, e := New(func() (wininput.Target, error) { return inputGeometry(), nil }, f, f, Options{Touch: true, Keyboard: true})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c, f
}
func emit(c *Controller, name string, fields map[string]interface{}) {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["eventName"] = name
	c.Handle(fields)
}
func point(x, y float64) map[string]interface{} { return map[string]interface{}{"x": x, "y": y} }
func TestTapAndDragUseNativeTouchAndRelease(t *testing.T) {
	c, f := newInput(t)
	emit(c, "SingleTap", point(0, .5))
	f.mu.Lock()
	if len(f.frames) != 2 || f.frames[0][0].Phase != "down" || f.frames[0][0].X != -1024 || f.frames[1][0].Phase != "up" {
		t.Fatal(f.frames)
	}
	f.mu.Unlock()
	emit(c, "Dragged", point(.2, .5)) // no invented down for orphan motion
	if c.Stats().Ignored != 1 {
		t.Fatal(c.Stats())
	}
	p := point(.2, .5)
	p["startOfDrag"] = true
	emit(c, "Dragged", p)
	emit(c, "Dragged", point(.4, .5))
	p = point(.6, .5)
	p["endOfDrag"] = true
	emit(c, "Dragged", p)
	f.mu.Lock()
	defer f.mu.Unlock()
	n := len(f.frames)
	if f.frames[n-1][0].Phase != "up" || f.frames[n-1][0].X != f.frames[n-2][0].X {
		t.Fatal("drag not released at previous injected point")
	}
}
func TestMalformedDragCancelsAndNextTapRecovers(t *testing.T) {
	c, f := newInput(t)
	p := point(.2, .3)
	p["startOfDrag"] = true
	emit(c, "Dragged", p)
	emit(c, "Dragged", map[string]interface{}{"x": "private", "y": .3, "endOfDrag": true})
	if c.Stats().Errors != 1 {
		t.Fatal(c.Stats())
	}
	f.mu.Lock()
	if f.frames[len(f.frames)-1][0].Phase != "cancel" {
		t.Fatal("stuck contact")
	}
	f.mu.Unlock()
	emit(c, "SingleTap", point(.5, .5))
	if c.Stats().Handled != 2 {
		t.Fatal(c.Stats())
	}
}
func TestScrollCumulativeBaselineDirectionAndAnchor(t *testing.T) {
	c, f := newInput(t)
	emit(c, "Scrolled", map[string]interface{}{"x": .2, "y": .2, "startOfPan": true})
	emit(c, "Scrolled", point(.2, .3))
	emit(c, "Scrolled", point(.2, .3))
	f.mu.Lock()
	defer f.mu.Unlock()
	scroll := []wininput.Packet{}
	for _, p := range f.mouse {
		if p.Scroll != 0 {
			scroll = append(scroll, p)
		}
	}
	if len(scroll) != 1 || scroll[0].Scroll < 191 || scroll[0].Scroll > 192 {
		t.Fatal("cumulative pan or natural Y direction wrong", scroll)
	}
	if len(f.mouse) < 2 || !f.mouse[0].Move {
		t.Fatal("wheel not anchored to captured display")
	}
}
func TestZoomPreservesHeldControlAndReleasesTemporaryControl(t *testing.T) {
	for _, held := range []bool{false, true} {
		t.Run(map[bool]string{false: "temporary", true: "held"}[held], func(t *testing.T) {
			c, f := newInput(t)
			if held {
				emit(c, "KeyboardPressEvent", map[string]interface{}{"keyCode": uint64(0xe0), "pressed": true})
			}
			emit(c, "Zoomed", map[string]interface{}{"scale": 1.0, "startOfZoom": true})
			emit(c, "Zoomed", map[string]interface{}{"scale": 1.25})
			f.mu.Lock()
			ups, downs := 0, 0
			for _, p := range f.keys {
				if p.VK == 0x11 {
					if p.Up {
						ups++
					} else {
						downs++
					}
				}
			}
			f.mu.Unlock()
			if downs != 1 || (!held && ups != 1) || (held && ups != 0) {
				t.Fatalf("Control held=%t down=%d up=%d", held, downs, ups)
			}
			if held {
				emit(c, "KeyboardPressEvent", map[string]interface{}{"keyCode": uint64(0xe0), "pressed": false})
			}
		})
	}
}
func TestSecondaryTapWithoutCoordinatesAndZoomFallback(t *testing.T) {
	c, f := newInput(t)
	emit(c, "TwoFingerSingleTap", nil)
	emit(c, "Zoomed", map[string]interface{}{"zoom": 1})
	if c.Stats().Errors != 0 || c.Stats().Handled != 2 {
		t.Fatal(c.Stats())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	down, up, zoom := false, false, false
	for _, p := range f.mouse {
		down = down || p.RightDown
		up = up || p.RightUp
		zoom = zoom || p.Scroll == 120
	}
	if !down || !up || !zoom {
		t.Fatal(f.mouse)
	}
}
func TestCloseAndRejectedEventReleaseTouchAndKeyboard(t *testing.T) {
	c, f := newInput(t)
	p := point(.2, .3)
	p["startOfDrag"] = true
	emit(c, "Dragged", p)
	emit(c, "KeyboardPressEvent", map[string]interface{}{"keyCode": uint64(0xe1), "pressed": true})
	if e := c.CancelActive(); e != nil {
		t.Fatal(e)
	}
	f.mu.Lock()
	if f.frames[len(f.frames)-1][0].Phase != "cancel" || !f.keys[len(f.keys)-1].Up {
		t.Fatal("cancel left input held")
	}
	f.mu.Unlock()
	emit(c, "KeyboardPressEvent", map[string]interface{}{"keyCode": uint64(0xe0), "pressed": true})
	if e := c.Close(); e != nil {
		t.Fatal(e)
	}
	f.mu.Lock()
	if !f.keys[len(f.keys)-1].Up {
		t.Fatal("close left modifier held")
	}
	count := len(f.keys)
	f.mu.Unlock()
	emit(c, "KeyboardPressEvent", map[string]interface{}{"keyCode": uint64(4), "pressed": true})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.keys) != count {
		t.Fatal("input after close")
	}
}
func TestTargetFailureContainedAndRecoveryNeverReplays(t *testing.T) {
	f := &inputFake{}
	var blocked atomic.Bool
	c, e := New(func() (wininput.Target, error) {
		if blocked.Load() {
			return wininput.Target{}, errors.New("private target")
		}
		return inputGeometry(), nil
	}, f, f, Options{Touch: true, Keyboard: true})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	blocked.Store(true)
	if err := c.Handle(map[string]interface{}{"eventName": "vKeyboardEvent", "inputString": "private"}); err != nil {
		t.Fatal("video would stop", err)
	}
	blocked.Store(false)
	emit(c, "vKeyboardEvent", map[string]interface{}{"inputString": "x"})
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.keys) != 2 || f.keys[0].Scan != 'x' {
		t.Fatal("failed text replayed")
	}
	if c.Stats().Errors != 1 {
		t.Fatal(c.Stats())
	}
}
func TestOptionsDisableInjection(t *testing.T) {
	f := &inputFake{}
	c, e := New(func() (wininput.Target, error) {
		t.Fatal("disabled input queried target")
		return wininput.Target{}, nil
	}, f, f, Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	emit(c, "SingleTap", point(.2, .3))
	emit(c, "vKeyboardEvent", map[string]interface{}{"inputString": "private"})
	if c.Stats().Ignored != 2 {
		t.Fatal(c.Stats())
	}
}

func TestPreloginPointerDoesNotDependOnTouchInjection(t *testing.T) {
	f := &inputFake{failTouch: true}
	c, err := New(func() (wininput.Target, error) { return inputGeometry(), nil }, f, f, Options{Touch: true, Keyboard: true, PointerMouse: true})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	emit(c, "SingleTap", point(.5, .5))
	if c.Stats().Errors != 0 {
		t.Fatal(c.Stats())
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.frames) != 0 || len(f.mouse) != 2 || !f.mouse[0].Down || !f.mouse[1].Up {
		t.Fatal("login click depended on native touch", f.frames, f.mouse)
	}
}
