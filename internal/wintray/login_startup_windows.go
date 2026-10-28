//go:build windows

package wintray

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

const loginRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const loginRunValue = "RelevoDisplay"

var startupAdvapi = syscall.NewLazyDLL("advapi32.dll")

// IsStartupLaunch identifies the login entry rather than a normal user launch.
func IsStartupLaunch() bool { return hasStartupFlag(os.Args) }

// SetLoginStartup changes only this user's RelevoDisplay Run value. It neither
// requests elevation nor changes Windows sign-in or automatic login settings.
func SetLoginStartup(enabled bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return err
	}
	command, err := loginStartupCommand(executable)
	if err != nil {
		return err
	}
	key, _ := syscall.UTF16PtrFromString(loginRunKey)
	value, _ := syscall.UTF16PtrFromString(loginRunValue)
	var handle syscall.Handle
	if enabled {
		result, _, _ := startupAdvapi.NewProc("RegCreateKeyExW").Call(uintptr(syscall.HKEY_CURRENT_USER), uintptr(unsafe.Pointer(key)), 0, 0, 0, syscall.KEY_SET_VALUE, 0, uintptr(unsafe.Pointer(&handle)), 0)
		if result != 0 {
			return fmt.Errorf("registrar inicio de Relevo: %w", syscall.Errno(result))
		}
		defer syscall.RegCloseKey(handle)
		data, err := syscall.UTF16FromString(command)
		if err != nil {
			return err
		}
		result, _, _ = startupAdvapi.NewProc("RegSetValueExW").Call(uintptr(handle), uintptr(unsafe.Pointer(value)), 0, syscall.REG_SZ, uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)*2))
		if result != 0 {
			return fmt.Errorf("guardar inicio de Relevo: %w", syscall.Errno(result))
		}
		return nil
	}
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, key, 0, syscall.KEY_QUERY_VALUE|syscall.KEY_SET_VALUE, &handle); err != nil {
		if err == syscall.ERROR_FILE_NOT_FOUND {
			return nil
		}
		return err
	}
	defer syscall.RegCloseKey(handle)
	// Do not delete a value installed by another executable under our name.
	var typ, size uint32
	err = syscall.RegQueryValueEx(handle, value, nil, &typ, nil, &size)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		return nil
	}
	if err != nil {
		return err
	}
	if typ != syscall.REG_SZ || size < 2 || size > 65536 {
		return fmt.Errorf("el inicio de Relevo contiene un valor ajeno; no se modificó")
	}
	data := make([]uint16, (size+1)/2)
	if err = syscall.RegQueryValueEx(handle, value, nil, &typ, (*byte)(unsafe.Pointer(&data[0])), &size); err != nil {
		return err
	}
	if typ != syscall.REG_SZ || syscall.UTF16ToString(data) != command {
		return fmt.Errorf("el inicio de Relevo apunta a otro ejecutable; activa esta copia antes de desactivarlo")
	}
	result, _, _ := startupAdvapi.NewProc("RegDeleteValueW").Call(uintptr(handle), uintptr(unsafe.Pointer(value)))
	if result != 0 && syscall.Errno(result) != syscall.ERROR_FILE_NOT_FOUND {
		return fmt.Errorf("quitar inicio de Relevo: %w", syscall.Errno(result))
	}
	return nil
}

// RemoveLegacyLoginStartup migrates the former portable launch entry. It only
// removes this product's exact fixed flag/filename; unrelated Run values remain.
func RemoveLegacyLoginStartup() error {
	key, _ := syscall.UTF16PtrFromString(loginRunKey)
	name, _ := syscall.UTF16PtrFromString(loginRunValue)
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, key, 0, syscall.KEY_QUERY_VALUE|syscall.KEY_SET_VALUE, &h); err != nil {
		if err == syscall.ERROR_FILE_NOT_FOUND {
			return nil
		}
		return err
	}
	defer syscall.RegCloseKey(h)
	var typ, size uint32
	err := syscall.RegQueryValueEx(h, name, nil, &typ, nil, &size)
	if err == syscall.ERROR_FILE_NOT_FOUND {
		return nil
	}
	if err != nil {
		return err
	}
	if typ != syscall.REG_SZ || size < 2 || size > 65536 {
		return nil // Preserve an unrelated or malformed Run entry.
	}
	data := make([]uint16, (size+1)/2)
	if err = syscall.RegQueryValueEx(h, name, nil, &typ, (*byte)(unsafe.Pointer(&data[0])), &size); err != nil {
		return err
	}
	command := syscall.UTF16ToString(data)
	if !legacyLoginCommand(command) {
		return nil // Preserve an unrelated or malformed Run entry.
	}
	result, _, _ := startupAdvapi.NewProc("RegDeleteValueW").Call(uintptr(h), uintptr(unsafe.Pointer(name)))
	if result != 0 && syscall.Errno(result) != syscall.ERROR_FILE_NOT_FOUND {
		return syscall.Errno(result)
	}
	return nil
}
