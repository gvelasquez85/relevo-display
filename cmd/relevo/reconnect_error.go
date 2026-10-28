package main

import (
	"errors"
	"fmt"
	"relevo/internal/wintray"
	"relevo/internal/winvirtual"
)

// The only output may intentionally remain advertised while the app is away.
// A separate cleanup failure (e.g. releasing ownership) must still stop retries.
func reconnectCleanupError(err error, depth int) error {
	if err == nil {
		return nil
	}
	if depth > 32 {
		return err
	}
	if _, ok := err.(*winvirtual.KeepDisplayActiveError); ok {
		return nil
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var remaining []error
		for _, child := range joined.Unwrap() {
			remaining = append(remaining, reconnectCleanupError(child, depth+1))
		}
		return errors.Join(remaining...)
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok && reconnectCleanupError(wrapped.Unwrap(), depth+1) == nil {
		return nil
	}
	return err
}
func compatibleSessionEnd(sessionErr, cleanupErr error, reconnect bool) error {
	if !reconnect {
		return errors.Join(sessionErr, cleanupErr)
	}
	if remaining := reconnectCleanupError(cleanupErr, 0); remaining != nil {
		return fmt.Errorf("no se pudo preparar la reconexión: %w", remaining)
	}
	return fmt.Errorf("%w: %v", wintray.ErrWaitingForDevice, sessionErr)
}
