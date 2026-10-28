package desktop

import (
	"context"
	"fmt"
	"image"
	"relevo/internal/wininput"
	"relevo/internal/winsecure"
	"runtime"
	"syscall"
	"unsafe"
)

type inputMonitorInfo struct {
	Size          uint32
	Monitor, Work winRect
	Flags         uint32
	Device        [32]uint16
}

// InputTarget returns a validator bound to this capture source, never the foreground window.
func InputTarget(source Source) (func() (wininput.Target, error), error) {
	s, ok := source.(*winSource)
	if !ok {
		return nil, fmt.Errorf("unsupported input source")
	}
	var identity string
	var r winRect
	var bounds image.Rectangle
	var found bool
	cb := syscall.NewCallback(func(m, dc, rr, data uintptr) uintptr {
		mi := inputMonitorInfo{Size: uint32(unsafe.Sizeof(inputMonitorInfo{}))}
		ok, _, _ := getMonitorInfo.Call(m, uintptr(unsafe.Pointer(&mi)))
		if ok != 0 {
			mr := image.Rect(int(mi.Monitor.Left), int(mi.Monitor.Top), int(mi.Monitor.Right), int(mi.Monitor.Bottom))
			bounds = bounds.Union(mr)
			name := syscall.UTF16ToString(mi.Device[:])
			if mi.Monitor == r && (identity == "" || identity == name) {
				found = true
				identity = name
			}
		}
		return 1
	})
	validate := func() (wininput.Target, error) {
		release, bindingErr := winsecure.Enter()
		if bindingErr != nil {
			return wininput.Target{}, bindingErr
		}
		defer release()
		s.mu.Lock()
		defer s.mu.Unlock()
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		dpi := user32.NewProc("SetThreadDpiAwarenessContext")
		if dpi.Find() == nil {
			old, _, _ := dpi.Call(^uintptr(3))
			if old != 0 {
				defer dpi.Call(old)
			}
		}
		if s.closed {
			return wininput.Target{}, fmt.Errorf("capture closed")
		}
		available, e := captureDesktopAvailable()
		if e != nil {
			return wininput.Target{}, e
		}
		if !available {
			return wininput.Target{}, fmt.Errorf("input desktop unavailable")
		}
		r = s.rect
		if s.lease != nil {
			d, e := captureDisplay(context.Background(), s.lease.DisplayName)
			if e != nil {
				return wininput.Target{}, e
			}
			if e := validateLiveCaptureMode(d.Mode); e != nil {
				return wininput.Target{}, e
			}
			r = winRect{d.Mode.X, d.Mode.Y, d.Mode.X + d.Mode.Width, d.Mode.Y + d.Mode.Height}
		}
		bounds = image.Rectangle{}
		found = false

		ok, _, err := enumDisplayMonitors.Call(0, 0, cb, 0)
		if ok == 0 {
			return wininput.Target{}, fmt.Errorf("enumerate input monitors: %v", err)
		}
		if !found {
			return wininput.Target{}, fmt.Errorf("capture monitor changed")
		}
		fit := winFitRectSize(r.Right-r.Left, r.Bottom-r.Top, int32(s.width), int32(s.height))
		return wininput.Target{Screen: image.Rect(int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)), Content: image.Rect(int(fit.Left), int(fit.Top), int(fit.Right), int(fit.Bottom)), Virtual: bounds, FrameWidth: s.width, FrameHeight: s.height}, nil
	}
	if _, err := validate(); err != nil {
		return nil, err
	}
	return validate, nil
}
