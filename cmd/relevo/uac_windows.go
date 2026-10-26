//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The explicit GUI action asks Windows for elevation of our own executable.
// --uac-probe exits before any USB, display, installation or configuration work.
func requestUACProbe() error {
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	args, _ := syscall.UTF16PtrFromString("--uac-probe")
	result, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), uintptr(unsafe.Pointer(args)), 0, 0)
	if result <= 32 {
		return fmt.Errorf("Windows no abrió la solicitud UAC o fue cancelada (código %d); no se modificó la instalación", result)
	}
	return nil
}
