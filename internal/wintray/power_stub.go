//go:build !windows

package wintray

import "errors"

func AcquireAwake() (func(), error) { return nil, errors.New("display sleep request requires Windows") }
