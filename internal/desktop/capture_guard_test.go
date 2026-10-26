package desktop

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"
)

func TestCaptureGuardPausesAndResumes(t *testing.T) {
	active := false
	calls := 0
	check := func() (bool, error) { return active, nil }
	capture := func() ([]byte, error) { calls++; return []byte("desktop"), nil }
	pause := []byte("paused")
	got, err := guardedCapture(context.Background(), check, capture, pause)
	if err != nil || string(got) != "paused" || calls != 0 {
		t.Fatalf("pause: %s %v calls %d", got, err, calls)
	}
	active = true
	got, err = guardedCapture(context.Background(), check, capture, pause)
	if err != nil || string(got) != "desktop" || calls != 1 {
		t.Fatalf("resume: %s %v calls %d", got, err, calls)
	}
}
func TestCaptureGuardDiscardsFrameWhenDesktopSwitches(t *testing.T) {
	active := true
	got, err := guardedCapture(context.Background(), func() (bool, error) { return active, nil }, func() ([]byte, error) { active = false; return []byte("sensitive"), nil }, []byte("paused"))
	if err != nil || string(got) != "paused" {
		t.Fatalf("%q %v", got, err)
	}
}
func TestCaptureGuardRetainsFatalErrors(t *testing.T) {
	fatal := errors.New("display disappeared")
	_, err := guardedCapture(context.Background(), func() (bool, error) { return true, nil }, func() ([]byte, error) { return nil, fatal }, nil)
	if !errors.Is(err, fatal) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = guardedCapture(ctx, func() (bool, error) { return false, nil }, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestCaptureGuardMasksFailureOnlyDuringDesktopSwitch(t *testing.T) {
	active := true
	got, err := guardedCapture(context.Background(), func() (bool, error) { return active, nil }, func() ([]byte, error) { active = false; return nil, errors.New("access denied") }, []byte("paused"))
	if err != nil || string(got) != "paused" {
		t.Fatalf("%q %v", got, err)
	}
}
func TestCursorPlacement(t *testing.T) {
	monitor := image.Rect(-1024, 200, 0, 968)
	dest := image.Rect(0, 0, 512, 384)
	got, ok := cursorPlacement(monitor, dest, image.Pt(-924, 300), image.Pt(4, 6), image.Pt(32, 48))
	if !ok || got != image.Rect(48, 47, 64, 71) {
		t.Fatalf("%v %v", got, ok)
	}
	if _, ok := cursorPlacement(monitor, dest, image.Pt(1, 300), image.Point{}, image.Pt(32, 32)); ok {
		t.Fatal("cursor from another monitor")
	}
	if _, ok := cursorPlacement(image.Rectangle{}, dest, image.Point{}, image.Point{}, image.Pt(32, 32)); ok {
		t.Fatal("empty monitor")
	}
}

func TestCaptureGuardPropagatesProbeFailure(t *testing.T) {
	fatal := errors.New("desktop query broken")
	_, err := guardedCapture(context.Background(), func() (bool, error) { return false, fatal }, func() ([]byte, error) { t.Fatal("must not capture"); return nil, nil }, nil)
	if !errors.Is(err, fatal) {
		t.Fatal(err)
	}
}
func TestCursorPlacementLetterboxAndHotspot(t *testing.T) {
	got, ok := cursorPlacement(image.Rect(0, 0, 1024, 768), image.Rect(160, 0, 1120, 720), image.Pt(0, 0), image.Pt(8, 8), image.Pt(32, 32))
	if !ok || got != image.Rect(153, -7, 183, 23) {
		t.Fatalf("%v %v", got, ok)
	}
}

func TestCaptureRecoveryBoundsNativeFailuresAndResets(t *testing.T) {
	var recovery captureRecovery
	start := time.Unix(100, 0)
	native := temporaryCaptureError{errors.New("GDI access denied")}
	for _, offset := range []time.Duration{0, time.Second} {
		got, err := recovery.result(start.Add(offset), nil, native, []byte("paused"))
		if err != nil || string(got) != "paused" {
			t.Fatalf("transient %q %v", got, err)
		}
	}
	_, err := recovery.result(start.Add(3*time.Second), nil, native, nil)
	if err == nil {
		t.Fatal("persistent native failure hidden")
	}
	_, err = recovery.result(start.Add(4*time.Second), []byte("ok"), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := recovery.result(start.Add(5*time.Second), nil, native, []byte("paused"))
	if err != nil || string(got) != "paused" {
		t.Fatal("recovery did not reset")
	}
	fatal := errors.New("display topology changed")
	_, err = recovery.result(start.Add(6*time.Second), nil, fatal, nil)
	if !errors.Is(err, fatal) {
		t.Fatal(err)
	}
}
