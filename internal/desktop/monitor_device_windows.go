//go:build windows

package desktop

import (
	"context"
	"errors"
	"relevo/internal/windriver"
	"syscall"
)

// DisconnectVirtualMonitor also handles an advertised monitor with no active
// video session. The capture lock prevents detaching another Relevo session.
func DisconnectVirtualMonitor(ctx context.Context, log func(string)) (err error) {
	h, err := acquireVirtualMutex()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, syscall.CloseHandle(h)) }()
	return windriver.EnsureDeviceEnabled(ctx, false, log)
}
