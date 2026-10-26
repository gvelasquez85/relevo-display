//go:build !windows

package wininput

import "fmt"

func NativeBackend() (Backend, error) {
	return nil, fmt.Errorf("remote touch currently requires Windows")
}
