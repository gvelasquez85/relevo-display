//go:build windows

package wintray

import (
	"fmt"
	"relevo/internal/remoteinput"
	"relevo/internal/wininput"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var keyboardPanelSerial atomic.Uint64
var keyboardPanelHandlers sync.Map

// One native callback for all panels avoids leaking a Go callback trampoline per opening.
var keyboardPanelProc = syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
	if handler, ok := keyboardPanelHandlers.Load(h); ok {
		return handler.(func(uintptr, uint32, uintptr, uintptr) uintptr)(h, m, w, l)
	}
	return call("DefWindowProcW", h, uintptr(m), w, l)
})

// ShowKeyboardPanel renders a Relevo keyboard on the streamed Windows monitor.
// It does not open the iOS keyboard and never activates its own window.
func ShowKeyboardPanel(target wininput.Target, send func(remoteinput.Key) error) (func(), error) {
	if target.Screen.Empty() || send == nil {
		return nil, fmt.Errorf("destino de teclado no disponible")
	}
	ready := make(chan error, 1)
	done := make(chan struct{})
	stop := make(chan struct{})
	var once sync.Once
	closePanel := func() {
		once.Do(func() { close(stop) })
		// The owner can detach its monitor after this bounded wait. It must not
		// invoke close synchronously from the send callback on this UI thread.
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		// Screen is in physical pixels. The Go scheduler may choose a different
		// thread than capture; establish DPI awareness on this exact UI thread.
		setDPI := user.NewProc("SetThreadDpiAwarenessContext")
		if err := setDPI.Find(); err != nil {
			ready <- fmt.Errorf("coordenadas físicas del teclado no disponibles")
			return
		}
		previous, _, _ := setDPI.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2 (-4)
		if previous == 0 {
			ready <- fmt.Errorf("no se pudo ajustar la escala del teclado")
			return
		}
		defer setDPI.Call(previous)
		instance, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
		name := p(fmt.Sprintf("RelevoKeyboardPanel%d", keyboardPanelSerial.Add(1)))
		width := target.Screen.Dx()
		height := target.Screen.Dy() * 2 / 5
		if height > 320 {
			height = 320
		}
		if height < 120 {
			height = 120
		}
		if height > target.Screen.Dy() {
			height = target.Screen.Dy()
		}
		keys := keyboardPanelLayout(width, height)
		shift := false
		status := ""
		gdi := syscall.NewLazyDLL("gdi32.dll")
		var hwnd uintptr
		handler := func(h uintptr, m uint32, w, l uintptr) uintptr {
			switch m {
			case 0x21:
				return 3 // MA_NOACTIVATE, retain click delivery.
			case 0xf:
				var ps labPaint
				dc := call("BeginPaint", h, uintptr(unsafe.Pointer(&ps)))
				defer call("EndPaint", h, uintptr(unsafe.Pointer(&ps)))
				font, _, _ := gdi.NewProc("GetStockObject").Call(17)
				old, _, _ := gdi.NewProc("SelectObject").Call(dc, font)
				defer gdi.NewProc("SelectObject").Call(dc, old)
				gdi.NewProc("SetBkMode").Call(dc, 1)
				for _, key := range keys {
					r := labRect{int32(key.Bounds.Min.X), int32(key.Bounds.Min.Y), int32(key.Bounds.Max.X), int32(key.Bounds.Max.Y)}
					call("FillRect", dc, uintptr(unsafe.Pointer(&r)), call("GetSysColorBrush", 15))
					label := key.Label
					if shift && key.Code == "" {
						label = key.event(true).Text
					}
					if key.Code == "shift" && shift {
						label = "MAYÚS ✓"
					}
					if status != "" && key.Code == "close" {
						label = "Error · cerrar"
					}
					call("DrawTextW", dc, uintptr(unsafe.Pointer(p(label))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), 0x25)
				}
				return 0
			case 0x202:
				key, ok := keyboardPanelHit(keys, int(int16(l&0xffff)), int(int16((l>>16)&0xffff)))
				if !ok {
					return 0
				}
				switch key.Code {
				case "close":
					call("DestroyWindow", h)
				case "shift":
					shift = !shift
					call("InvalidateRect", h, 0, 1)
				default:
					if err := send(key.event(shift)); err != nil {
						status = "No se pudo enviar la tecla"
						call("InvalidateRect", h, 0, 1)
					}
				}
				return 0
			case 0x10:
				call("DestroyWindow", h)
				return 0
			case 2:
				hwnd = 0
				return 0
			}
			return call("DefWindowProcW", h, uintptr(m), w, l)
		}
		c := wc{Proc: keyboardPanelProc, Instance: instance, Name: name, Cursor: call("LoadCursorW", 0, 32512), Background: call("GetSysColorBrush", 5)}
		c.Size = uint32(unsafe.Sizeof(c))
		if call("RegisterClassExW", uintptr(unsafe.Pointer(&c))) == 0 {
			ready <- fmt.Errorf("registrar teclado de Relevo")
			return
		}
		defer call("UnregisterClassW", uintptr(unsafe.Pointer(name)), instance)
		// WS_EX_NOACTIVATE | WS_EX_TOPMOST | WS_EX_TOOLWINDOW, WS_POPUP.
		hwnd = call("CreateWindowExW", 0x08000088, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p("Relevo · Teclado"))), 0x80000000, uintptr(target.Screen.Min.X), uintptr(target.Screen.Max.Y-height), uintptr(width), uintptr(height), 0, 0, instance, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("crear teclado de Relevo")
			return
		}
		registeredHandle := hwnd
		keyboardPanelHandlers.Store(hwnd, handler)
		defer keyboardPanelHandlers.Delete(registeredHandle)
		defer func() {
			if hwnd != 0 {
				call("DestroyWindow", hwnd)
			}
		}()
		select {
		case <-stop:
			ready <- fmt.Errorf("apertura de teclado cancelada")
			return
		default:
		}
		call("ShowWindow", hwnd, 4)
		call("UpdateWindow", hwnd)
		// Poll stop on the owning thread; no watcher can post to a recycled HWND.
		if call("SetTimer", hwnd, 1, 50, 0) == 0 {
			ready <- fmt.Errorf("iniciar actualización del teclado")
			return
		}
		ready <- nil
		defer func() {
			if hwnd != 0 {
				call("KillTimer", hwnd, 1)
			}
		}()
		var event msg
		for {
			r := call("GetMessageW", uintptr(unsafe.Pointer(&event)), 0, 0, 0)
			if r == 0 || int32(r) == -1 {
				return
			}
			select {
			case <-stop:
				call("DestroyWindow", hwnd)
				return
			default:
			}
			call("TranslateMessage", uintptr(unsafe.Pointer(&event)))
			call("DispatchMessageW", uintptr(unsafe.Pointer(&event)))
			if hwnd == 0 {
				return
			}
		}
	}()
	select {
	case err := <-ready:
		if err != nil {
			closePanel()
			return nil, err
		}
		return closePanel, nil
	case <-time.After(5 * time.Second):
		closePanel()
		return nil, fmt.Errorf("el teclado de Relevo no respondió al abrir")
	}
}
