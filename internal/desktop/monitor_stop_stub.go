//go:build !windows

package desktop

import (
	"context"
	"fmt"
)

func PrepareVirtualMonitorStop(context.Context, func(string)) error {
	return fmt.Errorf("Windows required")
}
