//go:build windows

package wininstall

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

var ui = syscall.NewLazyDLL("user32.dll")
var shell = syscall.NewLazyDLL("shell32.dll")
var kernel = syscall.NewLazyDLL("kernel32.dll")

func u16(s string) *uint16 { return syscall.StringToUTF16Ptr(s) }
func uiCall(name string, args ...uintptr) uintptr {
	r, _, _ := ui.NewProc(name).Call(args...)
	return r
}
func setText(h uintptr, text string) { uiCall("SetWindowTextW", h, uintptr(unsafe.Pointer(u16(text)))) }

type point struct{ X, Y int32 }
type message struct {
	H       uintptr
	M       uint32
	W, L    uintptr
	Time    uint32
	P       point
	Private uint32
}
type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	Extra, WinExtra                    int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}

func elevate(action string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	result, _, callErr := shell.NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(u16("runas"))), uintptr(unsafe.Pointer(u16(exe))), uintptr(unsafe.Pointer(u16("--setup-elevated="+action))), 0, 1)
	if result <= 32 {
		return fmt.Errorf("no se pudo autorizar la instalación: %v (código %d)", callErr, result)
	}
	return nil
}

// Run shows a single native maintenance GUI. Elevated work is explicit and
// never launches the ordinary desktop agent with an administrator token.
func Run(o Options) error {
	if err := validateAction(o.Action); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var hwnd, status uintptr
	var buttons []uintptr
	var busy bool
	var waitingElevation bool
	var mu sync.Mutex
	var result string
	const done = 0x8001
	instance, _, _ := kernel.NewProc("GetModuleHandleW").Call(0)
	className := u16("RelevoManagedSetup")
	updateStatus := func() {
		text := "Instala Relevo en una ubicación fija y prepara su servicio de Windows."
		if o.ServiceStatus != nil {
			state, err := o.ServiceStatus()
			if err == nil {
				text += "\r\nServicio: " + state
			} else {
				text += "\r\nServicio: " + err.Error()
			}
		}
		if o.DependencyStatus != nil {
			text += "\r\n" + o.DependencyStatus()
		} else {
			text += "\r\nUSB de Apple: conserva el componente ya instalado; no viene incluido en Relevo."
		}
		setText(status, text)
	}
	start := func(action string) {
		if busy {
			return
		}
		if !admin() {
			if err := elevate(action); err != nil {
				setText(status, err.Error())
			} else {
				waitingElevation = true
				setText(status, "Se abrió el instalador autorizado por Windows. Los resultados aparecerán en esa ventana.")
			}
			return
		}
		busy = true
		for _, b := range buttons {
			uiCall("EnableWindow", b, 0)
		}
		setText(status, "Preparando "+action+"… Espera a que termine esta operación.")
		go func() {
			text, err := operation(o, action)
			if err != nil {
				text = "No se completó la operación: " + err.Error()
			}
			mu.Lock()
			result = text
			mu.Unlock()
			uiCall("PostMessageW", hwnd, done, 0, 0)
		}()
	}
	wnd := syscall.NewCallback(func(h uintptr, m uint32, w, l uintptr) uintptr {
		switch m {
		case 0x6:
			if w&0xffff != 0 && waitingElevation {
				waitingElevation = false
				updateStatus()
			}
			return 0
		case 0x111:
			switch w & 0xffff {
			case 1:
				start("install")
			case 2:
				start("repair")
			case 3:
				if uiCall("MessageBoxW", h, uintptr(unsafe.Pointer(u16("¿Desinstalar Relevo? Se conservarán los controladores compartidos y tus ajustes."))), uintptr(unsafe.Pointer(u16("Relevo Display"))), 0x24) == 6 {
					start("uninstall")
				}
			case 4:
				target, err := InstalledExecutable()
				if err != nil {
					setText(status, err.Error())
					return 0
				}
				if admin() {
					setText(status, "Abre Relevo desde el menú Inicio o desde la ventana de instalación sin privilegios. El GUI de escritorio no se inicia con permisos de administrador.")
					return 0
				}
				if o.Launch == nil {
					setText(status, "No se ha configurado la apertura del GUI.")
					return 0
				}
				if err = o.Launch(target); err != nil {
					setText(status, err.Error())
				}
			case 5:
				updateStatus()
			}
			return 0
		case done:
			mu.Lock()
			text := result
			mu.Unlock()
			busy = false
			for _, b := range buttons {
				uiCall("EnableWindow", b, 1)
			}
			setText(status, text)
			return 0
		case 0x10:
			if busy {
				setText(status, "Espera a que termine la operación antes de cerrar.")
				return 0
			}
			uiCall("DestroyWindow", h)
			return 0
		case 0x2:
			uiCall("PostQuitMessage", 0)
			return 0
		}
		return uiCall("DefWindowProcW", h, uintptr(m), w, l)
	})
	cursor := uiCall("LoadCursorW", 0, 32512)
	wc := windowClass{Proc: wnd, Instance: instance, Cursor: cursor, Background: 6, Name: className}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if uiCall("RegisterClassExW", uintptr(unsafe.Pointer(&wc))) == 0 {
		return fmt.Errorf("no se pudo crear la ventana de instalación")
	}
	defer uiCall("UnregisterClassW", uintptr(unsafe.Pointer(className)), instance)
	hwnd = uiCall("CreateWindowExW", 0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(u16("Relevo Display · Instalación "+o.Version))), 0x00ca0000, 0x80000000, 0x80000000, 650, 490, 0, 0, instance, 0)
	if hwnd == 0 {
		return fmt.Errorf("no se pudo abrir la instalación")
	}
	font, _, _ := syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
	child := func(class, text string, style uintptr, x, y, width, height int, id uintptr) uintptr {
		h := uiCall("CreateWindowExW", 0, uintptr(unsafe.Pointer(u16(class))), uintptr(unsafe.Pointer(u16(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(width), uintptr(height), hwnd, id, instance, 0)
		uiCall("SendMessageW", h, 0x30, font, 1)
		return h
	}
	child("STATIC", "Relevo Display", 0, 24, 20, 590, 26, 0)
	child("STATIC", "Una instalación · servicio de Windows · pantalla, entrada y pruebas desde el GUI", 0, 24, 58, 590, 40, 0)
	buttons = append(buttons, child("BUTTON", "Instalar / Actualizar", 0x10001, 24, 112, 185, 38, 1), child("BUTTON", "Reparar", 0x10000, 224, 112, 180, 38, 2), child("BUTTON", "Desinstalar", 0x10000, 419, 112, 185, 38, 3))
	buttons = append(buttons, child("BUTTON", "Abrir Relevo y pruebas", 0x10000, 24, 165, 280, 38, 4), child("BUTTON", "Comprobar instalación", 0x10000, 324, 165, 280, 38, 5))
	status = child("EDIT", "", 0x200000|0x4|0x40|0x800, 24, 220, 580, 165, 0)
	child("STATIC", "La instalación solicita autorización UAC. No cambia tu inicio de sesión ni elimina otros controladores.", 0, 24, 400, 580, 35, 0)
	updateStatus()
	uiCall("ShowWindow", hwnd, 1)
	uiCall("UpdateWindow", hwnd)
	if o.Action != "" {
		id := uintptr(1)
		if o.Action == "repair" {
			id = 2
		} else if o.Action == "uninstall" {
			id = 3
		}
		uiCall("PostMessageW", hwnd, 0x111, id, 0)
	}
	var msg message
	for {
		r, _, e := ui.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) == -1 {
			return fmt.Errorf("instalador: %v", e)
		}
		if r == 0 {
			break
		}
		if uiCall("IsDialogMessageW", hwnd, uintptr(unsafe.Pointer(&msg))) == 0 {
			uiCall("TranslateMessage", uintptr(unsafe.Pointer(&msg)))
			uiCall("DispatchMessageW", uintptr(unsafe.Pointer(&msg)))
		}
	}
	return nil
}
