//go:build !windows

package desktop

import (
	"fmt"
	"relevo/internal/wininput"
)

func InputTarget(Source) (func() (wininput.Target, error), error) {
	return nil, fmt.Errorf("remote touch currently requires Windows")
}
