package desktop

import "testing"

func TestCaptureStatusTransitions(t *testing.T) {
	var s CaptureStatus
	for _, p := range []bool{false, true, true, false, false, true, false} {
		s.observe(p)
	}
	if s.Paused || s.Pauses != 2 || s.Recoveries != 2 {
		t.Fatalf("%+v", s)
	}
}
