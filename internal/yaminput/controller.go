// Package yaminput translates the compatible receiver's aggregate gesture and
// committed keyboard events. It does not claim independent multitouch contacts
// where the receiver supplies only a gesture summary.
package yaminput

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"relevo/internal/remoteinput"
	"relevo/internal/wininput"
)

type Options struct{ Touch, Keyboard bool }
type Stats struct{ Handled, Ignored, Errors uint64 }
type panState struct {
	active                     bool
	x, y, fractionX, fractionY float64
}
type zoomState struct {
	active bool
	scale  float64
}
type Controller struct {
	done                                        chan struct{}
	mu                                          sync.Mutex
	target                                      func() (wininput.Target, error)
	mouse                                       wininput.Backend
	backend                                     wininput.ExtendedBackend
	extended                                    *wininput.ExtendedController
	options                                     Options
	keyboard                                    keyboardDecoder
	stats                                       Stats
	drag, rightHeld, closed, broken, haveTarget bool
	pinned                                      wininput.Target
	lastX, lastY                                float64
	pans                                        map[string]panState
	zoom                                        zoomState
}
type inputCleanupError struct{ cause error }

func (e *inputCleanupError) Error() string {
	return "no se pudo liberar por completo la entrada remota"
}
func (e *inputCleanupError) Unwrap() error { return e.cause }
func New(target func() (wininput.Target, error), mouse wininput.Backend, extended wininput.ExtendedBackend, options Options) (*Controller, error) {
	if target == nil || (options.Touch && mouse == nil) || ((options.Touch || options.Keyboard) && extended == nil) {
		return nil, fmt.Errorf("entrada remota incompleta")
	}
	c := &Controller{target: target, mouse: mouse, backend: extended, options: options, pans: map[string]panState{}, done: make(chan struct{}), lastX: .5, lastY: .5}
	if options.Touch || options.Keyboard {
		c.extended = wininput.NewExtended(target, extended)
	}

	if options.Touch {
		go c.maintainMouseRelease()
	}
	return c, nil
}

// A failed secondary-button release must be retried after UAC even when no
// further gesture arrives. Only releases are retried; input events are not.
func (c *Controller) maintainMouseRelease() {
	timer := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-timer.C:
			c.mu.Lock()
			if !c.closed && c.broken && c.rightHeld {
				if c.mouse.Send(wininput.Packet{RightUp: true}) == nil {
					c.rightHeld = false
				}
			}
			c.mu.Unlock()
		}
	}
}
func (c *Controller) Stats() Stats { c.mu.Lock(); defer c.mu.Unlock(); return c.stats }
func (c *Controller) release(final bool) error {
	var err error
	if c.extended != nil {

		if final {
			err = errors.Join(err, c.extended.Close())
		} else {
			err = errors.Join(err, c.extended.RetryRelease())
		}
	}
	if c.rightHeld && c.mouse != nil {
		e := c.mouse.Send(wininput.Packet{RightUp: true})
		err = errors.Join(err, e)
		if e == nil {
			c.rightHeld = false
		}
	}
	if err == nil {
		if final {
			c.extended = nil
		}
		c.keyboard.Reset()
		c.drag = false
		c.haveTarget = false
		c.pans = map[string]panState{}
		c.zoom = zoomState{}
	}
	return err
}
func (c *Controller) fail() { c.stats.Errors++; c.broken = true; c.release(false) }

