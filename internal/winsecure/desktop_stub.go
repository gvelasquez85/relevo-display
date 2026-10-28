//go:build !windows

package winsecure

import "fmt"

func Enabled() bool          { return false }
func Enable() error          { return fmt.Errorf("Windows only") }
func Enter() (func(), error) { return func() {}, nil }
