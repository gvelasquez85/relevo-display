package winservice

import "time"

// userAgentLaunch remembers a successful process creation, not process lifetime.
// Explicit Exit must not turn into a service respawn loop. A proven logoff or
// another valid console session starts a fresh login generation.
type userAgentLaunch struct {
	session  uint32
	launched bool
	retryAt  time.Time
}

func (s *userAgentLaunch) due(session uint32, noUser bool, queryErr error, now time.Time) bool {
	if session == 0 || session == 0xffffffff {
		return false
	}
	if s.session != session {
		s.session = session
		s.launched = false
		s.retryAt = time.Time{}
	}
	if queryErr != nil {
		return false
	}
	if noUser {
		s.launched = false
		s.retryAt = time.Time{}
		return false
	}
	return !s.launched && !now.Before(s.retryAt)
}
func (s *userAgentLaunch) result(err error, now time.Time) {
	if err == nil {
		s.launched = true
		return
	}
	s.retryAt = now.Add(2 * time.Second)
}
