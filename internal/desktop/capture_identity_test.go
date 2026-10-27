package desktop

import (
	"relevo/internal/winvirtual"
	"testing"
)

func TestCaptureAcceptsAttachedPrimaryVirtualMonitor(t *testing.T) {
	if !capturableVirtualFlags(1 | 4) {
		t.Fatal("primary monitor rejected")
	}
	for _, flags := range []uint32{0, 4, 1 | 8} {
		if capturableVirtualFlags(flags) {
			t.Fatal("detached or mirror accepted", flags)
		}
	}
	for _, m := range []winvirtual.Mode{
		{Attached: true, Width: 1024, Height: 768},
		{Attached: true, X: -1024, Y: -200, Width: 1024, Height: 768},
		{Attached: true, Width: 1600, Height: 1200, Orientation: 1},
	} {
		if err := validateLiveCaptureMode(m); err != nil {
			t.Fatal("live topology change rejected", err)
		}
	}
	if err := validateLiveCaptureMode(winvirtual.Mode{Attached: true, Width: -1, Height: 768}); err == nil {
		t.Fatal("invalid bounds accepted")
	}
}
