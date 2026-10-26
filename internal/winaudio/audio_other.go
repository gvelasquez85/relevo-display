//go:build !windows || !amd64

package winaudio

import (
	"context"
	"fmt"
)

func Open(context.Context) (Source, error) {
	return nil, fmt.Errorf("WASAPI loopback requires Windows amd64")
}
