//go:build !windows

package wintray

import (
	"context"
	"fmt"
)

func OpenPairing(context.Context, string) (<-chan struct{}, error) {
	return nil, fmt.Errorf("Wi-Fi pairing requires the Windows GUI")
}
