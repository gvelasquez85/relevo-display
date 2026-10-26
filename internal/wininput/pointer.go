package wininput

import (
	"errors"
	"fmt"
	"math"
	"sync"
)

type PointerEvent struct {
	DX      float64 `json:"dx"`
	DY      float64 `json:"dy"`
	Button  string  `json:"button,omitempty"`
	Phase   string  `json:"phase,omitempty"`
	ScrollX float64 `json:"scrollX,omitempty"`
	ScrollY float64 `json:"scrollY,omitempty"`
}
type PointerController struct {
	mu                   sync.Mutex
	target               func() (Target, error)
	backend              Backend
	left, right, closed  bool
	fractionX, fractionY float64
}

func NewPointer(target func() (Target, error), b Backend) *PointerController {
	return &PointerController{target: target, backend: b}
}
func (p PointerEvent) Validate() error {
	for _, v := range []float64{p.DX, p.DY, p.ScrollX, p.ScrollY} {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 4096 {
			return fmt.Errorf("invalid pointer delta")
		}
	}
	if p.Button == "" {
		if p.Phase != "" {
			return fmt.Errorf("button missing")
		}
	} else if (p.Button != "left" && p.Button != "right") || (p.Phase != "down" && p.Phase != "up") {
		return fmt.Errorf("invalid pointer button")
	}
	return nil
}
func (c *PointerController) release() error {
	var e error
	if c.left {
		err := c.backend.Send(Packet{Relative: true, Up: true})
		e = errors.Join(e, err)
		if err == nil {
			c.left = false
		}
	}
	if c.right {
		err := c.backend.Send(Packet{Relative: true, RightUp: true})
		e = errors.Join(e, err)
		if err == nil {
			c.right = false
		}
	}
	c.fractionX = 0
	c.fractionY = 0
	return e
}
func (c *PointerController) Reset() error { c.mu.Lock(); defer c.mu.Unlock(); return c.release() }
func (c *PointerController) Handle(p PointerEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("pointer closed")
	}
	if e := p.Validate(); e != nil {
		return errors.Join(e, c.release())
	}
	if _, e := c.target(); e != nil {
		return errors.Join(e, c.release())
	}
	c.fractionX += p.DX
	c.fractionY += p.DY
	x, y := math.Trunc(c.fractionX), math.Trunc(c.fractionY)
	c.fractionX -= x
	c.fractionY -= y
	packet := Packet{Relative: true, Move: x != 0 || y != 0, X: int32(x), Y: int32(y)}
	if p.Button == "left" {
		packet.Down = p.Phase == "down"
		packet.Up = p.Phase == "up"
		if packet.Down {
			c.left = true
		}
	} else if p.Button == "right" {
		packet.RightDown = p.Phase == "down"
		packet.RightUp = p.Phase == "up"
		if packet.RightDown {
			c.right = true
		}
	}
	if packet.Move || p.Button != "" {
		if e := c.backend.Send(packet); e != nil {
			return errors.Join(e, c.release())
		}
	}
	if packet.Up {
		c.left = false
	}
	if packet.RightUp {
		c.right = false
	}
	for _, axis := range []struct {
		v          float64
		horizontal bool
	}{{p.ScrollY, false}, {p.ScrollX, true}} {
		if axis.v != 0 {
			if e := c.backend.Send(Packet{Relative: true, Scroll: int32(math.Round(axis.v)), HorizontalScroll: axis.horizontal}); e != nil {
				return errors.Join(e, c.release())
			}
		}
	}
	return nil
}
func (c *PointerController) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return c.release()
}
