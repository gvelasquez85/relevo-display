package wininput

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

type nativeBackend struct{}

func NativeBackend() (Backend, error) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		return nil, fmt.Errorf("touch requires Windows 64-bit")
	}
	return nativeBackend{}, nil
}
func (nativeBackend) Send(p Packet) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dll := syscall.NewLazyDLL("user32.dll")
	dpi := dll.NewProc("SetThreadDpiAwarenessContext")
	if dpi.Find() == nil {
		old, _, _ := dpi.Call(^uintptr(3))
		if old != 0 {
			defer dpi.Call(old)
		}
	}
	// INPUT is 40 bytes on Windows x64; MOUSEINPUT begins at aligned offset 8.
	var input struct {
		Kind              uint32
		Pad               uint32
		X, Y              int32
		Data, Flags, Time uint32
		Pad2              uint32
		Extra             uintptr
	}
	input.Kind = 0
	input.X = p.X
	input.Y = p.Y
	if p.Move {
		input.Flags |= 0x0001
		if !p.Relative {
			input.Flags |= 0x8000 | 0x4000
		}
	}
	if p.Down {
		input.Flags |= 2
	}
	if p.Up {
		input.Flags |= 4
	}
	if p.RightDown {
		input.Flags |= 8
	}
	if p.RightUp {
		input.Flags |= 16
	}
	if p.Scroll != 0 {
		input.Data = uint32(p.Scroll)
		if p.HorizontalScroll {
			input.Flags |= 0x1000
		} else {
			input.Flags |= 0x800
		}
	}
	n, _, e := dll.NewProc("SendInput").Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
	if n != 1 {
		return fmt.Errorf("SendInput rejected (desktop or integrity boundary): %v", e)
	}
	return nil
}
