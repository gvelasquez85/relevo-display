//go:build windows

package wintray

import (
	"context"
	"fmt"
	"golang.org/x/sys/windows"
	"path/filepath"
	"strings"
	"unsafe"
)

// CloseForMaintenance asks the recognized GUI to run its ordinary guarded Exit.
// It never terminates a process, bypasses sole-monitor protection, or closes a
// window identified only by its caption.
func CloseForMaintenance(ctx context.Context) error {
	api := windowsSingletonAPI{}
	for {
		window, err := api.window()
		if err != nil {
			return err
		}
		if window.main == 0 {
			return nil
		}
		if _, err = parseShellVersion(window.title); err != nil {
			return err
		}
		var pid uint32
		call("GetWindowThreadProcessId", window.main, uintptr(unsafe.Pointer(&pid)))
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
		if err != nil {
			return err
		}
		var path [32768]uint16
		size := uint32(len(path))
		err = windows.QueryFullProcessImageName(h, 0, &path[0], &size)
		windows.CloseHandle(h)
		if err != nil {
			return err
		}
		if !strings.EqualFold(filepath.Base(windows.UTF16ToString(path[:size])), "Relevo-Windows.exe") {
			return fmt.Errorf("la ventana abierta no pertenece al ejecutable Relevo")
		}
		if window.lab != 0 {
			if err = api.command(window.lab, 3); err != nil {
				return err
			}
		} else if err = api.command(window.main, 6); err != nil {
			return err
		}
		if err = api.wait(ctx); err != nil {
			return fmt.Errorf("no se cerró la sesión de pantalla; la instalación conserva la versión activa: %w", err)
		}
	}
}
