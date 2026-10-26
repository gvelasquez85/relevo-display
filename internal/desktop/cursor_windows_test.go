package desktop

import (
	"testing"
	"unsafe"
)

func TestCursorNativeLayouts(t *testing.T) {
	// These structures contain pointer-sized handles; Windows x64 is the shipped target.
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("x64 layout assertions")
	}
	if unsafe.Sizeof(cursorInfo{}) != 24 || unsafe.Offsetof(cursorInfo{}.Cursor) != 8 || unsafe.Offsetof(cursorInfo{}.X) != 16 {
		t.Fatal("CURSORINFO layout")
	}
	if unsafe.Sizeof(iconInfo{}) != 32 || unsafe.Offsetof(iconInfo{}.Mask) != 16 {
		t.Fatal("ICONINFO layout")
	}
	if unsafe.Sizeof(winBitmap{}) != 32 || unsafe.Offsetof(winBitmap{}.Bits) != 24 {
		t.Fatal("BITMAP layout")
	}
}
func TestWindowsFitNegotiatedIPadResolution(t *testing.T) {
	if got := winFitRectSize(1024, 768, 1024, 768); got != (winRect{0, 0, 1024, 768}) {
		t.Fatal(got)
	}
	if got := winFitRectSize(2048, 1536, 1024, 768); got != (winRect{0, 0, 1024, 768}) {
		t.Fatal(got)
	}
}
