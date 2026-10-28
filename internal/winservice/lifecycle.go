// Package winservice owns the fixed Relevo boot service. Desktop capture and
// interactive UI belong to the user's agent, not Windows session 0.
package winservice

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const Name = "RelevoDisplay"
const DisplayName = "Relevo Display"

func prepareWithRetry(ctx context.Context, prepare func(context.Context) error, delay time.Duration) error {
	var last error
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(fmt.Errorf("boot preparation: %w", err), last)
		}
		last = prepare(ctx)
		if last == nil {
			return ctx.Err()
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.Join(fmt.Errorf("boot preparation: %w", ctx.Err()), last)
		case <-timer.C:
		}
	}
}

// A missing/transitioning console or a failed token query is never evidence
// that a console is pre-login. Restrict the broker to proven absence of a user.
func eligibleConsole(session uint32, noUser bool, queryErr error) bool {
	return session != 0 && session != 0xffffffff && noUser && queryErr == nil
}
