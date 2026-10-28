package wintray

import (
	"errors"
	"fmt"
	"time"
)

var errWaitingForDesktop = errors.New("escritorio de usuario aún preparándose")

// Userinit, Explorer and DWM do not become ready atomically after authentication.
// Retry only during the initial handoff window; permanent driver errors must
// still reach the GUI instead of being hidden by an endless discovery loop.
func startupAttempt(err error, managed bool, started, now time.Time) error {
	if err == nil || !managed || errors.Is(err, ErrWaitingForDevice) {
		return err
	}
	if now.Sub(started) >= 30*time.Second {
		return err
	}
	return fmt.Errorf("%w: %v", errWaitingForDesktop, err)
}
