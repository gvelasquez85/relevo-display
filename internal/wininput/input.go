// Package wininput translates explicitly enabled single-finger input.
package wininput

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
)

var ErrOutsideContent = errors.New("touch outside displayed content")

type Target struct {
	Screen, Content, Virtual image.Rectangle
	FrameWidth, FrameHeight  int
}
type Event struct {
	Phase string
	X, Y  float64
}
type Packet struct {
	X, Y                                           int32
	Move, Down, Up                                 bool
	Relative, RightDown, RightUp, HorizontalScroll bool
	Scroll                                         int32
}
type Backend interface{ Send(Packet) error }
type Controller struct {
	mu           sync.Mutex
	target       func() (Target, error)
	backend      Backend
	held, closed bool
}

func New(target func() (Target, error), backend Backend) *Controller {
	return &Controller{target: target, backend: backend}
}
func Coordinates(t Target, x, y float64) (int32, int32, error) {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) || x < 0 || x > 1 || y < 0 || y > 1 {
		return 0, 0, fmt.Errorf("invalid normalized coordinates")
	}
	if t.Screen.Empty() || (t.Content.Dx() < 2 || t.Content.Dy() < 2) || t.Virtual.Dx() < 2 || t.Virtual.Dy() < 2 || t.FrameWidth < 2 || t.FrameHeight < 2 || !t.Screen.In(t.Virtual) {
		return 0, 0, fmt.Errorf("invalid input target")
	}
	px, py := x*float64(t.FrameWidth-1), y*float64(t.FrameHeight-1)
	if px < float64(t.Content.Min.X) || px > float64(t.Content.Max.X-1) || py < float64(t.Content.Min.Y) || py > float64(t.Content.Max.Y-1) {
		return 0, 0, ErrOutsideContent
	}
	sx := float64(t.Screen.Min.X) + (px-float64(t.Content.Min.X))*float64(t.Screen.Dx()-1)/float64(t.Content.Dx()-1)
	sy := float64(t.Screen.Min.Y) + (py-float64(t.Content.Min.Y))*float64(t.Screen.Dy()-1)/float64(t.Content.Dy()-1)
	return int32(math.Round((sx - float64(t.Virtual.Min.X)) * 65535 / float64(t.Virtual.Dx()-1))), int32(math.Round((sy - float64(t.Virtual.Min.Y)) * 65535 / float64(t.Virtual.Dy()-1))), nil
}
func (c *Controller) release() error {
	if !c.held {
		return nil
	}
	e := c.backend.Send(Packet{Up: true})
	if e == nil {
		c.held = false
	}
	return e
}
func (c *Controller) Handle(e Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("input closed")
	}
	if e.Phase == "cancel" {
		return c.release()
	}
	if e.Phase != "down" && e.Phase != "move" && e.Phase != "up" {
		return errors.Join(fmt.Errorf("invalid phase"), c.release())
	}
	// Validate before any coordinate-dependent OS operation.
	if math.IsNaN(e.X) || math.IsNaN(e.Y) || math.IsInf(e.X, 0) || math.IsInf(e.Y, 0) || e.X < 0 || e.X > 1 || e.Y < 0 || e.Y > 1 {
		return errors.Join(fmt.Errorf("invalid coordinates"), c.release())
	}
	t, err := c.target()
	if err != nil {
		return errors.Join(err, c.release())
	}
	x, y, err := Coordinates(t, e.X, e.Y)
	if errors.Is(err, ErrOutsideContent) {
		return c.release()
	}
	if err != nil {
		return errors.Join(err, c.release())
	}
	if e.Phase == "down" && c.held {
		return errors.Join(fmt.Errorf("duplicate down"), c.release())
	}
	if e.Phase != "down" && !c.held {
		return nil
	}
	p := Packet{X: x, Y: y, Move: true, Down: e.Phase == "down", Up: e.Phase == "up"}
	if err = c.backend.Send(p); err != nil {
		return errors.Join(err, c.release())
	}
	c.held = e.Phase != "up"
	return nil
}
func (c *Controller) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.release()
}
