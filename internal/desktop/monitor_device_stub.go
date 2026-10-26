//go:build !windows

package desktop

import (
	"context"
	"fmt"
)

func DisconnectVirtualMonitor(context.Context, func(string)) error {
	return fmt.Errorf("monitor device control is supported only on Windows")
}
