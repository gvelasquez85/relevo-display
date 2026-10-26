// Package winvirtual manages a reversible display configuration for the
// separately installed Windows virtual display driver.
package winvirtual

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

const hardwareID = `root\mttvdd`
const activationTimeout = 5 * time.Second

// Mode is a display's desktop position and display settings.
type Mode struct {
	X, Y, Width, Height                int32
	Refresh, BitsPerPixel, Orientation uint32
	Attached                           bool
}

// Display is one Windows display path. HardwareID must be normalized by Backend.
type Display struct {
	Name, HardwareID string
	Primary          bool
	Mode             Mode
}

// Backend implements native display enumeration and changes to a single path.
type Backend interface {
	List(context.Context) ([]Display, error)
	Set(context.Context, string, Mode) error
}

// Lease owns one applied configuration and restores its original settings.
// Public fields describe the selected display; mutating them does not change
// the restoration target or its safety checks.
type Lease struct {
	DisplayName       string
	Mode              Mode
	backend           Backend
	name              string
	original, applied Mode
	mu                sync.Mutex
	closed            bool
	closeErr          error
}

// Acquire attaches exactly one existing, nonprimary MTT virtual display to the
// right of the physical desktop. It never installs a driver or changes counts.
func Acquire(ctx context.Context, b Backend, width, height int32) (*Lease, error) {
	return AcquirePosition(ctx, b, width, height, "right")
}

// AcquirePosition attaches outside the selected edge of the active desktop.
// Empty position retains the historical right-hand placement.
func AcquirePosition(ctx context.Context, b Backend, width, height int32, position string) (*Lease, error) {
	if position == "" {
		position = "right"
	}
	if position != "right" && position != "left" && position != "above" && position != "below" {
		return nil, fmt.Errorf("unsupported display position %q", position)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil {
		return nil, errors.New("virtual display backend is nil")
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("virtual display dimensions must be positive")
	}
	displays, err := b.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list displays: %w", err)
	}
	var target Display
	count := 0
	physical := false
	edge := int64(math.MinInt32)
	if position == "left" || position == "above" {
		edge = math.MaxInt32
	}
	anchor := int32(0)
	anchorName := ""
	for _, d := range displays {
		if strings.EqualFold(d.HardwareID, hardwareID) {
			target = d
			count++
			continue
		}
		if d.Mode.Attached {
			if d.Mode.Width <= 0 || d.Mode.Height <= 0 ||
				int64(d.Mode.X)+int64(d.Mode.Width) > math.MaxInt32 ||
				int64(d.Mode.Y)+int64(d.Mode.Height) > math.MaxInt32 {
				return nil, errors.New("attached display has invalid desktop bounds")
			}
			physical = true
			candidate, perpendicular := int64(d.Mode.X)+int64(d.Mode.Width), d.Mode.Y
			switch position {
			case "left":
				candidate = int64(d.Mode.X)
			case "above":
				candidate, perpendicular = int64(d.Mode.Y), d.Mode.X
			case "below":
				candidate, perpendicular = int64(d.Mode.Y)+int64(d.Mode.Height), d.Mode.X
			}
			better := candidate > edge
			if position == "left" || position == "above" {
				better = candidate < edge
			}
			if better || candidate == edge && (anchorName == "" || perpendicular < anchor || perpendicular == anchor && d.Name < anchorName) {
				edge, anchor, anchorName = candidate, perpendicular, d.Name
			}
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("expected exactly one %s display; found %d", hardwareID, count)
	}
	if target.Primary {
		return nil, errors.New("refusing to change the primary virtual display")
	}
	if target.Name == "" {
		return nil, errors.New("virtual display has no path name")
	}
	if !physical {
		return nil, errors.New("an attached physical display is required")
	}
	x, y := edge, int64(anchor)
	switch position {
	case "left":
		x -= int64(width)
	case "above":
		x, y = int64(anchor), edge-int64(height)
	case "below":
		x, y = int64(anchor), edge
	}
	if x < math.MinInt32 || y < math.MinInt32 || x+int64(width) > math.MaxInt32 || y+int64(height) > math.MaxInt32 {
		return nil, errors.New("virtual display position overflows desktop coordinates")
	}
	desired := Mode{X: int32(x), Y: int32(y), Width: width, Height: height, Refresh: 60, BitsPerPixel: 32, Attached: true}
	lease := &Lease{DisplayName: target.Name, Mode: desired, backend: b, name: target.Name, original: target.Mode, applied: desired}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Check identity and baseline again immediately before applying, so changes
	// between initial discovery and this operation are not silently overwritten.
	current, err := lookup(ctx, b, target.Name)
	if err != nil {
		return nil, err
	}
	if current.Primary || current.Mode != target.Mode {
		return nil, errors.New("virtual display changed before activation")
	}
	if desired == target.Mode {
		return lease, nil
	}
	if err = b.Set(ctx, target.Name, desired); err != nil {
		return nil, errors.Join(fmt.Errorf("activate virtual display: %w", err), lease.restore())
	}
	verifyCtx, cancel := context.WithTimeout(ctx, activationTimeout)
	defer cancel()
	err = waitMode(verifyCtx, b, target.Name, desired)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("verify virtual display activation: %w", err), lease.restore())
	}
	return lease, nil
}

