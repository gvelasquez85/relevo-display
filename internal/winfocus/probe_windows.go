//go:build windows && amd64

package winfocus

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var (
	getGUIThreadInfo    = user32.NewProc("GetGUIThreadInfo")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	windowFromPoint     = user32.NewProc("WindowFromPoint")
	getAncestor         = user32.NewProc("GetAncestor")
	isWindowEnabled     = user32.NewProc("IsWindowEnabled")
	getClassName        = user32.NewProc("GetClassNameW")
	getWindowLong       = user32.NewProc("GetWindowLongPtrW")
	getWindowRect       = user32.NewProc("GetWindowRect")
	setDPI              = user32.NewProc("SetThreadDpiAwarenessContext")
)

type rect struct{ left, top, right, bottom int32 }
type guiThreadInfo struct {
	size, flags                                        uint32
	active, focus, capture, menuOwner, moveSize, caret uintptr
	caretRect                                          rect
}

// Probe reports whether a just-completed touch at physical desktop coordinates
// is on the foreground native Edit/RichEdit control that currently has focus.
// Call after input injection completes. Unknown/custom/browser controls return
// false, requiring the explicit keyboard button. No text is read or retained.
//
// This performs a fixed number of User32 metadata queries, without SendMessage,
// UIAutomation providers, COM, goroutines, polling or cross-process callbacks.
// Those choices avoid waiting on a hung application's message loop. Windows
// does not expose a hard deadline for these kernel/User32 queries; callers must
// not advertise a guaranteed wall-clock timeout. Activation races fail closed.
func Probe(x, y int32) (bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	// Match the physical coordinate system used by touch injection on mixed DPI.
	previous, _, _ := setDPI.Call(^uintptr(3)) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
	if previous == 0 {
		return false, errors.New("focus probe: cannot establish physical coordinates")
	}
	defer setDPI.Call(previous)
	foreground, _, _ := getForegroundWindow.Call()
	if foreground == 0 {
		return false, nil
	}
	info := guiThreadInfo{}
	info.size = uint32(unsafe.Sizeof(info))
	ok, _, _ := getGUIThreadInfo.Call(0, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return false, errors.New("focus probe: foreground metadata unavailable")
	}
	if info.focus == 0 || info.active == 0 || info.flags&0x1e != 0 {
		return false, nil
	}
	root, _, _ := getAncestor.Call(info.focus, 2) // GA_ROOT
	activeRoot, _, _ := getAncestor.Call(info.active, 2)
	if root != foreground || activeRoot != foreground {
		return false, nil
	}
	point := uint64(uint32(x)) | uint64(uint32(y))<<32
	hit, _, _ := windowFromPoint.Call(uintptr(point))
	if hit != info.focus {
		return false, nil
	}
	var bounds rect
	ok, _, _ = getWindowRect.Call(info.focus, uintptr(unsafe.Pointer(&bounds)))
	if ok == 0 || x < bounds.left || x >= bounds.right || y < bounds.top || y >= bounds.bottom {
		return false, nil
	}
	var class [128]uint16
	n, _, _ := getClassName.Call(info.focus, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
	if n == 0 || n >= uintptr(len(class)-1) {
		return false, nil
	}
	enabled, _, _ := isWindowEnabled.Call(info.focus)
	// GWL_STYLE (-16); style queries never request the control's contents.
	style, _, _ := getWindowLong.Call(info.focus, ^uintptr(15))
	after := guiThreadInfo{size: uint32(unsafe.Sizeof(info))}
	ok, _, _ = getGUIThreadInfo.Call(0, uintptr(unsafe.Pointer(&after)))
	current, _, _ := getForegroundWindow.Call()
	if ok == 0 || current != foreground || after.focus != info.focus || after.active != info.active {
		return false, nil
	}
	return editable(metadata{syscall.UTF16ToString(class[:n]), enabled != 0, style&0x800 != 0, true, true}), nil
}
