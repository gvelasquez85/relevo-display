package desktop

import (
	"fmt"
	"math"
	"relevo/internal/winvirtual"
)

// Attached and non-mirroring are required. PRIMARY_DEVICE is a legitimate
// user topology change, not a loss of the verified virtual device identity.
func capturableVirtualFlags(flags uint32) bool { return flags&1 != 0 && flags&8 == 0 }
func validateLiveCaptureMode(m winvirtual.Mode) error {
	if !m.Attached || m.Width <= 0 || m.Height <= 0 || int64(m.X)+int64(m.Width) > math.MaxInt32 || int64(m.Y)+int64(m.Height) > math.MaxInt32 {
		return fmt.Errorf("virtual monitor has invalid live bounds")
	}
	return nil
}