func lookup(ctx context.Context, b Backend, name string) (Display, error) {
	displays, err := b.List(ctx)
	if err != nil {
		return Display{}, fmt.Errorf("list virtual display: %w", err)
	}
	var found Display
	count := 0
	for _, d := range displays {
		if d.Name == name {
			found = d
			count++
		}
	}
	if count != 1 || !strings.EqualFold(found.HardwareID, hardwareID) {
		return Display{}, errors.New("virtual display identity changed or disappeared")
	}
	return found, nil
}

func waitMode(ctx context.Context, b Backend, name string, want Mode) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		d, err := lookup(ctx, b, name)
		if err != nil {
			return err
		}
		if d.Primary {
			return errors.New("virtual display became primary")
		}
		if d.Mode == want || !want.Attached && !d.Mode.Attached {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// restore uses a fresh bounded context: a canceled acquisition still needs to
// undo its own changes. It refuses to overwrite another process's settings.
func (l *Lease) restore() error {
	ctx, cancel := context.WithTimeout(context.Background(), activationTimeout)
	defer cancel()
	d, err := lookup(ctx, l.backend, l.name)
	if err != nil {
		return err
	}
	if d.Primary {
		return errors.New("refusing to restore a display that became primary")
	}
	if d.Mode == l.original {
		return nil
	}
	if d.Mode != l.applied {
		return errors.New("virtual display settings changed externally; restoration skipped")
	}
	if err = l.backend.Set(ctx, l.name, l.original); err != nil {
		return fmt.Errorf("restore virtual display: %w", err)
	}
	if err = waitMode(ctx, l.backend, l.name, l.original); err != nil {
		return fmt.Errorf("verify virtual display restoration: %w", err)
	}
	return nil
}

// Close restores the original attached or detached state exactly once.
func (l *Lease) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		l.closeErr = l.restore()
	}
	return l.closeErr
}

// ChooseCaptureMode selects only modes actually advertised by the driver. It
// prefers an exact match, then the smallest larger mode of the same aspect
// ratio. Other modes balance relative size and aspect error deterministically.
func ChooseCaptureMode(modes []Mode, width, height int32) (Mode, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return Mode{}, errors.New("requested capture dimensions must be between 1 and 16384")
	}
	var best Mode
	bestRank := 4
	bestScore := math.Inf(1)
	for _, m := range modes {
		if m.Width <= 0 || m.Height <= 0 || m.Width > 16384 || m.Height > 16384 || m.BitsPerPixel != 32 || m.Orientation != 0 || m.Refresh != 60 {
			continue
		}
		rank := 2
		dw := float64(m.Width-width) / float64(width)
		dh := float64(m.Height-height) / float64(height)
		aspect := float64(int64(m.Width)*int64(height)-int64(m.Height)*int64(width)) / float64(int64(m.Height)*int64(width))
		score := dw*dw + dh*dh + 4*aspect*aspect
		if m.Width == width && m.Height == height {
			rank = 0
			score = 0
		} else if int64(m.Width)*int64(height) == int64(m.Height)*int64(width) && m.Width >= width && m.Height >= height {
			rank = 1
			score = float64(int64(m.Width) * int64(m.Height))
		}
		area := int64(m.Width) * int64(m.Height)
		bestArea := int64(best.Width) * int64(best.Height)
		if rank < bestRank || rank == bestRank && (score < bestScore || score == bestScore && (area < bestArea || area == bestArea && (m.Width < best.Width || m.Width == best.Width && m.Height < best.Height))) {
			best = m
			bestRank = rank
			bestScore = score
		}
	}
	if bestRank == 4 {
		return Mode{}, errors.New("driver advertises no supported 32-bit, unrotated, 60 Hz capture modes")
	}
	return best, nil
}

// ExactCaptureMode requires the selected advertised mode; extension must not
// silently substitute the physical display's resolution or a different aspect.
func ExactCaptureMode(modes []Mode, width, height int32) (Mode, error) {
	for _, m := range modes {
		if m.Width == width && m.Height == height && m.Refresh == 60 && m.BitsPerPixel == 32 && m.Orientation == 0 && width > 0 && height > 0 && width <= 16384 && height <= 16384 {
			return m, nil
		}
	}
	return Mode{}, fmt.Errorf("virtual driver does not advertise %dx%d at 60 Hz; update the virtual display driver settings and restart Windows", width, height)
}

// CloseDetached removes the leased display from the desktop on disconnect.
// Activation failures still use Close/restore to roll back the original state.
func (l *Lease) CloseDetached() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.closed {
		l.closed = true
		l.original.Attached = false
		l.closeErr = l.restore()
	}
	return l.closeErr
}