// CancelActive releases contacts and keys after a rejected event or resize.
// A failed release blocks recreation until a later validated event can retry it.
func (c *Controller) CancelActive() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	err := c.release(false)
	c.broken = true
	if err != nil {
		c.stats.Errors++
		return &inputCleanupError{cause: err}
	}
	return nil
}
func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		close(c.done)
	}
	if err := c.release(true); err != nil {
		c.stats.Errors++
		return &inputCleanupError{cause: err}
	}
	return nil
}
func inputTargetValid(t wininput.Target) error {
	if t.FrameWidth < 2 || t.FrameHeight < 2 || t.FrameWidth > 16384 || t.FrameHeight > 16384 {
		return fmt.Errorf("invalid input target")
	}
	x := (float64(t.Content.Min.X) + float64(t.Content.Dx()-1)/2) / float64(t.FrameWidth-1)
	y := (float64(t.Content.Min.Y) + float64(t.Content.Dy()-1)/2) / float64(t.FrameHeight-1)
	_, _, err := wininput.Coordinates(t, x, y)
	return err
}
func (c *Controller) currentTarget() (wininput.Target, error) {
	t, err := c.target()
	if err != nil {
		return t, fmt.Errorf("remote input target unavailable")
	}
	if err := inputTargetValid(t); err != nil {
		return t, fmt.Errorf("invalid remote input target")
	}
	if c.broken {
		if err := c.release(false); err != nil {
			return t, fmt.Errorf("remote input release pending")
		}
		if err := c.release(true); err != nil {
			return t, fmt.Errorf("remote input release pending")
		}
		c.extended = wininput.NewExtended(c.target, c.backend)
		c.broken = false
	}
	if c.haveTarget && t != c.pinned {
		return t, fmt.Errorf("remote input target changed")
	}
	c.pinned = t
	c.haveTarget = true
	return t, nil
}

