package desktop

import (
	"testing"
	"unsafe"
)

func TestWindowsDisplayABI(t *testing.T) {
	dm := devModeW{}
	if unsafe.Sizeof(dm) != 220 || unsafe.Offsetof(dm.Fields) != 72 || unsafe.Offsetof(dm.X) != 76 || unsafe.Offsetof(dm.BitsPerPixel) != 168 || unsafe.Offsetof(dm.Width) != 172 || unsafe.Offsetof(dm.Frequency) != 184 {
		t.Fatal("DEVMODEW ABI mismatch")
	}
	if unsafe.Sizeof(displayDeviceW{}) != 840 {
		t.Fatal("DISPLAY_DEVICEW ABI mismatch")
	}
}
func TestMTTVDDIdentity(t *testing.T) {
	for _, id := range []string{`ROOT\MTTVDD`, `root\mttvdd\0000`} {
		if !isMTTVDD(id) {
			t.Errorf("reject %q", id)
		}
	}
	for _, id := range []string{"MTT Virtual Display", `ROOT\MTTVDD2`, `PCI\MTTVDD`, `root\mttvdd-malicious`, ""} {
		if isMTTVDD(id) {
			t.Errorf("accept %q", id)
		}
	}
}

func TestWindowsModeConversion(t *testing.T) {
	dm := devModeW{X: -1920, Y: 12, Width: 1920, Height: 1080, Frequency: 60, BitsPerPixel: 32, Orientation: 0}
	mode := modeFromNative(dm, true)
	if mode.X != -1920 || mode.Y != 12 || mode.Width != 1920 || mode.Height != 1080 || mode.Refresh != 60 || mode.BitsPerPixel != 32 || mode.Orientation != 0 || !mode.Attached {
		t.Fatalf("mode conversion lost fields: %+v", mode)
	}
}
