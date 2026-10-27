//go:build windows && amd64

package winfocus

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const focusHelperFlag = "--relevo-editable-focus-helper-v1"

// The same signed executable hosts its own UIA worker. Provider COM calls may
// hang indefinitely: only the disposable child performs them, never the GUI or
// capture process. Exit in init so no application windows or sessions start.
func init() {
	if len(os.Args) != 4 || os.Args[1] != focusHelperFlag {
		return
	}
	x, e1 := strconv.ParseInt(os.Args[2], 10, 32)
	y, e2 := strconv.ParseInt(os.Args[3], 10, 32)
	if e1 != nil || e2 != nil {
		os.Exit(2)
	}
	if automationProbe(int32(x), int32(y)) {
		os.Exit(0)
	}
	os.Exit(3)
}

// ProbeEditable adds UI Automation support for focused browser edit controls.
// It must run on the keyboard worker, not a video/control processing loop.
// Unknown and readonly fields fail closed. A hung provider child is killed and
// reaped after 500 ms; no detached goroutines or extra delivered binaries exist.
func ProbeEditable(x, y int32) (bool, error) {
	if yes, err := Probe(x, y); yes {
		return true, nil
	} else if err != nil {
		return false, err
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, focusHelperFlag, strconv.FormatInt(int64(x), 10), strconv.FormatInt(int64(y), 10))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	err = cmd.Run()
	if ctx.Err() != nil {
		return false, errors.New("editable focus provider timed out")
	}
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 3 {
		return false, nil
	}
	return false, errors.New("editable focus provider unavailable")
}

type comGUID struct {
	a    uint32
	b, c uint16
	d    [8]byte
}
type comObject struct{ table *[64]uintptr }

func (o *comObject) call(index int, args ...uintptr) uintptr {
	all := append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)
	result, _, _ := syscall.SyscallN(o.table[index], all...)
	return result
}
func succeeded(h uintptr) bool { return int32(h) >= 0 }

type automationVariant struct {
	kind     uint16
	reserved [3]uint16
	value    uint64
	extra    uint64
}

func automationProbe(x, y int32) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old, _, _ := setDPI.Call(^uintptr(3))
	if old == 0 {
		return false
	}
	defer setDPI.Call(old)
	foreground, _, _ := getForegroundWindow.Call()
	if foreground == 0 {
		return false
	}
	info := guiThreadInfo{size: uint32(unsafe.Sizeof(guiThreadInfo{}))}
	ok, _, _ := getGUIThreadInfo.Call(0, uintptr(unsafe.Pointer(&info)))
	if ok == 0 || info.focus == 0 || info.flags&0x1e != 0 {
		return false
	}
	root, _, _ := getAncestor.Call(info.focus, 2)
	if root != foreground {
		return false
	}
	hit, _, _ := windowFromPoint.Call(uintptr(uint64(uint32(x)) | uint64(uint32(y))<<32))
	hitRoot, _, _ := getAncestor.Call(hit, 2)
	if hitRoot != foreground {
		return false
	}
	ole := syscall.NewLazyDLL("ole32.dll")
	h, _, _ := ole.NewProc("CoInitializeEx").Call(0, 0)
	if !succeeded(h) {
		return false
	}
	defer ole.NewProc("CoUninitialize").Call()
	cls := comGUID{0xff48dba4, 0x60ef, 0x4201, [8]byte{0xaa, 0x87, 0x54, 0x10, 0x3e, 0xef, 0x59, 0x4e}}
	iid := comGUID{0x30cbe57d, 0xd9d0, 0x452a, [8]byte{0xab, 0x13, 0x7a, 0xc5, 0xac, 0x48, 0x25, 0xee}}
	var automation *comObject
	h, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&cls)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&automation)))
	if !succeeded(h) || automation == nil {
		return false
	}
	defer automation.call(2)
	var element *comObject
	if !succeeded(automation.call(8, uintptr(unsafe.Pointer(&element)))) || element == nil {
		return false
	}
	defer element.call(2)
	var kind, enabled, focused, focusable int32
	for _, p := range []struct {
		index int
		value *int32
	}{{21, &kind}, {28, &enabled}, {26, &focused}, {27, &focusable}} {
		if !succeeded(element.call(p.index, uintptr(unsafe.Pointer(p.value)))) {
			return false
		}
	}
	// GetCurrentPropertyValueEx(ignoreDefaultValue=true) distinguishes unsupported
	// ValuePattern readonly metadata from a genuine writable field. Never GetValue.
	var readonly automationVariant
	if !succeeded(element.call(11, 30046, 1, uintptr(unsafe.Pointer(&readonly)))) {
		return false
	}
	defer syscall.NewLazyDLL("oleaut32.dll").NewProc("VariantClear").Call(uintptr(unsafe.Pointer(&readonly)))
	var bounds rect
	if !succeeded(element.call(43, uintptr(unsafe.Pointer(&bounds)))) {
		return false
	}
	// Re-fetch focused element to exclude a focus race within a shared HWND.
	var again *comObject
	if !succeeded(automation.call(8, uintptr(unsafe.Pointer(&again)))) || again == nil {
		return false
	}
	defer again.call(2)
	var same int32
	if !succeeded(automation.call(3, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(again)), uintptr(unsafe.Pointer(&same)))) {
		return false
	}
	after := guiThreadInfo{size: uint32(unsafe.Sizeof(info))}
	ok, _, _ = getGUIThreadInfo.Call(0, uintptr(unsafe.Pointer(&after)))
	current, _, _ := getForegroundWindow.Call()
	return automationEditable(automationMetadata{kind, enabled != 0, focused != 0, focusable != 0, readonly.kind == 11, readonly.value&0xffff != 0, ok != 0 && current == foreground && after.focus == info.focus && after.active == info.active && same != 0, x >= bounds.left && x < bounds.right && y >= bounds.top && y < bounds.bottom})
}
