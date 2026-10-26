package main

import (
	"context"
)

// Only cancellation/deadline owned by this trial is an expected end. A frame
// source timeout while the trial remains active must stay visible as a failure.
func normalizeSessionError(ctx context.Context, err error) error {
	if owned := ctx.Err(); owned != nil {
		current := err
		for depth := 0; current != nil && depth < 32; depth++ {
			if current == owned {
				return nil
			}
			switch wrapped := current.(type) {
			case interface{ Unwrap() []error }:
				var next error
				for _, child := range wrapped.Unwrap() {
					if child == nil {
						continue
					}
					if next != nil {
						return err
					} // Preserve mixed cleanup failures.
					next = child
				}
				current = next
			case interface{ Unwrap() error }:
				current = wrapped.Unwrap()
			default:
				return err
			}
		}
	}
	return err
}
