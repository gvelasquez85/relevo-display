package desktop

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"relevo/internal/winvirtual"
)

var enumDisplayDevicesW = user32.NewProc("EnumDisplayDevicesW")
var enumDisplaySettingsExW = user32.NewProc("EnumDisplaySettingsExW")
var changeDisplaySettingsExW = user32.NewProc("ChangeDisplaySettingsExW")

type displayDeviceW struct {
	Size        uint32
	Name        [32]uint16
	Description [128]uint16
	Flags       uint32
	ID          [128]uint16
	Key         [128]uint16
}

// Public DEVMODEW display union layout; identical on Windows x86/x64/ARM64.
type devModeW struct {
	DeviceName                                           [32]uint16
	SpecVersion, DriverVersion, Size, DriverExtra        uint16
	Fields                                               uint32
	X, Y                                                 int32
	Orientation, FixedOutput                             uint32
	Color, Duplex, YResolution, TTOption, Collate        int16
	FormName                                             [32]uint16
	LogPixels                                            uint16
	BitsPerPixel, Width, Height, DisplayFlags, Frequency uint32
	Tail                                                 [8]uint32
}

var _ [220 - unsafe.Sizeof(devModeW{})]byte
var _ [unsafe.Sizeof(devModeW{}) - 220]byte

type windowsDisplayBackend struct{}

