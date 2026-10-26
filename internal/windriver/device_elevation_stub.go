//go:build !windows

package windriver

import "context"

func EnsureDeviceEnabled(ctx context.Context, enabled bool, log func(string)) error {
	return SetDeviceEnabled(ctx, enabled, log)
}
