package desktop

import (
	"context"
	"testing"
	"unsafe"
)

func TestWindowsNativeLayouts(t *testing.T) {
	if unsafe.Sizeof(monitorInfo{}) != 40 {
		t.Fatal("MONITORINFO layout mismatch")
	}
	if unsafe.Offsetof(bitmapInfo{}.Colors) != 40 {
		t.Fatal("BITMAPINFOHEADER layout mismatch")
	}
}
func TestWindowsRejectsExtendWithoutDriver(t *testing.T) {
	if _, err := Open(context.Background(), Options{Mode: "extend", Display: 1}); err == nil {
		t.Fatal("extend must require driver")
	}
}
func TestWindowsCancelledOpen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, Options{Mode: "desktop", Display: 1}); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}

func TestWindowsFitRect(t *testing.T) {
	for _, tc := range []struct {
		w, h int32
		want winRect
	}{
		{1920, 1080, winRect{0, 0, 1280, 720}},
		{1024, 768, winRect{160, 0, 1120, 720}},
		{720, 1280, winRect{437, 0, 842, 720}},
	} {
		if got := winFitRect(tc.w, tc.h); got != tc.want {
			t.Fatalf("%dx%d: got %+v want %+v", tc.w, tc.h, got, tc.want)
		}
	}
}