func isMTTVDD(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	return id == `root\mttvdd` || strings.HasPrefix(id, `root\mttvdd\`)
}

// DeviceKey is supplied by Windows, never by a caller. Read only the driver's
// exact MatchingDeviceId: display names/descriptions are not identity evidence.
func matchingDeviceID(key string) string {
	const prefix = `\Registry\Machine\`
	if !strings.HasPrefix(strings.ToLower(key), strings.ToLower(prefix)) {
		return ""
	}
	p, err := syscall.UTF16PtrFromString(key[len(prefix):])
	if err != nil {
		return ""
	}
	var h syscall.Handle
	if syscall.RegOpenKeyEx(syscall.HKEY_LOCAL_MACHINE, p, 0, syscall.KEY_QUERY_VALUE, &h) != nil {
		return ""
	}
	defer syscall.RegCloseKey(h)
	name, _ := syscall.UTF16PtrFromString("MatchingDeviceId")
	var typ, size uint32
	if syscall.RegQueryValueEx(h, name, nil, &typ, nil, &size) != nil || typ != syscall.REG_SZ || size < 2 || size > 8192 {
		return ""
	}
	data := make([]uint16, (size+1)/2)
	if syscall.RegQueryValueEx(h, name, nil, &typ, (*byte)(unsafe.Pointer(&data[0])), &size) != nil || typ != syscall.REG_SZ {
		return ""
	}
	return syscall.UTF16ToString(data)
}
func readDisplayMode(name string, attached bool) (devModeW, error) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return devModeW{}, err
	}
	dm := devModeW{Size: 220}
	which := uint32(0xffffffff)
	if !attached {
		which = 0xfffffffe
	}
	ok, _, _ := enumDisplaySettingsExW.Call(uintptr(unsafe.Pointer(p)), uintptr(which), uintptr(unsafe.Pointer(&dm)), 0)
	if ok == 0 {
		return dm, fmt.Errorf("cannot read mode for %s", name)
	}
	return dm, nil
}
func modeFromNative(dm devModeW, attached bool) winvirtual.Mode {
	return winvirtual.Mode{X: dm.X, Y: dm.Y, Width: int32(dm.Width), Height: int32(dm.Height), Refresh: dm.Frequency, BitsPerPixel: dm.BitsPerPixel, Orientation: dm.Orientation, Attached: attached}
}
func (windowsDisplayBackend) List(ctx context.Context) ([]winvirtual.Display, error) {
	var out []winvirtual.Display
	for i := uint32(0); ; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		d := displayDeviceW{Size: uint32(unsafe.Sizeof(displayDeviceW{}))}
		ok, _, _ := enumDisplayDevicesW.Call(0, uintptr(i), uintptr(unsafe.Pointer(&d)), 0)
		if ok == 0 {
			break
		}
		if d.Flags&8 != 0 {
			continue
		} // mirroring drivers have no capturable desktop
		name := syscall.UTF16ToString(d.Name[:])
		id := syscall.UTF16ToString(d.ID[:])
		if !isMTTVDD(id) {
			id = matchingDeviceID(syscall.UTF16ToString(d.Key[:]))
		}
		if isMTTVDD(id) {
			id = `root\mttvdd`
		}
		attached := d.Flags&1 != 0
		dm, err := readDisplayMode(name, attached)
		if err != nil {
			if attached || isMTTVDD(id) {
				return nil, err
			}
			continue
		}
		out = append(out, winvirtual.Display{Name: name, HardwareID: id, Primary: d.Flags&4 != 0, Mode: modeFromNative(dm, attached)})
	}
	return out, nil
}
func (b windowsDisplayBackend) Set(ctx context.Context, name string, m winvirtual.Mode) error {
	ds, err := b.List(ctx)
	if err != nil {
		return err
	}
	var found *winvirtual.Display
	for i := range ds {
		if ds[i].Name == name {
			found = &ds[i]
			break
		}
	}
	if found == nil || !isMTTVDD(found.HardwareID) || found.Primary {
		return fmt.Errorf("refusing to change unverified or primary display %q", name)
	}
	dm, err := readDisplayMode(name, found.Mode.Attached)
	if err != nil {
		return err
	}
	dm.DriverExtra = 0
	dm.Fields = 0x20 | 0x80000 | 0x100000 // position, width, height
	dm.X, dm.Y = m.X, m.Y
	if m.Attached {
		if m.Width <= 0 || m.Height <= 0 {
			return fmt.Errorf("invalid virtual display dimensions")
		}
		dm.Width, dm.Height = uint32(m.Width), uint32(m.Height)
		dm.BitsPerPixel, dm.Frequency, dm.Orientation = m.BitsPerPixel, m.Refresh, m.Orientation
		dm.Fields |= 0x40000 | 0x400000 | 0x80
	} else {
		dm.Width, dm.Height = 0, 0
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// A named dynamic update modifies only this adapter and leaves registry defaults intact.
	result, _, _ := changeDisplaySettingsExW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&dm)), 0, 0, 0)
	if int32(result) != 0 {
		action := "activar"
		if !m.Attached {
			action = "retirar"
		}
		return fmt.Errorf("no se pudo %s la pantalla virtual %s (%dx%d, posición %d,%d): ChangeDisplaySettingsExW status %d", action, name, dm.Width, dm.Height, m.X, m.Y, int32(result))
	}
	return nil
}

// Holding this named object prevents our own processes from leasing the same
// virtual adapter concurrently. Windows closes the handle after a crash too.
func acquireVirtualMutex() (syscall.Handle, error) {
	name, _ := syscall.UTF16PtrFromString(`Local\YamTrial.MttVDD`)
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("CreateMutexW")
	handle, _, err := proc.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return 0, fmt.Errorf("create virtual display lock: %w", err)
	}
	if err == syscall.ERROR_ALREADY_EXISTS {
		syscall.CloseHandle(syscall.Handle(handle))
		return 0, fmt.Errorf("virtual display is busy: another Relevo Display instance is using it")
	}
	return syscall.Handle(handle), nil
}

// Enumerate modes actually advertised by this driver installation; its XML
// defaults may not contain the smaller negotiated video frame resolution.
func virtualCaptureMode(ctx context.Context, b windowsDisplayBackend, width, height int) (winvirtual.Mode, error) {
	displays, err := b.List(ctx)
	if err != nil {
		return winvirtual.Mode{}, err
	}
	name := ""
	count := 0
	for _, d := range displays {
		if isMTTVDD(d.HardwareID) {
			if d.Primary {
				return winvirtual.Mode{}, fmt.Errorf("refusing primary virtual display")
			}
			name = d.Name
			count++
		}
	}
	if count != 1 {
		return winvirtual.Mode{}, fmt.Errorf("expected exactly one root\\mttvdd display; found %d", count)
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return winvirtual.Mode{}, err
	}
	var modes []winvirtual.Mode
	for i := uint32(0); i < 10000; i++ {
		if err := ctx.Err(); err != nil {
			return winvirtual.Mode{}, err
		}
		dm := devModeW{Size: 220}
		ok, _, _ := enumDisplaySettingsExW.Call(uintptr(unsafe.Pointer(p)), uintptr(i), uintptr(unsafe.Pointer(&dm)), 0)
		if ok == 0 {
			return winvirtual.ExactCaptureMode(modes, int32(width), int32(height))
		}
		modes = append(modes, modeFromNative(dm, true))
	}
	return winvirtual.Mode{}, fmt.Errorf("virtual display mode enumeration exceeded safety limit")
}
