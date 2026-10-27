package yaminput

import (
	"fmt"
	"math"
	"relevo/internal/remoteinput"
	"relevo/internal/wininput"
	"time"
)

// These are explicitly reconstructed contacts, never purported wire contact IDs.
// Aggregate Yam pan and scale streams cannot recover independent finger paths.
type nativeGesture struct {
	kind                                    string
	x, y, initialX, initialY, scale, radius float64
	minX, maxX, minY, maxY                  float64
	updated                                 time.Time
}

func (c *Controller) nativeEvent(e map[string]interface{}, kind string, t wininput.Target) (bool, error) {
	startKey, endKey := "startOfPan", "endOfPan"
	if kind == "Zoomed" {
		startKey, endKey = "startOfZoom", "endOfZoom"
	}
	start, ok := inputBool(e, startKey)
	if !ok {
		return true, fmt.Errorf("invalid native gesture phase")
	}
	end, ok := inputBool(e, endKey)
	if !ok {
		return true, fmt.Errorf("invalid native gesture phase")
	}
	var x, y, scale float64
	if kind == "Zoomed" {
		var valid bool
		scale, valid = inputNumber(e["scale"])
		// Discrete smart-zoom events do not contain a contact lifecycle.
		if _, present := e["scale"]; !present {
			if c.native.kind != "" {
				return true, fmt.Errorf("ambiguous native zoom")
			}
			return false, nil
		}
		if !valid || scale < .01 || scale > 100 {
			return true, fmt.Errorf("invalid native scale")
		}
	} else {
		var a, b bool
		x, a = inputNumber(e["x"])
		y, b = inputNumber(e["y"])
		if !a || !b || math.Abs(x) > 64 || math.Abs(y) > 64 {
			return true, fmt.Errorf("invalid native translation")
		}
	}
	if c.drag {
		return true, fmt.Errorf("native gesture during drag")
	}
	if c.native.kind != "" && (c.native.kind != kind || start) {
		return true, fmt.Errorf("ambiguous native gesture")
	}
	if c.native.kind == "" {
		if !start || end {
			return false, nil
		}
		minX, maxX := float64(t.Content.Min.X)/float64(t.FrameWidth-1), float64(t.Content.Max.X-1)/float64(t.FrameWidth-1)
		minY, maxY := float64(t.Content.Min.Y)/float64(t.FrameHeight-1), float64(t.Content.Max.Y-1)/float64(t.FrameHeight-1)
		radius := math.Min(.06, (maxX-minX)/4)
		if radius <= 0 || maxY <= minY {
			return true, fmt.Errorf("native content unavailable")
		}
		cx := math.Max(minX+radius, math.Min(maxX-radius, c.lastX))
		cy := math.Max(minY, math.Min(maxY, c.lastY))
		c.native = nativeGesture{kind: kind, x: cx, y: cy, initialX: x, initialY: y, scale: scale, radius: radius, minX: minX, maxX: maxX, minY: minY, maxY: maxY, updated: time.Now()}
		return true, c.nativeContacts("down", cx, cy, radius)
	}
	n := &c.native
	n.updated = time.Now()
	cx, cy, radius := n.x, n.y, n.radius
	if kind == "Zoomed" {
		radius *= scale / n.scale
		radius = math.Max(1/float64(t.FrameWidth-1), math.Min(radius, math.Min(cx-n.minX, n.maxX-cx)))
	} else {
		cx = math.Max(n.minX+radius, math.Min(n.maxX-radius, n.x+x-n.initialX))
		cy = math.Max(n.minY, math.Min(n.maxY, n.y+y-n.initialY))
	}
	phase := "move"
	if end {
		phase = "up"
	}
	err := c.nativeContacts(phase, cx, cy, radius)
	if err == nil && end {
		c.native = nativeGesture{}
	}
	return true, err
}
func (c *Controller) nativeContacts(phase string, x, y, radius float64) error {
	return c.extended.HandleContacts([]remoteinput.Contact{{ID: 1, Phase: phase, X: x - radius, Y: y}, {ID: 2, Phase: phase, X: x + radius, Y: y}})
}
