package desktop

import (
	"context"
	"fmt"
	"reflect"
	"relevo/internal/winvirtual"
	"strings"
	"syscall"
	"unsafe"
)

var ccdGetSizes = user32.NewProc("GetDisplayConfigBufferSizes")
var ccdQuery = user32.NewProc("QueryDisplayConfig")
var ccdDeviceInfo = user32.NewProc("DisplayConfigGetDeviceInfo")
var ccdSet = user32.NewProc("SetDisplayConfig")

func queryCCD(ctx context.Context) ([]ccdPath, []ccdMode, error) {
	for attempt := 0; attempt < 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		var np, nm uint32
		r, _, _ := ccdGetSizes.Call(1, uintptr(unsafe.Pointer(&np)), uintptr(unsafe.Pointer(&nm)))
		if r != 0 {
			return nil, nil, fmt.Errorf("GetDisplayConfigBufferSizes status %d", r)
		}
		if np == 0 || nm == 0 || np > 16384 || nm > 65536 {
			return nil, nil, fmt.Errorf("invalid CCD buffer sizes %d/%d", np, nm)
		}
		paths := make([]ccdPath, np)
		modes := make([]ccdMode, nm)
		r, _, _ = ccdQuery.Call(1, uintptr(unsafe.Pointer(&np)), uintptr(unsafe.Pointer(&paths[0])), uintptr(unsafe.Pointer(&nm)), uintptr(unsafe.Pointer(&modes[0])), 0)
		if r == 122 {
			continue
		}
		if r != 0 {
			return nil, nil, fmt.Errorf("QueryDisplayConfig status %d", r)
		}
		if np > uint32(len(paths)) || nm > uint32(len(modes)) {
			return nil, nil, fmt.Errorf("invalid CCD result sizes")
		}
		return paths[:np], modes[:nm], nil
	}
	return nil, nil, fmt.Errorf("CCD topology changed repeatedly while querying")
}
func setVirtualDisplayTopology(ctx context.Context, name string, want winvirtual.Mode, log func(string)) error {
	paths, modes, err := queryCCD(ctx)
	if err != nil {
		return err
	}
	keys := map[ccdSourceKey]bool{}
	var key ccdSourceKey
	matches := 0
	for _, p := range paths {
		k := ccdKey(p)
		if keys[k] {
			continue
		}
		keys[k] = true
		if err := ctx.Err(); err != nil {
			return err
		}
		n := ccdSourceName{Header: ccdDeviceHeader{Type: 1, Size: uint32(unsafe.Sizeof(ccdSourceName{})), Adapter: k.Adapter, ID: k.ID}}
		r, _, _ := ccdDeviceInfo.Call(uintptr(unsafe.Pointer(&n)))
		if r != 0 {
			if log != nil {
				log(fmt.Sprintf("display CCD source name unavailable: status %d", r))
			}
			continue
		}
		if strings.EqualFold(syscall.UTF16ToString(n.Name[:]), name) {
			key = k
			matches++
		}
	}
	if matches != 1 {
		return fmt.Errorf("expected unique CCD source for %s; found %d", name, matches)
	}
	planned, plannedModes, err := ccdPlan(paths, modes, key, want)
	if err != nil {
		return err
	}
	if log != nil {
		for i, p := range planned {
			if i >= 16 {
				break
			}
			log(fmt.Sprintf("display CCD path %d: flags=%#x sourceMode=%d targetMode=%d refresh=%d/%d scan=%d rotation=%d scaling=%d", i, p.Flags, p.Source.ModeIndex, p.Target.ModeIndex, p.Target.Refresh.Numerator, p.Target.Refresh.Denominator, p.Target.ScanLine, p.Target.Rotation, p.Target.Scaling))
			if p.Source.ModeIndex < uint32(len(plannedModes)) {
				mode := plannedModes[p.Source.ModeIndex]
				width, height, x, y := ccdSourceMode(mode)
				log(fmt.Sprintf("display CCD source %d: type=%d size=%dx%d position=%d,%d pixelFormat=%d", i, mode.Type, width, height, x, y, uint32(mode.Data[1])))
			}
		}
	}
	for _, flags := range []uintptr{0x20 | 0x40, 0x20 | 0x80} {
		if flags&0x80 != 0 {
			latestPaths, latestModes, e := queryCCD(ctx)
			if e != nil {
				return e
			}
			if !reflect.DeepEqual(paths, latestPaths) || !reflect.DeepEqual(modes, latestModes) {
				return fmt.Errorf("display topology changed during validation; refusing to overwrite concurrent settings")
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		r, _, _ := ccdSet.Call(uintptr(len(planned)), uintptr(unsafe.Pointer(&planned[0])), uintptr(len(plannedModes)), uintptr(unsafe.Pointer(&plannedModes[0])), flags)
		if log != nil {
			log(fmt.Sprintf("display CCD topology: attached=%t paths=%d modes=%d flags=%#x status=%d", want.Attached, len(planned), len(plannedModes), flags, r))
		}
		if r != 0 {
			return fmt.Errorf("SetDisplayConfig(%s, attached=%t, flags=%#x) status %d", name, want.Attached, flags, r)
		}
	}
	return nil
}
