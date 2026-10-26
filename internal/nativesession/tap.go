package nativesession

import (
	"math"
	"relevo/internal/remoteinput"
	"time"
)

type tapTracker struct {
	count   int
	valid   bool
	x, y    float64
	started time.Time
}

func (t *tapTracker) apply(cs []remoteinput.Contact) (float64, float64, bool) {
	ok := false
	var x, y float64
	for _, c := range cs {
		switch c.Phase {
		case "down":
			if t.count == 0 {
				t.valid = true
				t.x, t.y = c.X, c.Y
				t.started = time.Now()
			}
			t.count++
			if t.count > 1 {
				t.valid = false
			}
		case "move":
			if math.Abs(c.X-t.x) > .02 || math.Abs(c.Y-t.y) > .02 {
				t.valid = false
			}
		case "up":
			if t.count == 1 && t.valid && time.Since(t.started) < 700*time.Millisecond && math.Abs(c.X-t.x) <= .02 && math.Abs(c.Y-t.y) <= .02 {
				ok = true
				x, y = c.X, c.Y
			}
			t.count--
			if t.count < 0 {
				t.count = 0
			}
		case "cancel":
			t.valid = false
			t.count--
			if t.count < 0 {
				t.count = 0
			}
		}
	}
	return x, y, ok
}
