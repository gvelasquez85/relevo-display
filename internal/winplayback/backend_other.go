//go:build !windows

package winplayback

import "fmt"

func nativeBackend() (Backend, error) {
	return nil, fmt.Errorf("microphone monitoring requires Windows")
}
