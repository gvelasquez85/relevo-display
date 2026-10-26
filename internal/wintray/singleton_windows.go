//go:build windows

package wintray

import (
	"context"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

type windowsSingletonAPI struct{}

func (windowsSingletonAPI) create() (uintptr, bool, error) {
	h, _, e := syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(p(`Local\RelevoDisplay.NotificationShell`))))
	if h == 0 {
		return 0, false, fmt.Errorf("crear aplicación: %v", e)
	}
	return h, e == syscall.Errno(183), nil
}
func (windowsSingletonAPI) close(h uintptr) { syscall.CloseHandle(syscall.Handle(h)) }
func (windowsSingletonAPI) window() (singletonWindow, error) {
	main := call("FindWindowW", uintptr(unsafe.Pointer(p("RelevoDisplayNotificationWindow"))), 0)
	if main == 0 {
		return singletonWindow{}, nil
	}
	var text [256]uint16
	if call("GetWindowTextW", main, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text))) == 0 {

		if call("IsWindow", main) == 0 {
			return singletonWindow{}, nil
		}
		return singletonWindow{}, fmt.Errorf("no se pudo identificar la versión de Relevo abierta")
	}
	lab := call("FindWindowW", uintptr(unsafe.Pointer(p("RelevoNativeLaboratoryWindow"))), 0)
	if lab != 0 {
		var mainPID, labPID uint32
		call("GetWindowThreadProcessId", main, uintptr(unsafe.Pointer(&mainPID)))
		call("GetWindowThreadProcessId", lab, uintptr(unsafe.Pointer(&labPID)))
		if mainPID == 0 || labPID != mainPID {
			return singletonWindow{}, fmt.Errorf("la ventana de pruebas pertenece a otra instancia; ciérrala antes de actualizar")
		}
	}
	return singletonWindow{main: main, lab: lab, title: syscall.UTF16ToString(text[:])}, nil
}
func (windowsSingletonAPI) focus(h uintptr) { call("ShowWindow", h, 9); call("SetForegroundWindow", h) }
func (windowsSingletonAPI) command(h, id uintptr) error {
	if call("PostMessageW", h, 0x111, id, 0) == 0 {

		if call("IsWindow", h) == 0 {
			return nil
		}
		return fmt.Errorf("no se pudo solicitar el cierre seguro de Relevo; comprueba si la instancia anterior se ejecutó como administrador")
	}
	return nil
}
func (windowsSingletonAPI) wait(ctx context.Context) error {
	timer := time.NewTimer(singletonPollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
