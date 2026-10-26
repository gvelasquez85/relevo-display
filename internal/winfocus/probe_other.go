//go:build !windows || !amd64

package winfocus

import "errors"

// Probe is supported by the Windows amd64 host only.
func Probe(x, y int32) (bool, error) { return false, errors.New("focus probe requires Windows amd64") }
