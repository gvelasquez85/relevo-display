package desktop

import (
	"relevo/internal/winvirtual"
	"testing"
	"unsafe"
)

func TestActivationUsesAdvertisedNativeMode(t *testing.T) {
	want := winvirtual.Mode{X: 1920, Width: 1024, Height: 768, Refresh: 60, BitsPerPixel: 32, Attached: true}
	mode := devModeW{Size: 220, Fields: 0x20000000 | 0x200000, Width: 1024, Height: 768, BitsPerPixel: 32, Frequency: 60, DisplayFlags: 2, FixedOutput: 2}
	wrong := mode
	wrong.Width = 1920
	got, err := activationDisplayMode([]devModeW{wrong, mode}, want)
	if err != nil {
		t.Fatal(err)
	}
	if got.X != 1920 || got.Y != 0 || got.DisplayFlags != 2 || got.FixedOutput != 2 || got.Fields&0x20000000 == 0 || got.Fields&0x200000 == 0 {
		t.Fatalf("driver mode lost: %+v", got)
	}
	if got.Width != 1024 || got.Height != 768 || got.Size != 220 || got.DriverExtra != 0 {
		t.Fatalf("invalid selected mode: %+v", got)
	}
}
func TestActivationRejectsMissingAdvertisedMode(t *testing.T) {
	for _, mode := range []devModeW{{Width: 1920, Height: 1080, BitsPerPixel: 32, Frequency: 60}, {Width: 1024, Height: 768, BitsPerPixel: 32, Frequency: 30}, {Width: 1024, Height: 768, BitsPerPixel: 16, Frequency: 60}, {Width: 1024, Height: 768, BitsPerPixel: 32, Frequency: 60, Orientation: 1}} {
		if _, err := activationDisplayMode([]devModeW{mode}, winvirtual.Mode{Width: 1024, Height: 768, BitsPerPixel: 32, Refresh: 60}); err == nil {
			t.Fatal("accepted unsupported native mode")
		}
	}
}

func TestNativeDisplayModeLayout(t *testing.T) {
	var dm devModeW
	for _, tc := range []struct{ got, want uintptr }{{unsafe.Sizeof(dm), 220}, {unsafe.Offsetof(dm.X), 76}, {unsafe.Offsetof(dm.Orientation), 84}, {unsafe.Offsetof(dm.FixedOutput), 88}, {unsafe.Offsetof(dm.BitsPerPixel), 168}, {unsafe.Offsetof(dm.Width), 172}, {unsafe.Offsetof(dm.DisplayFlags), 180}, {unsafe.Offsetof(dm.Frequency), 184}} {
		if tc.got != tc.want {
			t.Fatalf("native ABI offset/size %d, want %d", tc.got, tc.want)
		}
	}
}
