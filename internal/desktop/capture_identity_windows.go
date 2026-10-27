package desktop

import (
	"context"
	"fmt"
	"syscall"
	"unsafe"

	"relevo/internal/winvirtual"
)

// captureDisplay reads identity and live coordinates on every frame. Unlike
// List it avoids registry/mode queries for every unrelated physical adapter.
func captureDisplay(ctx context.Context, name string) (winvirtual.Display, error) {
	for i := uint32(0); ; i++ {
		if err := ctx.Err(); err != nil {
			return winvirtual.Display{}, err
		}
		d := displayDeviceW{Size: uint32(unsafe.Sizeof(displayDeviceW{}))}
		ok, _, _ := enumDisplayDevicesW.Call(0, uintptr(i), uintptr(unsafe.Pointer(&d)), 0)
		if ok == 0 {
			break
		}
		if syscall.UTF16ToString(d.Name[:]) != name {
			continue
		}
		if d.Flags&8 != 0 {
			return winvirtual.Display{}, fmt.Errorf("virtual display identity became a mirroring device")
		}
		if !capturableVirtualFlags(d.Flags) {
			return winvirtual.Display{}, displayTransitionError{fmt.Errorf("virtual display is temporarily detached")}
		}
		id := syscall.UTF16ToString(d.ID[:])
		if !isMTTVDD(id) {
			id = matchingDeviceID(syscall.UTF16ToString(d.Key[:]))
		}
		if !isMTTVDD(id) {
			return winvirtual.Display{}, fmt.Errorf("virtual display hardware identity changed")
		}
		dm, err := readDisplayMode(name, true)
		if err != nil {
			return winvirtual.Display{}, displayTransitionError{err}
		}
		return winvirtual.Display{Name: name, HardwareID: id, Primary: d.Flags&4 != 0, Mode: modeFromNative(dm, true)}, nil
	}
	return winvirtual.Display{}, displayTransitionError{fmt.Errorf("virtual display temporarily unavailable")}
}
