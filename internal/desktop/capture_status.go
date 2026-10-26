package desktop

// CaptureStatus records transitions away from the accessible input desktop.
// It does not identify the cause (UAC, lock screen, or another desktop).
type CaptureStatus struct {
	Paused             bool
	Pauses, Recoveries int
}

func (s *CaptureStatus) observe(paused bool) {
	if paused && !s.Paused {
		s.Pauses++
	}
	if !paused && s.Paused {
		s.Recoveries++
	}
	s.Paused = paused
}
