//go:build !windows

package wininput

import "fmt"

func NativeExtendedBackend() (ExtendedBackend, error) {
	return nil, fmt.Errorf("multitouch and keyboard injection currently require Windows")
}
