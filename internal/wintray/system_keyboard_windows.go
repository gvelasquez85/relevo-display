//go:build windows

package wintray

import (
	"context"
	"fmt"
	"image"
	"path/filepath"
	"relevo/internal/wininput"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ShowSystemKeyboard opens the Windows accessibility keyboard on the streamed
// monitor. It does not request or emulate the native iOS keyboard.
func SystemKeyboardVisible() bool {
	return call("FindWindowW", uintptr(unsafe.Pointer(p("OSKMainClass"))), 0) != 0
}
func ShowSystemKeyboard(target wininput.Target) (func(), error) {
	return ShowSystemKeyboardContext(context.Background(), target)
}
func ShowSystemKeyboardContext(ctx context.Context, target wininput.Target) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	foreground := call("GetForegroundWindow")
	if target.Screen.Empty() {
		return nil, fmt.Errorf("pantalla del teclado no disponible")
	}
	// A preexisting keyboard belongs to the user. Do not move it, adopt it or
	// close it as a side effect of starting/stopping a display session.
	find := func() uintptr { return call("FindWindowW", uintptr(unsafe.Pointer(p("OSKMainClass"))), 0) }
	if find() != 0 {
		return nil, fmt.Errorf("el teclado de Windows ya está abierto; ciérralo para abrirlo en la pantalla del iPad")
	}
	kernel := syscall.NewLazyDLL("kernel32.dll")
	var path [32768]uint16
	n, _, e := kernel.NewProc("GetSystemDirectoryW").Call(uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)))
	if n == 0 || n >= uintptr(len(path)) {
		return nil, fmt.Errorf("localizar teclado de Windows: %v", e)
	}
	executable := filepath.Join(syscall.UTF16ToString(path[:n]), "osk.exe")
	// Shell launch is required for the OSK UIAccess executable. Do not invoke a
	// shell interpreter, PATH lookup, command file or third-party keyboard.
	info := shellExecuteInfo{Mask: 0x40, Verb: p("open"), File: p(executable), Show: 4}
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, e := shell.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return nil, fmt.Errorf("abrir teclado de Windows: %v", e)
	}
	var launchedPID uintptr
	if info.Process != 0 {
		launchedPID, _, _ = kernel.NewProc("GetProcessId").Call(info.Process)
		defer syscall.CloseHandle(syscall.Handle(info.Process))
	}
	var hwnd uintptr
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		hwnd = find()
		if hwnd != 0 {
			break
		}
		select {
		case <-ctx.Done():
			deadline = time.Now()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if hwnd == 0 {
		// A slow OSK can materialize after the visible readiness deadline. Clean
		// up only our exact launched process, never another user's keyboard.
		if launchedPID != 0 {
			go func() {
				limit := time.Now().Add(10 * time.Second)
				for time.Now().Before(limit) {
					late := find()
					var pid uint32
					if late != 0 {
						call("GetWindowThreadProcessId", late, uintptr(unsafe.Pointer(&pid)))
					}
					if late != 0 && uintptr(pid) == launchedPID {
						call("PostMessageW", late, 0x10, 0, 0)
						return
					}
					time.Sleep(100 * time.Millisecond)
				}
			}()
		}
		return nil, fmt.Errorf("Windows no mostró su teclado; comprueba la configuración de accesibilidad")
	}
	var windowPID uint32
	call("GetWindowThreadProcessId", hwnd, uintptr(unsafe.Pointer(&windowPID)))
	owned := launchedPID != 0 && uint32(launchedPID) == windowPID
	if !owned {
		return nil, fmt.Errorf("Windows abrió el teclado mediante otro proceso; no se modificó una ventana cuya propiedad no pudo comprobarse")
	}
	if err := ctx.Err(); err != nil {
		call("PostMessageW", hwnd, 0x10, 0, 0)
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	setDPI := user.NewProc("SetThreadDpiAwarenessContext")
	previous, _, _ := setDPI.Call(^uintptr(3))
	closeOwned := func() {
		var pid uint32
		call("GetWindowThreadProcessId", hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == windowPID && find() == hwnd {
			call("PostMessageW", hwnd, 0x10, 0, 0)
		}
	}
	if previous == 0 {
		closeOwned()
		return nil, fmt.Errorf("no se pudo ajustar la escala del teclado del sistema")
	}
	defer setDPI.Call(previous)
	var rect labRect
	if call("GetWindowRect", hwnd, uintptr(unsafe.Pointer(&rect))) == 0 {
		closeOwned()
		return nil, fmt.Errorf("no se pudo medir el teclado del sistema")
	}
	bounds := systemKeyboardBounds(target.Screen, image.Rect(int(rect.Left), int(rect.Top), int(rect.Right), int(rect.Bottom)))
	if bounds.Empty() {
		closeOwned()
		return nil, fmt.Errorf("tamaño del teclado del sistema no disponible")
	}
	// SWP_NOACTIVATE|SWP_SHOWWINDOW; OSK itself preserves the editor's focus.
	if call("SetWindowPos", hwnd, ^uintptr(0), uintptr(bounds.Min.X), uintptr(bounds.Min.Y), uintptr(bounds.Dx()), uintptr(bounds.Dy()), 0x50) == 0 {
		closeOwned()
		return nil, fmt.Errorf("no se pudo colocar el teclado de Windows en el iPad")
	}
	if foreground != 0 && call("GetForegroundWindow") == hwnd {
		call("SetForegroundWindow", foreground)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			closeOwned()
			limit := time.Now().Add(time.Second)
			for time.Now().Before(limit) {
				if find() != hwnd {
					return
				}
				time.Sleep(25 * time.Millisecond)
			}
		})
	}, nil
}
