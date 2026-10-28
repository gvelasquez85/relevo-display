package wintray

import (
	"errors"
	"time"
)

// ErrWaitingForDevice denotes discovery/connection failure before capture opens.
// Display/driver errors are deliberately not retried as discovery errors.
var ErrWaitingForDevice = errors.New("esperando iPad USB")

func autoConnect(stop <-chan struct{}, report func(string), run func() error, delay time.Duration) error {
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		err := run()
		if err != nil && !errors.Is(err, ErrWaitingForDevice) {
			return err
		}
		report("Esperando el iPad por USB… Abre la app compatible; se conectará automáticamente.")
		timer := time.NewTimer(delay)
		select {
		case <-stop:
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
