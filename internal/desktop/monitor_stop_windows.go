//go:build windows

package desktop

import (
	"context"
	"relevo/internal/windriver"
	"relevo/internal/winvirtual"
	"time"
)

// PrepareVirtualMonitorStop runs while video is still active. A sole iPad
// remains connected if Windows cannot restore any other real output.
func PrepareVirtualMonitorStop(ctx context.Context, log func(string)) error {
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return winvirtual.EnsureSafeDetach(bounded, windowsDisplayBackend{log: log})
}
func disableVirtualMonitorSafely(ctx context.Context, log func(string)) error {
	if err := PrepareVirtualMonitorStop(ctx, log); err != nil {
		return err
	}
	return windriver.EnsureDeviceEnabled(ctx, false, log)
}
