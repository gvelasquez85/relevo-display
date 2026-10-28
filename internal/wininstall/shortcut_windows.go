//go:build windows

package wininstall

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var clsidShellLink = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
var iidShellLink = windows.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
var iidPersistFile = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}

type comObject struct{ table *[32]uintptr }

func comCall(obj *comObject, index int, args ...uintptr) error {
	all := append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)
	r, _, _ := syscall.SyscallN(obj.table[index], all...)
	if int32(r) < 0 {
		return fmt.Errorf("acceso directo: HRESULT %#x", r)
	}
	return nil
}
func shortcutPath() (string, error) {
	programs, err := windows.KnownFolderPath(windows.FOLDERID_CommonPrograms, 0)
	if err != nil {
		return "", err
	}
	return filepath.Join(programs, "Relevo Display.lnk"), nil
}
func writeShortcut(target string) error {
	path, err := shortcutPath()
	if err != nil {
		return err
	}
	if err = safePath(path); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err = windows.CoInitializeEx(0, 2); err != nil && err != syscall.Errno(1) {
		return err
	}
	defer windows.CoUninitialize()
	var link *comObject
	r, _, _ := syscall.NewLazyDLL("ole32.dll").NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, 1, uintptr(unsafe.Pointer(&iidShellLink)), uintptr(unsafe.Pointer(&link)))
	if int32(r) < 0 || link == nil {
		return fmt.Errorf("crear acceso directo: HRESULT %#x", r)
	}
	defer comCall(link, 2)
	if err = comCall(link, 20, uintptr(unsafe.Pointer(u16(target)))); err != nil {
		return err
	}
	if err = comCall(link, 9, uintptr(unsafe.Pointer(u16(filepath.Dir(target))))); err != nil {
		return err
	}
	var persist *comObject
	if err = comCall(link, 0, uintptr(unsafe.Pointer(&iidPersistFile)), uintptr(unsafe.Pointer(&persist))); err != nil {
		return err
	}
	defer comCall(persist, 2)
	return comCall(persist, 6, uintptr(unsafe.Pointer(u16(path))), 1)
}
func removeShortcut() error {
	path, err := shortcutPath()
	if err != nil {
		return err
	}
	if err = safePath(path); err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
