//go:build !windows

package windriver

import (
	"context"
	"fmt"
)

func ReadDeviceState(context.Context) (DeviceState, error) {
	return DeviceState{}, fmt.Errorf("virtual display device state requires Windows")
}
func SetDeviceEnabled(context.Context, bool, func(string)) error {
	return fmt.Errorf("virtual display device control requires Windows")
}
