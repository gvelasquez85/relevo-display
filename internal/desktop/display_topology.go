package desktop

import (
	"encoding/binary"
	"fmt"
	"relevo/internal/winvirtual"
)

type ccdLUID struct {
	Low  uint32
	High int32
}
type ccdSource struct {
	Adapter               ccdLUID
	ID, ModeIndex, Status uint32
}
type ccdRational struct{ Numerator, Denominator uint32 }
type ccdTarget struct {
	Adapter                                      ccdLUID
	ID, ModeIndex, Technology, Rotation, Scaling uint32
	Refresh                                      ccdRational
	ScanLine                                     uint32
	Available                                    int32
	Status                                       uint32
}
type ccdPath struct {
	Source ccdSource
	Target ccdTarget
	Flags  uint32
}
type ccdMode struct {
	Type, ID uint32
	Adapter  ccdLUID
	Data     [6]uint64
}
type ccdDeviceHeader struct {
	Type, Size uint32
	Adapter    ccdLUID
	ID         uint32
}
type ccdSourceName struct {
	Header ccdDeviceHeader
	Name   [32]uint16
}

type ccdSourceKey struct {
	Adapter ccdLUID
	ID      uint32
}

const ccdInvalidMode = ^uint32(0)

func ccdKey(p ccdPath) ccdSourceKey { return ccdSourceKey{p.Source.Adapter, p.Source.ID} }
func ccdSourceMode(m ccdMode) (width, height uint32, x, y int32) {
	var b [48]byte
	for i, v := range m.Data {
		binary.LittleEndian.PutUint64(b[i*8:], v)
	}
	return binary.LittleEndian.Uint32(b[:]), binary.LittleEndian.Uint32(b[4:]), int32(binary.LittleEndian.Uint32(b[12:])), int32(binary.LittleEndian.Uint32(b[16:]))
}
func ccdPlan(paths []ccdPath, modes []ccdMode, key ccdSourceKey, want winvirtual.Mode) ([]ccdPath, []ccdMode, error) {
	selected := -1
	for i, p := range paths {
		if ccdKey(p) == key && p.Target.Available != 0 {
			if selected != -1 {
				return nil, nil, fmt.Errorf("ambiguous connected virtual display paths")
			}
			selected = i
		}
	}
	if selected < 0 {
		return nil, nil, fmt.Errorf("virtual display has no connected CCD target")
	}
	target := paths[selected]
	active := make([]ccdPath, 0, len(paths))
	for i, p := range paths {
		if p.Flags&1 == 0 {
			continue
		}
		if i != selected && ccdKey(p) == key {
			return nil, nil, fmt.Errorf("refusing shared or cloned virtual source")
		}
		if i != selected && p.Target.Adapter == target.Target.Adapter && p.Target.ID == target.Target.ID {
			return nil, nil, fmt.Errorf("refusing shared virtual target")
		}
		if i == selected {
			if p.Source.ModeIndex >= uint32(len(modes)) {
				return nil, nil, fmt.Errorf("invalid virtual source mode")
			}
			m := modes[p.Source.ModeIndex]
			if m.Type != 1 || m.Adapter != key.Adapter || m.ID != key.ID {
				return nil, nil, fmt.Errorf("mismatched virtual source mode")
			}
			_, _, x, y := ccdSourceMode(m)
			if x == 0 && y == 0 {
				return nil, nil, fmt.Errorf("refusing primary virtual source")
			}
			continue
		}

		if p.Source.ModeIndex >= uint32(len(modes)) || p.Target.ModeIndex >= uint32(len(modes)) {
			return nil, nil, fmt.Errorf("active display has invalid CCD mode indexes")
		}
		sourceMode, targetMode := modes[p.Source.ModeIndex], modes[p.Target.ModeIndex]
		if sourceMode.Type != 1 || sourceMode.Adapter != p.Source.Adapter || sourceMode.ID != p.Source.ID || targetMode.Type != 2 || targetMode.Adapter != p.Target.Adapter || targetMode.ID != p.Target.ID {
			return nil, nil, fmt.Errorf("active display has mismatched CCD mode references")
		}
		active = append(active, p)
	}
	if len(active) == 0 {
		return nil, nil, fmt.Errorf("refusing topology without other active displays")
	}
	outModes := append([]ccdMode(nil), modes...)
	if !want.Attached {
		return active, outModes, nil
	}
	if want.Width <= 0 || want.Height <= 0 || want.Width > 16384 || want.Height > 16384 || want.BitsPerPixel != 32 || want.Refresh == 0 || (want.X == 0 && want.Y == 0) || want.Orientation != 0 {
		return nil, nil, fmt.Errorf("invalid or primary virtual source request")
	}
	m := ccdMode{Type: 1, ID: key.ID, Adapter: key.Adapter}
	var b [48]byte
	binary.LittleEndian.PutUint32(b[:], uint32(want.Width))
	binary.LittleEndian.PutUint32(b[4:], uint32(want.Height))
	binary.LittleEndian.PutUint32(b[8:], 4) // DISPLAYCONFIG_PIXELFORMAT_32BPP
	binary.LittleEndian.PutUint32(b[12:], uint32(want.X))
	binary.LittleEndian.PutUint32(b[16:], uint32(want.Y))
	for i := range m.Data {
		m.Data[i] = binary.LittleEndian.Uint64(b[i*8:])
	}

	// A queried inactive source can retain a mode entry. CCD identifiers must
	// remain unique: replace that unshared source instead of appending a duplicate.
	sourceIndex := -1
	for i, existing := range outModes {
		if existing.Type == 1 && existing.Adapter == key.Adapter && existing.ID == key.ID {
			if sourceIndex != -1 {
				return nil, nil, fmt.Errorf("duplicate virtual CCD source mode identifiers")
			}
			sourceIndex = i
		}
	}
	if sourceIndex < 0 {
		sourceIndex = len(outModes)
		outModes = append(outModes, m)
	} else {
		outModes[sourceIndex] = m
	}
	target.Source.ModeIndex = uint32(sourceIndex)
	target.Target.ModeIndex = ccdInvalidMode
	target.Target.Rotation = 1
	target.Target.Scaling = 1
	// UNSPECIFIED is legal only with refresh 0/0. This path requests an
	// explicit refresh, so supply progressive ordering (Windows SDK contract).
	target.Target.ScanLine = 1
	target.Target.Refresh = ccdRational{want.Refresh, 1}
	target.Flags = 1
	active = append(active, target)
	return active, outModes, nil
}
