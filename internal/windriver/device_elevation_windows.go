//go:build windows

package windriver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type deviceShellExecuteInfo struct {
	Size, Mask                        uint32
	Window                            uintptr
	Verb, File, Parameters, Directory *uint16
	Show                              int32
	Instance, IDList                  uintptr
	Class                             *uint16
	ClassKey                          uintptr
	HotKey                            uint32
	Icon, Process                     uintptr
}

// EnsureDeviceEnabled keeps elevation limited to one fixed adapter operation.
// The normal GUI and the unrelated peripherals continue without elevation.
func EnsureDeviceEnabled(ctx context.Context, enabled bool, log func(string)) error {
	err := SetDeviceEnabled(ctx, enabled, log)
	if err == nil {
		return confirmDeviceState(enabled, log)
	}
	if !errors.Is(err, ErrDeviceElevationRequired) {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	action := "disable"
	description := "desconectar"
	if enabled {
		action = "enable"
		description = "conectar"
	}
	if log != nil {
		log("Windows solicitará permiso para " + description + " únicamente el adaptador del monitor virtual.")
	}
	info := deviceShellExecuteInfo{Mask: 0x40, Verb: u16("runas"), File: u16(exe), Parameters: u16("--display-device=" + action), Show: 0}
	info.Size = uint32(unsafe.Sizeof(info))
	r, _, e := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return fmt.Errorf("Windows no autorizó %s el monitor virtual: %v", description, e)
	}
	if info.Process == 0 {
		return fmt.Errorf("Windows no devolvió el proceso de control del monitor")
	}
	defer syscall.CloseHandle(syscall.Handle(info.Process))
	// Do not abandon an in-flight PnP operation when Stop is pressed: the helper
	// must finish before cleanup can issue the inverse operation.
	kernel := syscall.NewLazyDLL("kernel32.dll")
	r, _, e = kernel.NewProc("WaitForSingleObject").Call(info.Process, 0xffffffff)
	if r != 0 {
		return fmt.Errorf("esperar control del monitor: %v", e)
	}
	var code uint32
	r, _, e = kernel.NewProc("GetExitCodeProcess").Call(info.Process, uintptr(unsafe.Pointer(&code)))
	if r == 0 {
		return fmt.Errorf("leer resultado del control del monitor: %v", e)
	}
	if code != 0 {
		if code&0xffff0000 == 0x10000 {
			return fmt.Errorf("Windows no pudo %s el monitor virtual: ConfigMgr status %#x", description, code&0xffff)
		}
		return fmt.Errorf("Windows no pudo %s el monitor virtual (proceso de control: %d)", description, code)
	}
	// Verify using PnP rather than trusting a successful helper exit code.
	if err := confirmDeviceState(enabled, log); err != nil {
		return err
	}
	return ctx.Err()
}

func confirmDeviceState(enabled bool, log func(string)) error {
	state, err := ReadDeviceState(context.Background())
	if err != nil {
		return err
	}
	if enabled && !state.Enabled || !enabled && !state.Disabled {
		return fmt.Errorf("PnP no confirmó el estado solicitado del monitor (enabled=%t status=%#x problema=%d)", enabled, state.Status, state.Problem)
	}
	if log != nil {
		log(fmt.Sprintf("Monitor virtual verificado: enabled=%t disabled=%t status=%#x problema=%d", state.Enabled, state.Disabled, state.Status, state.Problem))
	}
	return nil
}
