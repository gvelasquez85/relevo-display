//go:build !windows

package windriver

import (
	"context"
	"fmt"
)

func Supported() bool { return false }
func Install(context.Context, func(string)) error {
	return fmt.Errorf("virtual display driver installation is supported only on Windows amd64")
}
