package wintray

// stopPreflight is owned by the GUI thread. Tokens invalidate asynchronous
// topology checks when a session ends or another session starts.
type stopPreflight struct {
	serial  uint64
	pending bool
	exit    bool
}

func (s *stopPreflight) newSession() { s.serial++; s.pending = false; s.exit = false }
func (s *stopPreflight) finishSession() bool {
	exit := s.pending && s.exit
	s.serial++
	s.pending = false
	s.exit = false
	return exit
}
func (s *stopPreflight) begin(exit bool) (uint64, bool) {
	if s.pending {
		s.exit = s.exit || exit
		return s.serial, false
	}
	s.serial++
	s.pending = true
	s.exit = exit
	return s.serial, true
}
func (s *stopPreflight) complete(token uint64) (bool, bool) {
	if !s.pending || token != s.serial {
		return false, false
	}
	exit := s.exit
	s.pending = false
	s.exit = false
	return exit, true
}
