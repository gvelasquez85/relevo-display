package main

import (
	"context"
	"errors"
)

// Only cancellation/deadline owned by this trial is an expected end. A frame
// source timeout while the trial remains active must stay visible as a failure.
func normalizeSessionError(ctx context.Context, err error) error {
	if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return nil
	}
	return err
}
