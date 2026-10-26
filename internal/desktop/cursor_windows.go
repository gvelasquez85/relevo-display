package desktop

import (
	"fmt"
	"image"
	"strings"
	"syscall"
	"unsafe"
)

var getCursorInfo = user32.NewProc("GetCursorInfo")
var copyIcon = user32.NewProc("CopyIcon")
var destroyIcon = user32.NewProc("DestroyIcon")
var getIconInfo = user32.NewProc("GetIconInfo")
var drawIconEx = user32.NewProc("DrawIconEx")
var getObject = gdi32.NewProc("GetObjectW")
var saveDC = gdi32.NewProc("SaveDC")
var restoreDC = gdi32.NewProc("RestoreDC")
var intersectClipRect = gdi32.NewProc("IntersectClipRect")
var openInputDesktop = user32.NewProc("OpenInputDesktop")
var closeDesktop = user32.NewProc("CloseDesktop")
var getThreadDesktop = user32.NewProc("GetThreadDesktop")
var getUserObjectInformation = user32.NewProc("GetUserObjectInformationW")
var getCurrentThreadID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")

type cursorInfo struct {
	Size, Flags uint32
	Cursor      uintptr
	X, Y        int32
}
type iconInfo struct {
	IsIcon, HotX, HotY uint32
	Mask, Color        uintptr
}
type winBitmap struct {
	Type, Width, Height, WidthBytes int32
	Planes, BitsPixel               uint16
	Bits                            uintptr
}

// No SwitchDesktop/SetThreadDesktop: do not change the user's desktop or enter
// the secure desktop. An inaccessible input desktop means pause until it returns.
func captureDesktopAvailable() (bool, error) {
	input, _, err := openInputDesktop.Call(0, 0, 1) // DESKTOP_READOBJECTS only
	if input == 0 {
		if err == syscall.ERROR_ACCESS_DENIED || err == syscall.Errno(170) {
			return false, nil
		}
		return false, fmt.Errorf("OpenInputDesktop: %v", err)
	}
	defer closeDesktop.Call(input)
	tid, _, _ := getCurrentThreadID.Call()
	current, _, err := getThreadDesktop.Call(tid)
	if current == 0 {
		return false, fmt.Errorf("GetThreadDesktop: %v", err)
	}
	inputName, err := desktopObjectName(input)
	if err != nil {
		return false, err
	}
	currentName, err := desktopObjectName(current)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(inputName, currentName), nil
}
func desktopObjectName(handle uintptr) (string, error) {
	var name [256]uint16
	var needed uint32
	ok, _, err := getUserObjectInformation.Call(handle, 2, uintptr(unsafe.Pointer(&name[0])), uintptr(unsafe.Sizeof(name)), uintptr(unsafe.Pointer(&needed)))
	if ok == 0 {
		return "", fmt.Errorf("GetUserObjectInformation desktop name: %v", err)
	}
	return syscall.UTF16ToString(name[:]), nil
}

func drawCaptureCursor(dc uintptr, monitor, dest winRect) error {
	ci := cursorInfo{Size: uint32(unsafe.Sizeof(cursorInfo{}))}
	ok, _, err := getCursorInfo.Call(uintptr(unsafe.Pointer(&ci)))
	if ok == 0 {
		return fmt.Errorf("GetCursorInfo: %v", err)
	}
	if ci.Flags&1 == 0 || ci.Cursor == 0 {
		return nil
	} // hidden/suppressed pointer
	cursor, _, err := copyIcon.Call(ci.Cursor)
	if cursor == 0 {
		return fmt.Errorf("CopyIcon cursor: %v", err)
	}
	defer destroyIcon.Call(cursor)
	var info iconInfo
	ok, _, err = getIconInfo.Call(cursor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return fmt.Errorf("GetIconInfo: %v", err)
	}
	// GetIconInfo allocates these bitmaps; they are not the shared cursor handle.
	if info.Mask != 0 {
		defer deleteObject.Call(info.Mask)
	}
	if info.Color != 0 {
		defer deleteObject.Call(info.Color)
	}
	var bitmap winBitmap
	handle := info.Color
	if handle == 0 {
		handle = info.Mask
	}
	n, _, err := getObject.Call(handle, unsafe.Sizeof(bitmap), uintptr(unsafe.Pointer(&bitmap)))
	if n == 0 {
		return fmt.Errorf("GetObject cursor bitmap: %v", err)
	}
	height := bitmap.Height
	if info.Color == 0 {
		height /= 2
	} // monochrome AND/XOR masks
	r, visible := cursorPlacement(image.Rect(int(monitor.Left), int(monitor.Top), int(monitor.Right), int(monitor.Bottom)), image.Rect(int(dest.Left), int(dest.Top), int(dest.Right), int(dest.Bottom)), image.Pt(int(ci.X), int(ci.Y)), image.Pt(int(info.HotX), int(info.HotY)), image.Pt(int(bitmap.Width), int(height)))
	if !visible {
		return nil
	}
	saved, _, err := saveDC.Call(dc)
	if saved == 0 {
		return fmt.Errorf("SaveDC cursor: %v", err)
	}
	defer restoreDC.Call(dc, saved)
	clip, _, err := intersectClipRect.Call(dc, uintptr(int64(dest.Left)), uintptr(int64(dest.Top)), uintptr(int64(dest.Right)), uintptr(int64(dest.Bottom)))
	if clip == 0 {
		return fmt.Errorf("IntersectClipRect cursor: %v", err)
	}
	ok, _, err = drawIconEx.Call(dc, uintptr(int64(r.Min.X)), uintptr(int64(r.Min.Y)), cursor, uintptr(r.Dx()), uintptr(r.Dy()), 0, 0, 3) // DI_NORMAL
	if ok == 0 {
		return fmt.Errorf("DrawIconEx: %v", err)
	}
	return nil
}
