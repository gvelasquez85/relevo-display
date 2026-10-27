package winvirtual

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// KeepDisplayActiveError prevents removing the computer's last visible output.
// Callers must preserve both the adapter and any live video session on this error.
type KeepDisplayActiveError struct{ Cause error }

func (e *KeepDisplayActiveError) Error() string {
	return "se conservó la pantalla del iPad: no hay otra pantalla activa y no se pudo recuperar una salida alternativa"
}
func (e *KeepDisplayActiveError) Unwrap() error { return e.Cause }

// OtherOutputRestorer restores only an available alternative output. A success
// must be verified by enumeration; a successful native call alone is insufficient.
type OtherOutputRestorer interface{ RestoreOtherOutput(context.Context) error }

func otherOutputAttached(ds []Display) bool {
	for _, d := range ds {
		if !strings.EqualFold(d.HardwareID, hardwareID) && d.Mode.Attached && d.Mode.Width > 0 && d.Mode.Height > 0 {
			return true
		}
	}
	return false
}

// EnsureSafeDetach is mandatory immediately before retiring the virtual adapter.
// It never detaches any output itself. Transient all-inactive enumeration fails
// closed: Windows may still be rebuilding the sole virtual display path.
func EnsureSafeDetach(ctx context.Context, b Backend) error {
	if err := ctx.Err(); err != nil {
		return &KeepDisplayActiveError{err}
	}
	if b == nil {
		return &KeepDisplayActiveError{errors.New("display backend unavailable")}
	}
	ds, err := b.List(ctx)
	if err != nil {
		return &KeepDisplayActiveError{err}
	}
	if otherOutputAttached(ds) {
		return nil
	}
	r, ok := b.(OtherOutputRestorer)
	if !ok {
		return &KeepDisplayActiveError{errors.New("no alternative output recovery available")}
	}
	bounded, cancel := context.WithTimeout(ctx, activationTimeout)
	defer cancel()
	if err = r.RestoreOtherOutput(bounded); err != nil {
		return &KeepDisplayActiveError{err}
	}
	ds, err = b.List(bounded)
	if err != nil {
		return &KeepDisplayActiveError{err}
	}
	if !otherOutputAttached(ds) {
		return &KeepDisplayActiveError{fmt.Errorf("native recovery did not produce another active output")}
	}
	return nil
}
