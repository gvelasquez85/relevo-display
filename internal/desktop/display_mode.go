package desktop

import (
	"fmt"
	"relevo/internal/winvirtual"
	"unsafe"
)

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

// Use the driver's full enumerated mode, not registry defaults of an inactive
// adapter. Position is the only session-specific override of that native mode.
func activationDisplayMode(modes []devModeW, want winvirtual.Mode) (devModeW, error) {
	for _, dm := range modes {
		if int32(dm.Width) != want.Width || int32(dm.Height) != want.Height || dm.BitsPerPixel != want.BitsPerPixel || dm.Frequency != want.Refresh || dm.Orientation != want.Orientation {
			continue
		}
		dm.Size = 220
		dm.DriverExtra = 0
		dm.X, dm.Y = want.X, want.Y
		dm.Fields &= 0x20 | 0x80 | 0x40000 | 0x80000 | 0x100000 | 0x200000 | 0x400000 | 0x20000000
		dm.Fields |= 0x20 | 0x80 | 0x40000 | 0x80000 | 0x100000 | 0x400000
		return dm, nil
	}
	return devModeW{}, fmt.Errorf("driver no longer advertises requested native mode %dx%d at %d Hz", want.Width, want.Height, want.Refresh)
}
