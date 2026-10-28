//go:build !windows

package winsecure

import (
	"fmt"
	"net"
)

func VerifyAppleUSBConnection(net.Conn) error { return fmt.Errorf("Windows only") }
