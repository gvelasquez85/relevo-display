//go:build windows

package wintray

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// OpenPairing shows a temporary QR inside this application. Closing the window
// cancels the pending pairing; its owner cancels ctx once authenticated.
func OpenPairing(ctx context.Context, uri string) (<-chan struct{}, error) {
	pixels, width, e := pairingPixels(uri)
	if e != nil {
		return nil, e
	}
	ready := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		k := syscall.NewLazyDLL("kernel32.dll")
		instance, _, _ := k.NewProc("GetModuleHandleW").Call(0)
		name := p("RelevoEphemeralWiFiPairing")
		var hwnd uintptr
		var mu sync.Mutex
		alive := false
		proc := syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
			switch m {
			case 0xf:
				var ps labPaint
				dc := call("BeginPaint", h, uintptr(unsafe.Pointer(&ps)))
				defer call("EndPaint", h, uintptr(unsafe.Pointer(&ps)))
				info := labBitmapInfo{Size: 40, Width: int32(width), Height: -int32(width), Planes: 1, BitCount: 32}
				syscall.NewLazyDLL("gdi32.dll").NewProc("StretchDIBits").Call(dc, uintptr((560-width)/2), 96, uintptr(width), uintptr(width), 0, 0, uintptr(width), uintptr(width), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&info)), 0, 0xcc0020)
				runtime.KeepAlive(pixels)
				return 0
			case 0x10:
				call("DestroyWindow", h)
				return 0
			case 2:
				mu.Lock()
				alive = false
				mu.Unlock()
				call("PostQuitMessage", 0)
				return 0
			}
			return call("DefWindowProcW", h, uintptr(m), w, l)
		})
		icon := monitorIcon(instance)
		if icon != 0 {
			defer call("DestroyIcon", icon)
		}
		c := wc{Proc: proc, Instance: instance, Name: name, Icon: icon, SmallIcon: icon, Cursor: call("LoadCursorW", 0, 32512), Background: 6}
		c.Size = uint32(unsafe.Sizeof(c))
		if call("RegisterClassExW", uintptr(unsafe.Pointer(&c))) == 0 {
			ready <- fmt.Errorf("abrir ventana Wi-Fi")
			return
		}
		defer call("UnregisterClassW", uintptr(unsafe.Pointer(name)), instance)
		hwnd = call("CreateWindowExW", 0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(p("Relevo · Conectar iPad por Wi-Fi"))), 0xca0000, 0x80000000, 0x80000000, 580, 650, 0, 0, instance, 0)
		if hwnd == 0 {
			ready <- fmt.Errorf("crear ventana Wi-Fi")
			return
		}
		font := syscall.NewLazyDLL("gdi32.dll")
		hf, _, _ := font.NewProc("GetStockObject").Call(17)
		child := func(class, text string, style uintptr, x, y, w, h int) uintptr {
			handle := call("CreateWindowExW", 0, uintptr(unsafe.Pointer(p(class))), uintptr(unsafe.Pointer(p(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), hwnd, 0, instance, 0)
			call("SendMessageW", handle, 0x30, hf, 1)
			return handle
		}
		child("STATIC", "Abre Relevo en el iPad → Conectar Wi-Fi → Escanear QR.", 0, 22, 18, 518, 40)
		child("STATIC", "Ambos equipos en la misma red. El QR vence al conectar o cerrar.", 0, 22, 59, 518, 32)
		child("STATIC", "Alternativa: copia este enlace en Conectar Wi-Fi del iPad.", 0, 22, 430, 518, 24)
		child("EDIT", uri, 0x211844, 22, 460, 518, 86)
		child("STATIC", "El enlace es temporal. No se guarda ni se incluye en los diagnósticos.", 0, 22, 558, 518, 40)
		mu.Lock()
		alive = true
		mu.Unlock()
		call("ShowWindow", hwnd, 1)
		call("UpdateWindow", hwnd)
		ready <- nil
		watcherDone := make(chan struct{})
		defer close(watcherDone)
		go func() {
			select {
			case <-ctx.Done():
				mu.Lock()
				if alive {
					call("PostMessageW", hwnd, 0x10, 0, 0)
				}
				mu.Unlock()
			case <-watcherDone:
			}
		}()
		var m msg
		for {
			n := int32(call("GetMessageW", uintptr(unsafe.Pointer(&m)), 0, 0, 0))
			if n <= 0 {
				break
			}
			call("TranslateMessage", uintptr(unsafe.Pointer(&m)))
			call("DispatchMessageW", uintptr(unsafe.Pointer(&m)))
		}
		mu.Lock()
		if alive {
			alive = false
			mu.Unlock()
			call("DestroyWindow", hwnd)
		} else {
			mu.Unlock()
		}
	}()
	if e := <-ready; e != nil {
		return nil, e
	}
	return done, nil
}
