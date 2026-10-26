package wininput

import "fmt"

// PhysicalCoordinates maps a normalized displayed-image point to physical
// Windows desktop pixels, preserving negative origins and rejecting letterbox
// bands or invalid/stale target geometry. It performs no input injection.
func PhysicalCoordinates(t Target, x, y float64) (int32, int32, error) {
	return pixelCoordinates(t, x, y)
}

// CancelActive synchronously releases all held contacts and keyboard modifiers
// before changing input modes. A successful call keeps the controller usable.
// Release failures are sticky so no new input is accepted with uncertain state.
func (c *ExtendedController) CancelActive() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(); e != nil {
		return e
	}
	if e := c.release(); e != nil {
		c.failure = e
		return e
	}
	return nil
}

// RetryRelease retries only outstanding releases after a failed injection.
// It leaves the failure sticky: a caller must recreate the controller before
// accepting new actions. Unlike Close it preserves background release retries.
func (c *ExtendedController) RetryRelease() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return fmt.Errorf("input closed")
	}
	if e := c.release(); e != nil {
		c.failure = e
		return e
	}
	return nil
}
