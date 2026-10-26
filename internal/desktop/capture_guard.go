package desktop

import (
	"context"
	"errors"
	"image"
	"time"
)

// guardedCapture never returns a frame while another (for example UAC) desktop
// owns input. The caller keeps sending the safe frame and servicing USB traffic.
// Rechecking after capture also covers a desktop transition during GDI calls.
func guardedCapture(ctx context.Context, available func() (bool, error), capture func() ([]byte, error), paused []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	active, err := available()
	if err != nil {
		return nil, err
	}
	if !active {
		return paused, nil
	}
	frame, captureErr := capture()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	active, err = available()
	if err != nil {
		return nil, err
	}
	if !active {
		return paused, nil
	}
	return frame, captureErr
}

// cursorPlacement scales the cursor and its hotspot through the same monitor
// transform as the desktop. Negative virtual desktop coordinates are valid.
func cursorPlacement(monitor, dest image.Rectangle, position, hotspot, size image.Point) (image.Rectangle, bool) {
	if monitor.Empty() || dest.Empty() || size.X <= 0 || size.Y <= 0 || !position.In(monitor) {
		return image.Rectangle{}, false
	}
	x := dest.Min.X + (position.X-monitor.Min.X-hotspot.X)*dest.Dx()/monitor.Dx()
	y := dest.Min.Y + (position.Y-monitor.Min.Y-hotspot.Y)*dest.Dy()/monitor.Dy()
	w, h := size.X*dest.Dx()/monitor.Dx(), size.Y*dest.Dy()/monitor.Dy()
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return image.Rect(x, y, x+w, y+h), true
}

// Only GDI/cursor read failures qualify for a short transition grace. Identity,
// topology and cancellation errors are never hidden by this recovery state.
type temporaryCaptureError struct{ error }

func (e temporaryCaptureError) Unwrap() error { return e.error }

type captureRecovery struct{ firstFailure time.Time }

func (r *captureRecovery) result(now time.Time, frame []byte, err error, paused []byte) ([]byte, error) {
	if err == nil {
		r.firstFailure = time.Time{}
		return frame, nil
	}
	var native temporaryCaptureError
	if !errors.As(err, &native) {
		return nil, err
	}
	if r.firstFailure.IsZero() {
		r.firstFailure = now
	}
	if now.Sub(r.firstFailure) < 2*time.Second {
		return paused, nil
	}
	return nil, err
}