// Handle always contains input failures: pointer/keyboard problems must not stop
// video. Counters report them without retaining text, coordinates or raw events.
func (c *Controller) Handle(event map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		c.stats.Ignored++
		return nil
	}
	name, _ := event["eventName"].(string)
	keyboard := name == "KeyboardEvent" || name == "KeyboardPressEvent" || name == "vKeyboardEvent"
	touch := name == "SingleTap" || name == "SingleTap2nd" || name == "TwoFingerSingleTap" || name == "Dragged" || name == "HoverEvent" || name == "Scrolled" || name == "TwoFingerPanned" || name == "Zoomed"
	if (!keyboard && !touch) || (keyboard && !c.options.Keyboard) || (touch && !c.options.Touch) {
		c.stats.Ignored++
		return nil
	}
	t, err := c.currentTarget()
	if err != nil {
		c.fail()
		return nil
	}
	handled, err := c.handle(event, name, t, keyboard)
	if err != nil {
		c.fail()
	} else if handled {
		c.stats.Handled++
	} else {
		c.stats.Ignored++
	}
	return nil
}
func (c *Controller) handle(event map[string]interface{}, name string, t wininput.Target, keyboard bool) (bool, error) {
	if keyboard {
		keys, handled, err := c.keyboard.Decode(event)
		if err != nil || !handled {
			return handled, err
		}
		for _, key := range keys {
			if err := c.extended.HandleKey(key); err != nil {
				return true, fmt.Errorf("remote keyboard injection failed")
			}
		}
		return true, nil
	}
	switch name {
	case "Scrolled", "TwoFingerPanned":
		return c.pan(event, name, t)
	case "Zoomed":
		return c.pinch(event, t)
	}
	if name == "TwoFingerSingleTap" {
		if _, hasX := event["x"]; !hasX {
			if _, hasY := event["y"]; !hasY {
				event = map[string]interface{}{"x": c.lastX, "y": c.lastY}
			}
		}
	}
	x, okX := inputNumber(event["x"])
	y, okY := inputNumber(event["y"])
	if !okX || !okY || x < 0 || x > 1 || y < 0 || y > 1 {
		return true, fmt.Errorf("invalid remote pointer coordinates")
	}
	ax, ay, err := wininput.Coordinates(t, x, y)
	if errors.Is(err, wininput.ErrOutsideContent) {
		if c.drag {
			return true, fmt.Errorf("drag left visible content")
		}
		return false, nil
	}
	if err != nil {
		return true, fmt.Errorf("invalid remote pointer target")
	}
	c.lastX, c.lastY = x, y
	switch name {
	case "HoverEvent":
		if c.drag {
			return false, nil
		}
		return true, c.mouse.Send(wininput.Packet{Move: true, X: ax, Y: ay})
	case "SingleTap":
		if c.drag {
			return true, fmt.Errorf("tap during active drag")
		}
		if err := c.extended.HandleContacts([]remoteinput.Contact{{ID: 1, Phase: "down", X: x, Y: y}}); err != nil {
			return true, err
		}
		return true, c.extended.HandleContacts([]remoteinput.Contact{{ID: 1, Phase: "up", X: x, Y: y}})
	case "SingleTap2nd", "TwoFingerSingleTap":
		if c.drag {
			return true, fmt.Errorf("secondary click during active drag")
		}
		c.rightHeld = true
		if err := c.mouse.Send(wininput.Packet{Move: true, X: ax, Y: ay, RightDown: true}); err != nil {
			return true, err
		}
		if err := c.mouse.Send(wininput.Packet{RightUp: true}); err != nil {
			return true, err
		}
		c.rightHeld = false
		return true, nil
	case "Dragged":
		start, ok := inputBool(event, "startOfDrag")
		if !ok {
			return true, fmt.Errorf("invalid remote drag phase")
		}
		end, ok := inputBool(event, "endOfDrag")
		if !ok {
			return true, fmt.Errorf("invalid remote drag phase")
		}
		if start {
			if c.drag {
				return true, fmt.Errorf("duplicate remote drag start")
			}
			if err := c.extended.HandleContacts([]remoteinput.Contact{{ID: 1, Phase: "down", X: x, Y: y}}); err != nil {
				return true, err
			}
			c.drag = true
		}
		if !c.drag {
			return false, nil
		}
		phase := "move"
		if end {
			phase = "up"
		} else if start {
			return true, nil
		}
		if err := c.extended.HandleContacts([]remoteinput.Contact{{ID: 1, Phase: phase, X: x, Y: y}}); err != nil {
			return true, err
		}
		if end {
			c.drag = false
		}
		return true, nil
	}
	return false, nil
}
func inputNumber(value interface{}) (float64, bool) {
	var n float64
	switch v := value.(type) {
	case float64:
		n = v
	case float32:
		n = float64(v)
	case int:
		n = float64(v)
	case int8:
		n = float64(v)
	case int16:
		n = float64(v)
	case int32:
		n = float64(v)
	case int64:
		n = float64(v)
	case uint:
		n = float64(v)
	case uint8:
		n = float64(v)
	case uint16:
		n = float64(v)
	case uint32:
		n = float64(v)
	case uint64:
		n = float64(v)
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}
func inputBool(event map[string]interface{}, key string) (bool, bool) {
	value, present := event[key]
	if !present {
		return false, true
	}
	b, ok := value.(bool)
	return b, ok
}
func (c *Controller) anchor(t wininput.Target) error {
	x, y, err := wininput.Coordinates(t, c.lastX, c.lastY)
	if err != nil {
		return fmt.Errorf("remote gesture anchor unavailable")
	}
	return c.mouse.Send(wininput.Packet{Move: true, X: x, Y: y})
}
func (c *Controller) pan(event map[string]interface{}, name string, t wininput.Target) (bool, error) {
	x, okX := inputNumber(event["x"])
	y, okY := inputNumber(event["y"])
	if !okX || !okY || math.Abs(x) > 64 || math.Abs(y) > 64 {
		return true, fmt.Errorf("invalid remote pan translation")
	}
	startKey, endKey := "startOfScroll", "endOfScroll"
	if name == "TwoFingerPanned" {
		startKey, endKey = "startOfPan", "endOfPan"
	}
	start, ok := inputBool(event, startKey)
	if !ok {
		return true, fmt.Errorf("invalid remote pan phase")
	}
	end, ok := inputBool(event, endKey)
	if !ok {
		return true, fmt.Errorf("invalid remote pan phase")
	}
	if name == "Scrolled" {
		if _, present := event[startKey]; !present {
			start, ok = inputBool(event, "startOfPan")
			if !ok {
				return true, fmt.Errorf("invalid remote pan phase")
			}
		}
		if _, present := event[endKey]; !present {
			end, ok = inputBool(event, "endOfPan")
			if !ok {
				return true, fmt.Errorf("invalid remote pan phase")
			}
		}
	}
	if c.drag {
		return true, fmt.Errorf("scroll during active drag")
	}
	state := c.pans[name]
	if start || !state.active {
		c.pans[name] = panState{active: !end, x: x, y: y}
		return true, nil
	}
	// Receiver translations are cumulative normalized distances, not successive
	// wheel deltas. Preserve sub-unit fractions, with no jump at gesture start.
	state.fractionX += -(x - state.x) * float64(t.FrameWidth) * 120 / 48
	state.fractionY += (y - state.y) * float64(t.FrameHeight) * 120 / 48
	state.x, state.y = x, y
	for _, axis := range []struct {
		fraction   *float64
		horizontal bool
	}{{&state.fractionY, false}, {&state.fractionX, true}} {
		delta := math.Trunc(*axis.fraction)
		if delta > 1200 {
			delta = 1200
		}
		if delta < -1200 {
			delta = -1200
		}
		if delta != 0 {
			if err := c.anchor(t); err != nil {
				return true, err
			}
			if err := c.mouse.Send(wininput.Packet{Relative: true, HorizontalScroll: axis.horizontal, Scroll: int32(delta)}); err != nil {
				return true, err
			}
		}
		// Drop an excessive remainder rather than replaying a large old movement.
		*axis.fraction -= math.Trunc(*axis.fraction)
	}
	state.active = !end
	c.pans[name] = state
	return true, nil
}
func (c *Controller) pinch(event map[string]interface{}, t wininput.Target) (bool, error) {
	scale, ok := inputNumber(event["scale"])
	if _, present := event["scale"]; !present {
		zoom, valid := inputNumber(event["zoom"])
		if valid && (zoom == 1 || zoom == 2) {
			delta := int32(120)
			if zoom == 2 {
				delta = -120
			}
			return c.zoomWheel(t, delta)
		}
	}
	if !ok || scale < 0.01 || scale > 100 {
		return true, fmt.Errorf("invalid remote zoom scale")
	}
	start, ok := inputBool(event, "startOfZoom")
	if !ok {
		return true, fmt.Errorf("invalid remote zoom phase")
	}
	end, ok := inputBool(event, "endOfZoom")
	if !ok {
		return true, fmt.Errorf("invalid remote zoom phase")
	}
	if start || !c.zoom.active {
		c.zoom = zoomState{active: !end, scale: scale}
		return true, nil
	}
	steps := int(math.Trunc(math.Log(scale/c.zoom.scale) / math.Log(1.12)))
	if steps > 8 {
		steps = 8
	}
	if steps < -8 {
		steps = -8
	}
	if steps != 0 {
		if _, err := c.zoomWheel(t, int32(steps*120)); err != nil {
			return true, err
		}
		c.zoom.scale *= math.Pow(1.12, float64(steps))
	}
	c.zoom.active = !end
	return true, nil
}

func (c *Controller) zoomWheel(t wininput.Target, delta int32) (bool, error) {
	if c.drag {
		return true, fmt.Errorf("zoom during active drag")
	}
	if err := c.anchor(t); err != nil {
		return true, err
	}
	held := c.keyboard.Held("Control")
	if !held {
		if err := c.extended.HandleKey(remoteinput.Key{Kind: "down", Code: "Control"}); err != nil {
			return true, err
		}
	}
	if err := c.mouse.Send(wininput.Packet{Relative: true, Scroll: delta}); err != nil {
		return true, err
	}
	if !held {
		if err := c.extended.HandleKey(remoteinput.Key{Kind: "up", Code: "Control"}); err != nil {
			return true, err
		}
	}
	return true, nil
}
